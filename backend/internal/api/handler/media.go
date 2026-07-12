package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/inox/inox/backend/internal/api/middleware"
	"github.com/inox/inox/backend/internal/api/respond"
	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/media"
)

// MediaHandler exposes HTTP endpoints for uploading videos, managing catalog assets,
// and retrieving adaptive bitrate HLS renditions.
type MediaHandler struct {
	service       media.Service
	streamBaseURL string
}

// NewMediaHandler initializes the HTTP media library controller.
func NewMediaHandler(service media.Service, streamBaseURL string) *MediaHandler {
	return &MediaHandler{service: service, streamBaseURL: streamBaseURL}
}

func (h *MediaHandler) NormalizeAsset(asset *domain.MediaAsset) *domain.MediaAsset {
	if asset == nil || h.streamBaseURL == "" {
		return asset
	}
	base := strings.TrimSuffix(h.streamBaseURL, "/")
	replaceURL := func(u string) string {
		if u == "" {
			return u
		}
		for _, prefix := range []string{
			"http://localhost:9000/inox-media",
			"http://127.0.0.1:9000/inox-media",
			"http://localhost:9000",
			"http://127.0.0.1:9000",
		} {
			if strings.HasPrefix(u, prefix) {
				clean := strings.TrimPrefix(u, prefix)
				clean = strings.TrimPrefix(clean, "/")
				return fmt.Sprintf("%s/%s", base, clean)
			}
		}
		return u
	}
	asset.SourceURL = replaceURL(asset.SourceURL)
	asset.HLSMasterURL = replaceURL(asset.HLSMasterURL)
	if asset.Renditions != nil {
		for _, r := range asset.Renditions {
			r.PlaylistURL = replaceURL(r.PlaylistURL)
		}
	}
	return asset
}

type registerMediaReq struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	SourceURL    string `json:"source_url"`
	ThumbnailURL string `json:"thumbnail_url"`
}

// Register catalogs an external HLS manifest or MP4 video URL without uploading a physical file.
func (h *MediaHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerMediaReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.WriteError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	if req.Title == "" || req.SourceURL == "" {
		respond.WriteError(w, http.StatusBadRequest, "title and source_url are required")
		return
	}

	userID := getOptionalUserID(r)
	asset, err := h.service.RegisterExternalMedia(r.Context(), req.Title, req.Description, req.SourceURL, req.ThumbnailURL, userID)
	if err != nil {
		respond.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respond.WriteJSON(w, http.StatusCreated, h.NormalizeAsset(asset))
}

// Upload handles multipart file upload of raw MP4 or HLS files, triggering asynchronous FFmpeg transcoding.
func (h *MediaHandler) Upload(w http.ResponseWriter, r *http.Request) {
	// Limit upload size to 1GB in memory/temp storage
	if err := r.ParseMultipartForm(1024 << 20); err != nil {
		respond.WriteError(w, http.StatusBadRequest, "file upload too large or malformed")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respond.WriteError(w, http.StatusBadRequest, "file parameter missing")
		return
	}
	defer file.Close()

	title := r.FormValue("title")
	if title == "" {
		title = header.Filename
	}
	description := r.FormValue("description")
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "video/mp4"
	}

	userID := getOptionalUserID(r)
	asset, err := h.service.UploadMedia(r.Context(), title, description, file, header.Size, contentType, userID)
	if err != nil {
		respond.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respond.WriteJSON(w, http.StatusCreated, h.NormalizeAsset(asset))
}

type presignedReq struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

// CreatePresignedUpload handles requests to initiate direct S3/MinIO uploads up to 3GB.
func (h *MediaHandler) CreatePresignedUpload(w http.ResponseWriter, r *http.Request) {
	var req presignedReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Size > 3<<30 {
		respond.WriteError(w, http.StatusBadRequest, "file size exceeds 3GB limit")
		return
	}
	userID := getOptionalUserID(r)
	asset, uploadURL, key, err := h.service.CreatePresignedUpload(r.Context(), req.Title, req.Description, req.Filename, req.ContentType, req.Size, userID)
	if err != nil {
		respond.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.WriteJSON(w, http.StatusCreated, map[string]any{
		"asset":      asset,
		"upload_url": uploadURL,
		"key":        key,
	})
}

type completeUploadReq struct {
	AssetID string `json:"asset_id"`
	Key     string `json:"key"`
}

// CompleteDirectUpload confirms a direct object storage upload and starts FFmpeg HLS transcoding.
func (h *MediaHandler) CompleteDirectUpload(w http.ResponseWriter, r *http.Request) {
	var req completeUploadReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	asset, err := h.service.CompleteDirectUpload(r.Context(), req.AssetID, req.Key)
	if err != nil {
		respond.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.WriteJSON(w, http.StatusOK, h.NormalizeAsset(asset))
}

type uploadProgressReader struct {
	reader          io.Reader
	totalBytes      int64
	bytesRead       int64
	lastReportedPct int
	key             string
}

func (u *uploadProgressReader) Read(p []byte) (n int, err error) {
	n, err = u.reader.Read(p)
	if n > 0 && u.totalBytes > 0 {
		u.bytesRead += int64(n)
		pct := int((u.bytesRead * 100) / u.totalBytes)
		if pct > 100 {
			pct = 100
		}
		if pct-u.lastReportedPct >= 10 || pct == 100 {
			u.lastReportedPct = pct
			slog.Info("media upload progression", "key", u.key, "progress_percent", pct, "bytes_transferred", u.bytesRead, "total_bytes", u.totalBytes)
		}
	}
	return n, err
}

// UploadDirectLocal handles direct HTTP PUT streaming of file bytes to local disk storage (up to 3GB)
// during local development and offline mode when not using cloud MinIO/S3.
func (h *MediaHandler) UploadDirectLocal(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		respond.WriteError(w, http.StatusBadRequest, "missing key parameter")
		return
	}
	maxSizeStr := r.URL.Query().Get("max_size")
	maxSizeBytes, _ := strconv.ParseInt(maxSizeStr, 10, 64)
	if maxSizeBytes <= 0 {
		maxSizeBytes = 3 << 30 // Default 3GB
	}

	// Enforce limit without buffering whole file in RAM using io.LimitReader
	limitedReader := io.LimitReader(r.Body, maxSizeBytes+1)
	defer r.Body.Close()

	progReader := &uploadProgressReader{
		reader:     limitedReader,
		totalBytes: r.ContentLength,
		key:        key,
	}

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	slog.Info("uploading file to object storage", "key", key, "size", r.ContentLength)
	// Save directly to storage service using stream reader
	_, err := h.service.SaveDirectLocalFile(r.Context(), key, progReader, r.ContentLength, contentType)
	if err != nil {
		respond.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("direct storage write failed: %v", err))
		return
	}

	w.WriteHeader(http.StatusOK)
}

// List returns paginated media library catalog items.
func (h *MediaHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	assets, err := h.service.ListMedia(r.Context(), limit, offset)
	if err != nil {
		respond.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for i, a := range assets {
		assets[i] = h.NormalizeAsset(a)
	}

	respond.WriteJSON(w, http.StatusOK, assets)
}

// GetByID returns detailed information and HLS renditions for a specific media asset.
func (h *MediaHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
		id = parts[len(parts)-1]
	}
	if id == "" {
		respond.WriteError(w, http.StatusBadRequest, "invalid media asset id")
		return
	}

	asset, err := h.service.GetMediaByID(r.Context(), id)
	if err != nil {
		respond.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	respond.WriteJSON(w, http.StatusOK, h.NormalizeAsset(asset))
}

// Delete removes an asset from the catalog and purges its stored files.
func (h *MediaHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
		id = parts[len(parts)-1]
	}
	if id == "" {
		respond.WriteError(w, http.StatusBadRequest, "invalid media asset id")
		return
	}

	if err := h.service.DeleteMedia(r.Context(), id); err != nil {
		respond.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func getOptionalUserID(r *http.Request) *string {
	if session, ok := middleware.GetSessionFromContext(r.Context()); ok && session != nil {
		return &session.UserID
	}
	return nil
}

// StreamProxy proxies HLS playlists (.m3u8), segments (.ts), and direct videos from MinIO object storage
// or local disk to remote tunneled browsers without CORS or connection refused errors.
func (h *MediaHandler) StreamProxy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Expose-Headers", "*")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	key := r.URL.Path
	for _, prefix := range []string{"/media/stream/", "/inox-media/", "/api/v1/media/stream/"} {
		if strings.HasPrefix(key, prefix) {
			key = strings.TrimPrefix(key, prefix)
			break
		}
	}
	key = strings.TrimPrefix(key, "/")
	if strings.HasPrefix(key, "inox-media/") {
		key = strings.TrimPrefix(key, "inox-media/")
	}

	reader, contentType, err := h.service.GetStreamFile(r.Context(), key)
	if err != nil {
		slog.Warn("stream proxy object not found or error reading from storage", "key", key, "error", err)
		http.Error(w, "media asset not found", http.StatusNotFound)
		return
	}
	defer reader.Close()

	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else if strings.HasSuffix(key, ".m3u8") {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	} else if strings.HasSuffix(key, ".ts") {
		w.Header().Set("Content-Type", "video/mp2t")
	} else if strings.HasSuffix(key, ".mp4") {
		w.Header().Set("Content-Type", "video/mp4")
	}

	if strings.HasSuffix(key, ".ts") || strings.HasSuffix(key, ".mp4") {
		w.Header().Set("Cache-Control", "public, max-age=31536000")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}

	w.WriteHeader(http.StatusOK)
	io.Copy(w, reader)
}

