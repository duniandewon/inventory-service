package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Env struct {
	DatabaseUrl string
	RedisUrl    string
	Port        string
	JwtSecret   string

	OTPLength        int
	OTPTTL           time.Duration
	OTPMaxRequests   int
	OTPRequestWindow time.Duration
	OTPMaxAttempts   int
	OTPLockoutTTL    time.Duration

	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

func getEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("Environment variable %s is required", key)
	}
	return val
}

func getEnvIntOrDefault(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		log.Fatalf("Environment variable %s must be an integer: %v", key, err)
	}
	return parsed
}

func getEnvDurationOrDefault(key string, fallback time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(val)
	if err != nil {
		log.Fatalf("Environment variable %s must be a duration (e.g. 5m): %v", key, err)
	}
	return parsed
}

func NewEnv() *Env {
	godotenv.Load()

	return &Env{
		Port:        getEnv("PORT"),
		DatabaseUrl: getEnv("DATABASE_URL"),
		RedisUrl:    getEnv("REDIS_URL"),
		JwtSecret:   getEnv("JWT_SECRET"),

		OTPLength:        getEnvIntOrDefault("OTP_LENGTH", 6),
		OTPTTL:           getEnvDurationOrDefault("OTP_TTL", 5*time.Minute),
		OTPMaxRequests:   getEnvIntOrDefault("OTP_MAX_REQUESTS", 3),
		OTPRequestWindow: getEnvDurationOrDefault("OTP_REQUEST_WINDOW", 15*time.Minute),
		OTPMaxAttempts:   getEnvIntOrDefault("OTP_MAX_ATTEMPTS", 5),
		OTPLockoutTTL:    getEnvDurationOrDefault("OTP_LOCKOUT_TTL", 30*time.Minute),

		AccessTokenTTL:  getEnvDurationOrDefault("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL: getEnvDurationOrDefault("REFRESH_TOKEN_TTL", 7*24*time.Hour),
	}
}
