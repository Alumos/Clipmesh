package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr            string
	DataDir         string
	DatabasePath    string
	FilesDir        string
	AdminUsername   string
	AdminPassword   string
	CookieSecure    bool
	SessionTTL      time.Duration
	TextLimit       int
	FileTTL         time.Duration
	CleanupInterval time.Duration
	CORSOrigins     []string
	MaxUploadSize   int64
}

func Load() (Config, error) {
	dataDir := envString("CLIPMESH_DATA_DIR", "/data")
	adminUsername := strings.TrimSpace(os.Getenv("CLIPMESH_ADMIN_USERNAME"))
	adminPassword := os.Getenv("CLIPMESH_ADMIN_PASSWORD")
	if adminUsername == "" || adminPassword == "" {
		return Config{}, fmt.Errorf("CLIPMESH_ADMIN_USERNAME and CLIPMESH_ADMIN_PASSWORD are required")
	}
	cookieSecure, err := envBool("CLIPMESH_COOKIE_SECURE", false)
	if err != nil {
		return Config{}, fmt.Errorf("CLIPMESH_COOKIE_SECURE must be true or false")
	}
	sessionTTL, err := envDuration("CLIPMESH_SESSION_TTL", 7*24*time.Hour)
	if err != nil || sessionTTL <= 0 {
		return Config{}, fmt.Errorf("CLIPMESH_SESSION_TTL must be a positive duration, e.g. 168h")
	}
	textLimit, err := envInt("CLIPMESH_TEXT_LIMIT", 100)
	if err != nil || textLimit < 1 {
		return Config{}, fmt.Errorf("CLIPMESH_TEXT_LIMIT must be a positive integer")
	}

	fileTTL, err := envDuration("CLIPMESH_FILE_TTL", 24*time.Hour)
	if err != nil || fileTTL <= 0 {
		return Config{}, fmt.Errorf("CLIPMESH_FILE_TTL must be a positive duration, e.g. 24h")
	}
	cleanupInterval, err := envDuration("CLIPMESH_CLEANUP_INTERVAL", time.Hour)
	if err != nil || cleanupInterval <= 0 {
		return Config{}, fmt.Errorf("CLIPMESH_CLEANUP_INTERVAL must be a positive duration, e.g. 1h")
	}
	maxUploadSize, err := parseBytes(envString("CLIPMESH_MAX_UPLOAD_SIZE", "100MB"))
	if err != nil || maxUploadSize < 1 {
		return Config{}, fmt.Errorf("CLIPMESH_MAX_UPLOAD_SIZE must be a positive size, e.g. 100MB")
	}

	origins := make([]string, 0)
	for _, origin := range strings.Split(envString("CLIPMESH_CORS_ORIGINS", ""), ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			origins = append(origins, origin)
		}
	}

	return Config{
		Addr:            envString("CLIPMESH_ADDR", ":9000"),
		DataDir:         dataDir,
		DatabasePath:    filepath.Join(dataDir, "clipmesh.db"),
		FilesDir:        filepath.Join(dataDir, "files"),
		AdminUsername:   adminUsername,
		AdminPassword:   adminPassword,
		CookieSecure:    cookieSecure,
		SessionTTL:      sessionTTL,
		TextLimit:       textLimit,
		FileTTL:         fileTTL,
		CleanupInterval: cleanupInterval,
		CORSOrigins:     origins,
		MaxUploadSize:   maxUploadSize,
	}, nil
}

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) (int, error) {
	value := envString(key, strconv.Itoa(fallback))
	return strconv.Atoi(value)
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	value := envString(key, fallback.String())
	return time.ParseDuration(value)
}

func envBool(key string, fallback bool) (bool, error) {
	value := envString(key, strconv.FormatBool(fallback))
	return strconv.ParseBool(value)
}

func parseBytes(value string) (int64, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	units := []struct {
		suffix string
		factor int64
	}{
		{"GB", 1024 * 1024 * 1024},
		{"GIB", 1024 * 1024 * 1024},
		{"MB", 1024 * 1024},
		{"MIB", 1024 * 1024},
		{"KB", 1024},
		{"KIB", 1024},
		{"B", 1},
	}
	for _, unit := range units {
		if strings.HasSuffix(normalized, unit.suffix) {
			number := strings.TrimSpace(strings.TrimSuffix(normalized, unit.suffix))
			parsed, err := strconv.ParseFloat(number, 64)
			if err != nil || parsed <= 0 {
				return 0, fmt.Errorf("invalid size %q", value)
			}
			return int64(parsed * float64(unit.factor)), nil
		}
	}
	parsed, err := strconv.ParseInt(normalized, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q", value)
	}
	return parsed, nil
}
