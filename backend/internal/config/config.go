package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Environment            string
	HTTPPort               string
	LogLevel               string
	DatabaseURL            string
	RedisURL               string
	SessionSecret          string
	SessionDurationHours   string
	StorageDir             string
	MediaStreamBaseURL     string
	MinioEndpoint          string
	MinioRootUser          string
	MinioRootPassword      string
	MinioUseSSL            string
	MinioBucketName        string
	CORSAllowedOrigins     string
	LiveProxyBaseURL       string
	LiveSourceAllowedHosts string
	WebRTCICEServers       string
	WebRTCPortMin          string
	WebRTCPortMax          string
	WebRTCPublicIP         string
}

func Load() (*Config, error) {
	loadDotEnv()

	cfg := &Config{
		Environment: getEnv("APP_ENV", "development"),
		HTTPPort:    getEnv("HTTP_PORT", "8080"),
		LogLevel:    getEnv("LOG_LEVEL", "debug"),
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
		CORSAllowedOrigins:   getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:5174,http://localhost:3000,http://localhost:3001"),
		LiveProxyBaseURL:     getEnv("LIVE_PROXY_BASE_URL", "http://localhost:8080/api/v1/live"),
		// Hosts live channel sources may be pulled from. Empty is refused outright in
		// production; elsewhere it allows any public host with a startup warning, so
		// that local development does not push operators towards a wildcard.
		LiveSourceAllowedHosts: getEnv("LIVE_SOURCE_ALLOWED_HOSTS", ""),
		WebRTCICEServers:       getEnv("WEBRTC_ICE_SERVERS", "stun:stun.l.google.com:19302,stun:stun1.l.google.com:19302"),
		WebRTCPortMin:          getEnv("WEBRTC_PORT_MIN", "50000"),
		WebRTCPortMax:          getEnv("WEBRTC_PORT_MAX", "50100"),
		// Set to the VM's public address when the host sits behind 1:1 NAT (Oracle
		// Cloud, EC2, GCE). Left empty the SFU advertises its private IP, which no
		// remote browser can route to, and voice chat never leaves "checking".
		WebRTCPublicIP: getEnv("WEBRTC_PUBLIC_IP", ""),
	}

	// Fail fast if DATABASE_URL or REDIS_URL is empty
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}
	if cfg.RedisURL == "" {
		return nil, fmt.Errorf("REDIS_URL environment variable is required")
	}

	// SECURITY: Prevent production deployments from using the default hardcoded session secret.
	// Every installation sharing the same secret allows cross-site session forgery.
	isProd := strings.ToLower(cfg.Environment) == "production" || strings.ToLower(cfg.Environment) == "staging"
	defaultSecret := "supersecretkey1234567890abcdefghijklmnopqrstuvwxyz"
	if isProd && cfg.SessionSecret == defaultSecret {
		return nil, fmt.Errorf("SESSION_SECRET must be explicitly configured in production/staging environments (do not use the default value)")
	}
	if isProd && len(cfg.SessionSecret) < 32 {
		return nil, fmt.Errorf("SESSION_SECRET must be at least 32 characters long in production/staging environments (current length: %d)", len(cfg.SessionSecret))
	}

	return cfg, nil
}

// IsProd returns true if the application is running in production or staging mode.
func (c *Config) IsProd() bool {
	env := strings.ToLower(c.Environment)
	return env == "production" || env == "staging"
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
