package handler

import (
	"encoding/json"
	"errors"
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

// publicURL rewrites a stored URL that names MinIO's local address to the
// stream base browsers can reach.
func (h *MediaHandler) publicURL(u string) string {
	if u == "" || h.streamBaseURL == "" {
		return u
	}
	base := strings.TrimSuffix(h.streamBaseURL, "/")
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

func (h *MediaHandler) NormalizeAsset(asset *domain.MediaAsset) *domain.MediaAsset {
	if asset == nil || h.streamBaseURL == "" {
		return asset
	}
	asset.SourceURL = h.publicURL(asset.SourceURL)
	asset.HLSMasterURL = h.publicURL(asset.HLSMasterURL)
	asset.ThumbnailURL = h.publicURL(asset.ThumbnailURL)
	for _, r := range asset.Renditions {
		r.PlaylistURL = h.publicURL(r.PlaylistURL)
	}
	for _, c := range asset.Captions {
		h.normalizeCaption(c)
	}
	return asset
}

func (h *MediaHandler) normalizeCaption(c *domain.MediaCaption) *domain.MediaCaption {
	if c != nil {
		c.URL = h.publicURL(c.URL)
	}
	return c
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
// maxMultipartUpload caps the buffered multipart path, matching the size the
// admin portal advertises on its "Multipart" tab. Larger files go through the
// presigned direct-upload path instead.
const maxMultipartUpload = 1 << 30 // 1 GB

func (h *MediaHandler) Upload(w http.ResponseWriter, r *http.Request) {
	// MaxBytesReader is what actually enforces the cap. ParseMultipartForm's
	// argument is the in-memory buffer size, not a limit, so passing 1 GB there
	// meant a 252 MB upload was held entirely in RAM on a 6 GB host. 32 MB in
	// memory with the remainder spilled to temp files is the intended shape.
	r.Body = http.MaxBytesReader(w, r.Body, maxMultipartUpload)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
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

// maxThumbnailUpload caps a poster image. Posters are shown at card size, so
// anything near this is far larger than it needs to be.
const maxThumbnailUpload = 10 << 20 // 10 MB

// UploadThumbnail replaces an asset's poster with an uploaded image.
func (h *MediaHandler) UploadThumbnail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respond.WriteError(w, http.StatusBadRequest, "invalid media asset id")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxThumbnailUpload)
	if err := r.ParseMultipartForm(maxThumbnailUpload); err != nil {
		respond.WriteError(w, http.StatusBadRequest, "thumbnail too large or malformed (10 MB max)")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		respond.WriteError(w, http.StatusBadRequest, "file parameter missing")
		return
	}
	defer file.Close()

	// Sniffed rather than taken from the part header, which is whatever the
	// client claims and is what the stream proxy will later serve it as.
	sniff := make([]byte, 512)
	n, _ := io.ReadFull(file, sniff)
	contentType := http.DetectContentType(sniff[:n])
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		respond.WriteError(w, http.StatusInternalServerError, "failed to read thumbnail")
		return
	}

	asset, err := h.service.UploadThumbnail(r.Context(), id, file, header.Size, contentType)
	if err != nil {
		switch {
		case errors.Is(err, media.ErrUnsupportedThumbnail):
			respond.WriteError(w, http.StatusUnsupportedMediaType, err.Error())
		case strings.Contains(err.Error(), "not found"):
			respond.WriteError(w, http.StatusNotFound, err.Error())
		default:
			respond.WriteError(w, http.StatusInternalServerError, err.Error())
		}
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
	} else if strings.HasSuffix(key, ".vtt") {
		w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	}

	// Posters, caption tracks and avatars are stored under a fresh key each
	// time one is replaced, so they can be cached as long as segments.
	if strings.HasSuffix(key, ".ts") || strings.HasSuffix(key, ".mp4") || strings.HasPrefix(key, "thumbs/") || strings.HasPrefix(key, "captions/") || strings.HasPrefix(key, "avatars/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}

	w.WriteHeader(http.StatusOK)
	io.Copy(w, reader)
}

type updateDetailsReq struct {
	Title        *string `json:"title"`
	Description  *string `json:"description"`
	ThumbnailURL *string `json:"thumbnail_url"`
}

// UpdateDetails edits a video's title, description, or points its poster at an
// image link. Fields left out of the body are left as they are.
func (h *MediaHandler) UpdateDetails(w http.ResponseWriter, r *http.Request) {
	var req updateDetailsReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		respond.WriteError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	asset, err := h.service.UpdateDetails(r.Context(), r.PathValue("id"), media.DetailsUpdate{
		Title:        req.Title,
		Description:  req.Description,
		ThumbnailURL: req.ThumbnailURL,
	})
	if err != nil {
		writeMediaError(w, err)
		return
	}
	respond.WriteJSON(w, http.StatusOK, h.NormalizeAsset(asset))
}

// ResetThumbnail drops a video's custom poster in favour of a captured frame.
func (h *MediaHandler) ResetThumbnail(w http.ResponseWriter, r *http.Request) {
	asset, err := h.service.ResetThumbnail(r.Context(), r.PathValue("id"))
	if err != nil {
		writeMediaError(w, err)
		return
	}
	respond.WriteJSON(w, http.StatusOK, h.NormalizeAsset(asset))
}

// CaptionsForMedia lists the caption tracks of the library video at ?url=, the
// URL a room is playing. Rooms only know what they play by URL, and a pasted
// link to a library video should get its captions too.
func (h *MediaHandler) CaptionsForMedia(w http.ResponseWriter, r *http.Request) {
	captions, err := h.service.CaptionsForMediaURL(r.Context(), r.URL.Query().Get("url"))
	if err != nil {
		respond.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, c := range captions {
		h.normalizeCaption(c)
	}
	respond.WriteJSON(w, http.StatusOK, captions)
}

type captionReq struct {
	Language  *string `json:"language"`
	Label     *string `json:"label"`
	Content   *string `json:"content"`
	IsDefault *bool   `json:"is_default"`
}

// readCaptionRequest accepts a caption track either as a multipart upload (a
// .vtt or .srt "file" plus form fields) or as JSON carrying the text itself,
// which is what the portal's caption editor sends.
func readCaptionRequest(w http.ResponseWriter, r *http.Request) (captionReq, []byte, error) {
	var req captionReq
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, media.MaxCaptionBytes+(1<<20))
		if err := r.ParseMultipartForm(media.MaxCaptionBytes + (1 << 20)); err != nil {
			return req, nil, errors.New("caption file too large or malformed (2 MB max)")
		}
		for field, dst := range map[string]**string{"language": &req.Language, "label": &req.Label} {
			if vals, ok := r.MultipartForm.Value[field]; ok && len(vals) > 0 {
				v := vals[0]
				*dst = &v
			}
		}
		if v := r.FormValue("is_default"); v != "" {
			b := v == "true" || v == "1" || v == "on"
			req.IsDefault = &b
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			return req, nil, nil
		}
		defer file.Close()
		content, err := io.ReadAll(io.LimitReader(file, media.MaxCaptionBytes+1))
		if err != nil {
			return req, nil, errors.New("failed to read caption file")
		}
		return req, content, nil
	}

	// JSON escaping can double the size of the text it carries.
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2*media.MaxCaptionBytes)).Decode(&req); err != nil {
		return req, nil, errors.New("invalid request payload")
	}
	if req.Content != nil {
		return req, []byte(*req.Content), nil
	}
	return req, nil, nil
}

// CreateCaption adds a language's caption track to a video, or replaces the
// track it already has in that language.
func (h *MediaHandler) CreateCaption(w http.ResponseWriter, r *http.Request) {
	req, content, err := readCaptionRequest(w, r)
	if err != nil {
		respond.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Language == nil || content == nil {
		respond.WriteError(w, http.StatusBadRequest, "language and a caption file or content are required")
		return
	}
	label := ""
	if req.Label != nil {
		label = *req.Label
	}
	caption, err := h.service.SaveCaption(r.Context(), r.PathValue("id"), media.CaptionInput{
		Language:  *req.Language,
		Label:     label,
		Content:   content,
		IsDefault: req.IsDefault,
	})
	if err != nil {
		writeMediaError(w, err)
		return
	}
	respond.WriteJSON(w, http.StatusCreated, h.normalizeCaption(caption))
}

// UpdateCaption renames a track, changes its language or default flag, or
// replaces its cues.
func (h *MediaHandler) UpdateCaption(w http.ResponseWriter, r *http.Request) {
	req, content, err := readCaptionRequest(w, r)
	if err != nil {
		respond.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	caption, err := h.service.UpdateCaption(r.Context(), r.PathValue("id"), r.PathValue("caption_id"), media.CaptionUpdate{
		Language:  req.Language,
		Label:     req.Label,
		Content:   content,
		IsDefault: req.IsDefault,
	})
	if err != nil {
		writeMediaError(w, err)
		return
	}
	respond.WriteJSON(w, http.StatusOK, h.normalizeCaption(caption))
}

// DeleteCaption removes a track and its stored file.
func (h *MediaHandler) DeleteCaption(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteCaption(r.Context(), r.PathValue("id"), r.PathValue("caption_id")); err != nil {
		writeMediaError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeMediaError maps the media service's validation and lookup errors onto
// status codes; anything else is the server's fault.
func writeMediaError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, media.ErrInvalidDetails),
		errors.Is(err, media.ErrInvalidCaptions),
		errors.Is(err, media.ErrInvalidCaptionTrack),
		errors.Is(err, media.ErrInvalidLanguage):
		respond.WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, media.ErrLiveCaptions),
		errors.Is(err, media.ErrCaptionLanguageTaken):
		respond.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, media.ErrCaptionNotFound),
		strings.Contains(err.Error(), "not found"):
		respond.WriteError(w, http.StatusNotFound, err.Error())
	default:
		respond.WriteError(w, http.StatusInternalServerError, err.Error())
	}
}
