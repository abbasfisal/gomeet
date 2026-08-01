package config

import (
	"os"
	"strconv"
)

type Config struct {
	ServerPort  string
	DatabaseURL string
	RedisURL    string
	RedisPass   string
	RedisDB     int
}

func Load() *Config {
	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	return &Config{
		ServerPort:  getEnv("SERVER_PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "host=localhost user=meetroom password=meetroom dbname=meetroom port=5432 sslmode=disable TimeZone=UTC"),
		RedisURL:    getEnv("REDIS_URL", "localhost:6379"),
		RedisPass:   getEnv("REDIS_PASSWORD", ""),
		RedisDB:     redisDB,
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
