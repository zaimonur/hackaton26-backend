package config

import "os"

type Config struct {
	JWTSecret string
	Port      string
}

func LoadConfig() *Config {
	return &Config{
		JWTSecret: getEnv("JWT_SECRET", "default-insecure-secret"),
		Port:      getEnv("PORT", "8080"),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
