package media

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/storage"
)

// Processor handles asynchronous video transcoding, slicing videos into short HLS segments
// and generating multi-bitrate playlists (1080p, 720p, 480p) for Adaptive Bitrate (ABR) streaming.
type Processor struct {
	repo       Repository
	storage    storage.Service
	ffmpegPath string
	mu         sync.Mutex
	activeJobs map[string]context.CancelFunc
	wg         sync.WaitGroup
}

// NewProcessor initializes the HLS transcoding worker.
func NewProcessor(repo Repository, storage storage.Service) *Processor {
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		slog.Warn("ffmpeg binary not found in PATH; media processor will use Instant Direct-Stream proxy mode", "error", err)
	} else {
		slog.Info("ffmpeg binary detected for HLS transcoding", "path", path)
	}
	return &Processor{
		repo:       repo,
		storage:    storage,
		ffmpegPath: path,
		activeJobs: make(map[string]context.CancelFunc),
	}
}

// ProcessAsset executes background video transcoding or instant proxy stream setup.
func (p *Processor) ProcessAsset(assetID, storageKey, sourceURL string) {
	ctx, cancel := context.WithCancel(context.Background())
	p.mu.Lock()
	if p.activeJobs == nil {
		p.activeJobs = make(map[string]context.CancelFunc)
	}
	p.activeJobs[assetID] = cancel
	p.wg.Add(1)
	p.mu.Unlock()

	defer func() {
		cancel()
		p.mu.Lock()
		delete(p.activeJobs, assetID)
		p.mu.Unlock()
		p.wg.Done()
	}()

	slog.Info("starting asynchronous media processing", "asset_id", assetID, "source_url", sourceURL)

	if err := p.repo.UpdateAssetProgress(ctx, assetID, 1, domain.MediaStatusProcessing); err != nil {
		_ = p.repo.UpdateAssetStatus(ctx, assetID, domain.MediaStatusProcessing, "")
	}

	// If FFmpeg is not installed or available, use Instant Direct-Stream Proxy Mode
	if p.ffmpegPath == "" {
		slog.Info("using instant direct-stream mode for asset", "asset_id", assetID)
		select {
		case <-time.After(1 * time.Second):
		case <-ctx.Done():
			slog.Info("processing interrupted by shutdown during direct-stream validation", "asset_id", assetID)
			_ = p.repo.UpdateAssetProgress(context.Background(), assetID, 0, domain.MediaStatusPending)
			return
		}

		// Add default high-def rendition record pointing to direct video URL
		rendition := &domain.MediaRendition{
			MediaAssetID: assetID,
			Resolution:   "1080p (Direct)",
			BitrateKbps:  4500,
			PlaylistURL:  sourceURL,
		}
		_ = p.repo.AddRendition(ctx, rendition)

		if err := p.repo.UpdateAssetProgress(ctx, assetID, 100, domain.MediaStatusReady); err != nil {
			_ = p.repo.UpdateAssetStatus(ctx, assetID, domain.MediaStatusReady, sourceURL)
		}
		return
	}

	// FFmpeg Transcoding Pipeline
	tmpDir, err := os.MkdirTemp("", "inox-hls-*")
	if err != nil {
		slog.Error("failed to create temp dir for transcoding", "error", err)
		_ = p.repo.UpdateAssetStatus(ctx, assetID, domain.MediaStatusFailed, "")
		return
	}
	defer os.RemoveAll(tmpDir)

	hasAudio := p.hasAudioStream(sourceURL)

	// Optimized FFmpeg command for 3-tier Adaptive Bitrate HLS ladder (1080p, 720p, 480p)
	// Using pad=ceil(iw/2)*2:ceil(ih/2)*2 ensures both width and height are even integers,
	// preventing 'width not divisible by 2' encoder errors when force_original_aspect_ratio produces odd dimensions.
	args := []string{
		"-i", sourceURL,
		"-filter_complex",
		"[0:v]split=3[v1][v2][v3];[v1]scale=w=1920:h=1080:force_original_aspect_ratio=decrease,pad=ceil(iw/2)*2:ceil(ih/2)*2[v1out];[v2]scale=w=1280:h=720:force_original_aspect_ratio=decrease,pad=ceil(iw/2)*2:ceil(ih/2)*2[v2out];[v3]scale=w=854:h=480:force_original_aspect_ratio=decrease,pad=ceil(iw/2)*2:ceil(ih/2)*2[v3out]",
		"-map", "[v1out]", "-c:v:0", "libx264", "-profile:v:0", "main", "-pix_fmt:v:0", "yuv420p", "-b:v:0", "4500k", "-maxrate:v:0", "5000k", "-bufsize:v:0", "6000k",
		"-map", "[v2out]", "-c:v:1", "libx264", "-profile:v:1", "main", "-pix_fmt:v:1", "yuv420p", "-b:v:1", "2500k", "-maxrate:v:1", "2800k", "-bufsize:v:1", "3500k",
		"-map", "[v3out]", "-c:v:2", "libx264", "-profile:v:2", "main", "-pix_fmt:v:2", "yuv420p", "-b:v:2", "1000k", "-maxrate:v:2", "1200k", "-bufsize:v:2", "1500k",
	}

	if hasAudio {
		args = append(args,
			"-map", "0:a:0?", "-map", "0:a:0?", "-map", "0:a:0?",
			"-c:a", "aac", "-b:a", "128k", "-ar", "48000",
		)
	}

	varStreamMap := "v:0,name:1080p v:1,name:720p v:2,name:480p"
	if hasAudio {
		varStreamMap = "v:0,a:0,name:1080p v:1,a:1,name:720p v:2,a:2,name:480p"
	}

	args = append(args,
		"-f", "hls",
		"-hls_time", "4",
		"-hls_playlist_type", "vod",
		"-hls_flags", "independent_segments",
		"-hls_segment_filename", "stream_%v_data_%03d.ts",
		"-master_pl_name", "master.m3u8",
		"-var_stream_map", varStreamMap,
		"-progress", "pipe:1",
		"stream_%v.m3u8",
	)

	cmd := exec.CommandContext(ctx, p.ffmpegPath, args...)
	cmd.Dir = tmpDir
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		slog.Error("failed to create stdout pipe for ffmpeg progress", "error", err)
		_ = p.repo.UpdateAssetStatus(ctx, assetID, domain.MediaStatusFailed, "")
		return
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	slog.Info("executing ffmpeg transcoding ladder", "cmd", cmd.String())
	if err := cmd.Start(); err != nil {
		slog.Error("failed to start ffmpeg", "error", err)
		_ = p.repo.UpdateAssetStatus(ctx, assetID, domain.MediaStatusFailed, "")
		return
	}

	totalDurationUs := p.getVideoDurationUs(sourceURL)
	lastReportedPct := 1
	scanner := bufio.NewScanner(stdoutPipe)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "out_time_us=") {
			timeStr := strings.TrimPrefix(line, "out_time_us=")
			if us, err := strconv.ParseInt(timeStr, 10, 64); err == nil && totalDurationUs > 0 {
				pct := int((us * 95) / totalDurationUs) // Reserve last 5% for HLS segment upload
				if pct > 95 {
					pct = 95
				}
				if pct-lastReportedPct >= 5 || pct == 95 {
					lastReportedPct = pct
					slog.Info("transcoding progression", "asset_id", assetID, "progress_percent", pct, "status", domain.MediaStatusProcessing)
					_ = p.repo.UpdateAssetProgress(ctx, assetID, pct, domain.MediaStatusProcessing)
				}
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			slog.Info("ffmpeg transcoding interrupted by server shutdown; resetting status to pending", "asset_id", assetID)
			_ = p.repo.UpdateAssetProgress(context.Background(), assetID, 0, domain.MediaStatusPending)
			return
		}
		slog.Error("ffmpeg transcoding failed", "error", err, "stderr", stderrBuf.String())
		_ = p.repo.UpdateAssetStatus(context.Background(), assetID, domain.MediaStatusFailed, "")
		return
	}

	// Transcoding finished, updating to 96% while uploading HLS segments to MinIO
	_ = p.repo.UpdateAssetProgress(ctx, assetID, 96, domain.MediaStatusProcessing)

	// Upload generated HLS manifests and video chunks to object storage
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		slog.Error("failed to read transcoded files", "error", err)
		_ = p.repo.UpdateAssetStatus(ctx, assetID, domain.MediaStatusFailed, "")
		return
	}

	var masterURL string
	var uploadErrors int
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		filePath := filepath.Join(tmpDir, file.Name())
		f, err := os.Open(filePath)
		if err != nil {
			uploadErrors++
			continue
		}
		info, _ := f.Stat()
		contentType := "video/mp2t"
		if filepath.Ext(file.Name()) == ".m3u8" {
			contentType = "application/vnd.apple.mpegurl"
		}

		storageKey := fmt.Sprintf("hls/%s/%s", assetID, file.Name())
		url, err := p.storage.SaveFile(ctx, storageKey, f, info.Size(), contentType)
		f.Close()
		if err != nil {
			slog.Error("failed to upload hls segment", "file", file.Name(), "error", err)
			uploadErrors++
			continue
		}

		if file.Name() == "master.m3u8" {
			masterURL = url
		} else if file.Name() == "stream_1080p.m3u8" {
			_ = p.repo.AddRendition(ctx, &domain.MediaRendition{MediaAssetID: assetID, Resolution: "1080p", BitrateKbps: 4500, PlaylistURL: url})
		} else if file.Name() == "stream_720p.m3u8" {
			_ = p.repo.AddRendition(ctx, &domain.MediaRendition{MediaAssetID: assetID, Resolution: "720p", BitrateKbps: 2500, PlaylistURL: url})
		} else if file.Name() == "stream_480p.m3u8" {
			_ = p.repo.AddRendition(ctx, &domain.MediaRendition{MediaAssetID: assetID, Resolution: "480p", BitrateKbps: 1000, PlaylistURL: url})
		}
	}

	if uploadErrors > 0 || masterURL == "" {
		slog.Error("hls segment/manifest upload failed; marking asset transcoding as failed", "asset_id", assetID, "upload_errors", uploadErrors)
		_ = p.repo.UpdateAssetProgress(ctx, assetID, 0, domain.MediaStatusFailed)
		_ = p.repo.UpdateAssetStatus(ctx, assetID, domain.MediaStatusFailed, "")
		return
	}

	if err := p.repo.UpdateAssetProgress(ctx, assetID, 100, domain.MediaStatusReady); err != nil {
		_ = p.repo.UpdateAssetStatus(ctx, assetID, domain.MediaStatusReady, masterURL)
	} else {
		_ = p.repo.UpdateAssetStatus(ctx, assetID, domain.MediaStatusReady, masterURL)
	}
	slog.Info("media transcoding and hls upload complete", "asset_id", assetID, "master_url", masterURL, "progress_percent", 100)
}

func (p *Processor) hasAudioStream(sourceURL string) bool {
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		// Fallback assumption if ffprobe is not found
		return true
	}
	cmd := exec.Command(ffprobePath,
		"-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "stream=index",
		"-of", "csv=p=0",
		sourceURL,
	)
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return false
	}
	return true
}

func (p *Processor) getVideoDurationUs(sourceURL string) int64 {
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		return 0
	}
	cmd := exec.Command(ffprobePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		sourceURL,
	)
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return 0
	}
	var sec float64
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%f", &sec); err != nil || sec <= 0 {
		return 0
	}
	return int64(sec * 1000000)
}

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Shutdown cleanly cancels all actively running transcoding jobs, waiting for them
// to transition back to 'pending' state in the database before shutting down.
func (p *Processor) Shutdown(ctx context.Context) {
	p.mu.Lock()
	count := len(p.activeJobs)
	slog.Info("shutting down media processor; interrupting active transcode jobs...", "active_jobs", count)
	for _, cancel := range p.activeJobs {
		cancel()
	}
	p.mu.Unlock()

	// Wait for all transcode goroutines to exit and clean up DB state
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("all transcode jobs cleanly shut down and reset to pending")
	case <-ctx.Done():
		slog.Warn("timeout waiting for transcode jobs to shut down")
	}
}
