package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
)

// MinioStorage implements Service using MinIO or AWS S3 over pure REST API with zero external dependencies.
// It uses AWS Signature V4 authentication, automatically provisions buckets, and applies public streaming & CORS policies.
type MinioStorage struct {
	endpoint      string
	accessKey     string
	secretKey     string
	bucket        string
	streamBaseURL string
	useSSL        bool
	httpClient    *http.Client
}

// NewMinioStorage initializes a direct MinIO / S3 storage engine and configures the bucket.
func NewMinioStorage(endpoint, accessKey, secretKey, bucket, streamBaseURL string, useSSL bool) (*MinioStorage, error) {
	s := &MinioStorage{
		endpoint:      endpoint,
		accessKey:     accessKey,
		secretKey:     secretKey,
		bucket:        bucket,
		streamBaseURL: streamBaseURL,
		useSSL:        useSSL,
		httpClient:    &http.Client{Timeout: 60 * time.Second},
	}

	// Ensure bucket exists on startup and apply CORS / public read policies
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.ensureBucket(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize MinIO bucket %s: %w", bucket, err)
	}

	return s, nil
}

func (s *MinioStorage) getScheme() string {
	if s.useSSL {
		return "https"
	}
	return "http"
}

// ensureBucket checks if the bucket exists; if not, creates it and sets up CORS & public access.
func (s *MinioStorage) ensureBucket(ctx context.Context) error {
	// Check HEAD /bucket
	headURL := fmt.Sprintf("%s://%s/%s", s.getScheme(), s.endpoint, s.bucket)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, headURL, nil)
	if err != nil {
		return err
	}
	s.signRequest(req, emptySHA256)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Create Bucket via PUT /bucket
		putReq, _ := http.NewRequestWithContext(ctx, http.MethodPut, headURL, nil)
		s.signRequest(putReq, emptySHA256)
		createResp, err := s.httpClient.Do(putReq)
		if err != nil {
			return err
		}
		createResp.Body.Close()
		if createResp.StatusCode >= 400 {
			return fmt.Errorf("create bucket failed with status: %d", createResp.StatusCode)
		}
	}

	// Apply CORS rule to allow direct browser uploads (PUT, GET, OPTIONS from all origins)
	corsXML := `<CORSConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><CORSRule><AllowedOrigin>*</AllowedOrigin><AllowedMethod>GET</AllowedMethod><AllowedMethod>PUT</AllowedMethod><AllowedMethod>POST</AllowedMethod><AllowedMethod>DELETE</AllowedMethod><AllowedMethod>HEAD</AllowedMethod><AllowedHeader>*</AllowedHeader><ExposeHeader>ETag</ExposeHeader></CORSRule></CORSConfiguration>`
	corsURL := fmt.Sprintf("%s://%s/%s?cors", s.getScheme(), s.endpoint, s.bucket)
	corsReq, _ := http.NewRequestWithContext(ctx, http.MethodPut, corsURL, strings.NewReader(corsXML))
	corsReq.Header.Set("Content-Type", "application/xml")
	s.signRequest(corsReq, sha256Hex([]byte(corsXML)))
	if corsResp, err := s.httpClient.Do(corsReq); err == nil {
		corsResp.Body.Close()
	}

	// Apply Public Read policy so HLS streaming segments (.m3u8/.ts) can be read by video players without signatures
	policyJSON := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, s.bucket)
	policyURL := fmt.Sprintf("%s://%s/%s?policy", s.getScheme(), s.endpoint, s.bucket)
	policyReq, _ := http.NewRequestWithContext(ctx, http.MethodPut, policyURL, strings.NewReader(policyJSON))
	policyReq.Header.Set("Content-Type", "application/json")
	s.signRequest(policyReq, sha256Hex([]byte(policyJSON)))
	if policyResp, err := s.httpClient.Do(policyReq); err == nil {
		policyResp.Body.Close()
	}

	return nil
}

// SaveFile streams an object directly to the MinIO bucket via AWS Signature V4 PUT request.
func (s *MinioStorage) SaveFile(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (string, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	cleanKey := strings.TrimPrefix(key, "/")
	urlStr := fmt.Sprintf("%s://%s/%s/%s", s.getScheme(), s.endpoint, s.bucket, cleanKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, urlStr, reader)
	if err != nil {
		return "", err
	}
	if size >= 0 {
		req.ContentLength = size
	}
	req.Header.Set("Content-Type", contentType)

	// Use UNSIGNED-PAYLOAD so we can stream arbitrarily large files directly without computing SHA256 of entire body in RAM
	s.signRequest(req, "UNSIGNED-PAYLOAD")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("minio PUT request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("minio PUT failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return urlStr, nil
}

// GetFile opens a stream to read an object directly from MinIO.
func (s *MinioStorage) GetFile(ctx context.Context, key string) (io.ReadCloser, string, error) {
	cleanKey := strings.TrimPrefix(key, "/")
	if strings.HasPrefix(cleanKey, s.bucket+"/") {
		cleanKey = strings.TrimPrefix(cleanKey, s.bucket+"/")
	}
	urlStr := fmt.Sprintf("%s://%s/%s/%s", s.getScheme(), s.endpoint, s.bucket, cleanKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, "", err
	}
	s.signRequest(req, emptySHA256)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, "", fmt.Errorf("object not found: %s", key)
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, "", fmt.Errorf("minio GET failed with status %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return resp.Body, contentType, nil
}

// GetStreamURL returns the direct public HTTP URL for streaming the object from MinIO or the proxy endpoint.
func (s *MinioStorage) GetStreamURL(ctx context.Context, key string) (string, error) {
	cleanKey := strings.TrimPrefix(key, "/")
	if strings.HasPrefix(cleanKey, s.bucket+"/") {
		cleanKey = strings.TrimPrefix(cleanKey, s.bucket+"/")
	}
	if s.streamBaseURL != "" {
		base := strings.TrimSuffix(s.streamBaseURL, "/")
		return fmt.Sprintf("%s/%s", base, cleanKey), nil
	}
	return fmt.Sprintf("%s://%s/%s/%s", s.getScheme(), s.endpoint, s.bucket, cleanKey), nil
}

// DeleteFile removes an object from MinIO.
func (s *MinioStorage) DeleteFile(ctx context.Context, key string) error {
	cleanKey := strings.TrimPrefix(key, "/")
	urlStr := fmt.Sprintf("%s://%s/%s/%s", s.getScheme(), s.endpoint, s.bucket, cleanKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, urlStr, nil)
	if err != nil {
		return err
	}
	s.signRequest(req, emptySHA256)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// GeneratePresignedURL generates a direct S3/MinIO pre-signed PUT URL valid for 1 hour.
// The browser uploads directly to MinIO, bypassing the Go API server completely.
func (s *MinioStorage) GeneratePresignedURL(ctx context.Context, key string, contentType string, maxSizeBytes int64) (string, error) {
	cleanKey := strings.TrimPrefix(key, "/")
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStr := now.Format("20060102")
	region := "us-east-1"
	service := "s3"
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStr, region, service)

	queryParams := map[string]string{
		"X-Amz-Algorithm":      "AWS4-HMAC-SHA256",
		"X-Amz-Credential":     fmt.Sprintf("%s/%s", s.accessKey, credentialScope),
		"X-Amz-Date":           amzDate,
		"X-Amz-Expires":        "3600",
		"X-Amz-SignedHeaders":  "host",
	}

	var keys []string
	for k := range queryParams {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var encodedParts []string
	for _, k := range keys {
		encodedParts = append(encodedParts, fmt.Sprintf("%s=%s", url.QueryEscape(k), url.QueryEscape(queryParams[k])))
	}
	canonicalQueryString := strings.Join(encodedParts, "&")

	canonicalHeaders := fmt.Sprintf("host:%s\n", s.endpoint)
	signedHeaders := "host"
	payloadHash := "UNSIGNED-PAYLOAD"

	canonicalURI := "/" + path.Join(s.bucket, cleanKey)
	if !strings.HasPrefix(canonicalURI, "/") {
		canonicalURI = "/" + canonicalURI
	}

	canonicalRequest := fmt.Sprintf("PUT\n%s\n%s\n%s\n%s\n%s",
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	)

	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	)

	signingKey := getSigningKey(s.secretKey, dateStr, region, service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	finalURL := fmt.Sprintf("%s://%s%s?%s&X-Amz-Signature=%s", s.getScheme(), s.endpoint, canonicalURI, canonicalQueryString, signature)
	return finalURL, nil
}

const emptySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key []byte, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func getSigningKey(secretKey, dateStr, regionName, serviceName string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(dateStr))
	kRegion := hmacSHA256(kDate, []byte(regionName))
	kService := hmacSHA256(kRegion, []byte(serviceName))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}

func (s *MinioStorage) signRequest(req *http.Request, payloadHash string) {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStr := now.Format("20060102")
	region := "us-east-1"
	service := "s3"
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStr, region, service)

	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)

	var headerKeys []string
	for k := range req.Header {
		lower := strings.ToLower(k)
		if lower == "host" || lower == "content-type" || strings.HasPrefix(lower, "x-amz-") {
			headerKeys = append(headerKeys, lower)
		}
	}
	headerKeys = append(headerKeys, "host")
	sort.Strings(headerKeys)

	// Remove duplicates
	var uniqueKeys []string
	for i, k := range headerKeys {
		if i == 0 || k != headerKeys[i-1] {
			uniqueKeys = append(uniqueKeys, k)
		}
	}

	var canonicalHeaders bytes.Buffer
	for _, k := range uniqueKeys {
		if k == "host" {
			canonicalHeaders.WriteString(fmt.Sprintf("host:%s\n", s.endpoint))
		} else {
			canonicalHeaders.WriteString(fmt.Sprintf("%s:%s\n", k, strings.TrimSpace(req.Header.Get(k))))
		}
	}
	signedHeaders := strings.Join(uniqueKeys, ";")

	canonicalURI := req.URL.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	if !strings.HasPrefix(canonicalURI, "/") {
		canonicalURI = "/" + canonicalURI
	}

	// Sort query params
	var queryKeys []string
	queryMap := req.URL.Query()
	for k := range queryMap {
		queryKeys = append(queryKeys, k)
	}
	sort.Strings(queryKeys)
	var queryParts []string
	for _, k := range queryKeys {
		queryParts = append(queryParts, fmt.Sprintf("%s=%s", url.QueryEscape(k), url.QueryEscape(queryMap.Get(k))))
	}
	canonicalQueryString := strings.Join(queryParts, "&")

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders.String(),
		signedHeaders,
		payloadHash,
	)

	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	)

	signingKey := getSigningKey(s.secretKey, dateStr, region, service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.accessKey, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)
}
