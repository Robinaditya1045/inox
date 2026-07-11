package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Environment          string
	HTTPPort             string
	LogLevel             string
	DatabaseURL          string
	RedisURL             string
	SessionSecret        string
	SessionDurationHours string
	StorageDir           string
	MediaStreamBaseURL   string
	MinioEndpoint        string
	MinioRootUser        string
	MinioRootPassword    string
	MinioUseSSL          string
	MinioBucketName      string
	CORSAllowedOrigins   string
	WebRTCICEServers     string
	WebRTCPortMin        string
	WebRTCPortMax        string
}

func Load() (*Config, error) {
	loadDotEnv()

	cfg := &Config{
		Environment:          getEnv("APP_ENV", "development"),
		HTTPPort:             getEnv("HTTP_PORT", "8080"),
		LogLevel:             getEnv("LOG_LEVEL", "debug"),
		// Default local dev DSN (Data Source Name)
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/inox?sslmode=disable"),
		RedisURL:             getEnv("REDIS_URL", "redis://localhost:6379/0"),
		SessionSecret:        getEnv("SESSION_SECRET", "supersecretkey1234567890abcdefghijklmnopqrstuvwxyz"),
		SessionDurationHours: getEnv("SESSION_DURATION_HOURS", "168"),
		StorageDir:           getEnv("STORAGE_DIR", "./storage_data"),
		MediaStreamBaseURL:   getEnv("MEDIA_STREAM_BASE_URL", "http://localhost:8080/media/stream"),
		MinioEndpoint:        getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinioRootUser:        getEnv("MINIO_ROOT_USER", "minioadmin"),
		MinioRootPassword:    getEnv("MINIO_ROOT_PASSWORD", "minioadmin"),
		MinioUseSSL:          getEnv("MINIO_USE_SSL", "false"),
		MinioBucketName:      getEnv("MINIO_BUCKET_NAME", "inox-media"),
		CORSAllowedOrigins:   getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:5174,http://localhost:3000"),
		WebRTCICEServers:     getEnv("WEBRTC_ICE_SERVERS", "stun:stun.l.google.com:19302,stun:stun1.l.google.com:19302"),
		WebRTCPortMin:        getEnv("WEBRTC_PORT_MIN", "50000"),
		WebRTCPortMax:        getEnv("WEBRTC_PORT_MAX", "50100"),
	}

	// Fail fast if DATABASE_URL or REDIS_URL is empty
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}
	if cfg.RedisURL == "" {
		return nil, fmt.Errorf("REDIS_URL environment variable is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}

// loadDotEnv checks for .env in current and parent directories and populates missing os environment variables.
func loadDotEnv() {
	paths := []string{".env", filepath.Join("..", ".env"), filepath.Join("..", "..", ".env")}
	for _, path := range paths {
		file, err := os.Open(path)
		if err == nil {
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					val := strings.TrimSpace(parts[1])
					if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
						val = val[1 : len(val)-1]
					}
					if os.Getenv(key) == "" {
						os.Setenv(key, val)
					}
				}
			}
			file.Close()
			break
		}
	}
}

