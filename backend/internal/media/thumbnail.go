package media

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/inox/inox/backend/internal/domain"
)

// Posters are shown at card size in the library, so there is nothing to gain
// from a full-resolution frame. 640 wide stays sharp on a 2x display at the
// library's largest card width.
const (
	thumbnailWidth   = 640
	thumbnailTimeout = 90 * time.Second
)

// thumbnailOffset picks where in a video to look for its poster frame. The very
// start is usually a black frame or a title card, so it looks a fifth of the way
// in, but never past two minutes: the seek has to reach that point, and on a
// remote source every second skipped is bytes fetched.
func thumbnailOffset(durationSeconds float64) float64 {
	if durationSeconds <= 0 {
		return 5
	}
	at := durationSeconds * 0.2
	switch {
	case at > 120:
		return 120
	case at < 1:
		return 0
	}
	return at
}

// CanCaptureFrames reports whether this host has the ffmpeg needed to take posters.
func (p *Processor) CanCaptureFrames() bool {
	return p.ffmpegPath != ""
}

// GenerateThumbnail captures a poster frame for an asset that is already
// playable but has none: assets from before posters were captured, or ones
// whose capture failed during transcoding. It reads the source over its URL
// rather than staging it, since one frame needs only a small part of the file.
func (p *Processor) GenerateThumbnail(ctx context.Context, asset *domain.MediaAsset) error {
	if asset.Kind == domain.MediaKindLive {
		return errors.New("live channels have no frame to capture")
	}
	if asset.ThumbnailURL != "" {
		return nil
	}
	input := asset.SourceURL
	if input == "" {
		input = asset.HLSMasterURL
	}
	durationSeconds := p.probeDurationSeconds(ctx, input)
	if asset.DurationSeconds == 0 && durationSeconds > 0 {
		_ = p.repo.SetDuration(ctx, asset.ID, int(durationSeconds+0.5))
	}
	return p.captureThumbnail(ctx, asset.ID, input, durationSeconds)
}

// captureThumbnail extracts one representative frame of input and stores it as
// the asset's poster, unless the asset gained one meanwhile.
func (p *Processor) captureThumbnail(ctx context.Context, assetID, input string, durationSeconds float64) error {
	if p.ffmpegPath == "" {
		return errors.New("ffmpeg is not available")
	}

	dir, err := os.MkdirTemp("", "inox-thumb-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "poster.jpg")

	// A seek past the end produces no frame rather than an error, which happens
	// when the duration could not be probed and the default offset is too far
	// into a short clip; the start of the video is the fallback for that. Any
	// other failure (an unreachable source, a file ffmpeg cannot read) would
	// only fail again at another offset.
	at := thumbnailOffset(durationSeconds)
	err = p.extractFrame(ctx, input, out, at)
	if errors.Is(err, errNoFrame) && at > 0 {
		err = p.extractFrame(ctx, input, out, 0)
	}
	if err != nil {
		return err
	}

	f, err := os.Open(out)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}

	// A fresh key per capture, so the stream proxy can let browsers cache posters
	// indefinitely: a replaced poster arrives under a new URL.
	key := fmt.Sprintf("thumbs/%s-%s.jpg", assetID, generateUUID()[:8])
	url, err := p.storage.SaveFile(ctx, key, f, info.Size(), "image/jpeg")
	if err != nil {
		return fmt.Errorf("upload poster: %w", err)
	}
	if err := p.repo.SetThumbnail(ctx, assetID, url, true); err != nil {
		return err
	}
	slog.Info("captured poster frame for media asset", "asset_id", assetID, "key", key)
	return nil
}

// errNoFrame is ffmpeg exiting cleanly without having written a frame.
var errNoFrame = errors.New("ffmpeg produced no frame")

// probeDurationSeconds is getVideoDurationUs bounded in time, for sources read
// over the network that may never answer. Zero means unknown.
func (p *Processor) probeDurationSeconds(ctx context.Context, input string) float64 {
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		return 0
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, ffprobePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		input,
	).Output()
	if err != nil {
		return 0
	}
	var sec float64
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%f", &sec); err != nil || sec <= 0 {
		return 0
	}
	return sec
}

func (p *Processor) extractFrame(ctx context.Context, input, out string, at float64) error {
	ctx, cancel := context.WithTimeout(ctx, thumbnailTimeout)
	defer cancel()

	// -ss before -i seeks on the demuxer, which jumps rather than decoding up to
	// the offset. The thumbnail filter then looks at the next few dozen frames
	// and keeps the most typical one, which skips past a fade or a black frame
	// the seek happened to land on.
	cmd := exec.CommandContext(ctx, p.ffmpegPath,
		"-hide_banner", "-loglevel", "error", "-y",
		"-ss", fmt.Sprintf("%.2f", at),
		"-i", input,
		"-vf", fmt.Sprintf("thumbnail=48,scale='min(%d,iw)':-2", thumbnailWidth),
		"-frames:v", "1",
		"-q:v", "4",
		out,
	)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg frame capture at %.1fs: %w: %s", at, err, strings.TrimSpace(stderr.String()))
	}
	if info, err := os.Stat(out); err != nil || info.Size() == 0 {
		return fmt.Errorf("%w at %.1fs", errNoFrame, at)
	}
	return nil
}
