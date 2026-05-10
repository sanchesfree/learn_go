package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config — конфигурация сервиса.
// На проде обычно используют viper/envconfig, но здесь показан ручной разбор env —
// это базовый навык, который нужно понимать.
type Config struct {
	// Server
	HTTPAddr string
	HTTPPort int

	// Database
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// Redis
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// Auth
	JWTSecret string

	// Worker
	WorkerInterval time.Duration

	// External API (для примера HTTP-клиента)
	NotifyAPIURL string
}

// Load читает конфиг из переменных окружения.
// В реальном проекте здесь был бы viper или подобная либа.
func Load() (*Config, error) {
	cfg := &Config{
		HTTPAddr:       getEnv("HTTP_ADDR", "0.0.0.0"),
		HTTPPort:       getEnvInt("HTTP_PORT", 8080),
		DBHost:         getEnv("DB_HOST", "localhost"),
		DBPort:         getEnvInt("DB_PORT", 5432),
		DBUser:         getEnv("DB_USER", "postgres"),
		DBPassword:     getEnv("DB_PASSWORD", "postgres"),
		DBName:         getEnv("DB_NAME", "booking"),
		DBSSLMode:      getEnv("DB_SSLMODE", "disable"),
		RedisAddr:      getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:  getEnv("REDIS_PASSWORD", ""),
		RedisDB:        getEnvInt("REDIS_DB", 0),
		JWTSecret:      getEnv("JWT_SECRET", "dev-secret-change-me"),
		WorkerInterval: getEnvDuration("WORKER_INTERVAL", "1m"),
		NotifyAPIURL:   getEnv("NOTIFY_API_URL", "http://localhost:9999/notify"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode,
	)
}

func (c *Config) validate() error {
	if c.DBPassword == "" {
		return fmt.Errorf("DB_PASSWORD is required")
	}
	return nil
}

// --- Helpers ---

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvDuration(key string, fallback string) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		v = fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return time.Minute
	}
	return d
}
