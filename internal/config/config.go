package config

import (
	"os"
	"strconv"
)

type Config struct {
	BettingTime    int
	TimeTillResult int
	NextGameDelay  int
	OnlineInterval int

	PricesFile  string
	PostgresDSN string

	RedisAddr              string
	RedisPassword          string
	RedisDB                int
	RedisTokenStaleSeconds int
	RedisTokenTouchSeconds int
}

func Load() *Config {
	return &Config{
		BettingTime:    getEnvInt("BETTING_TIME", 60),
		TimeTillResult: getEnvInt("TIME_TILL_RESULT", 5),
		NextGameDelay:  getEnvInt("NEXT_GAME_DELAY", 1),
		OnlineInterval: getEnvInt("ONLINE_INTERVAL", 5),

		PricesFile:  os.Getenv("PRICES_FILE"),
		PostgresDSN: getEnv("POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"),

		RedisAddr:              getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:          getEnv("REDIS_PASSWORD", ""),
		RedisDB:                getEnvInt("REDIS_DB", 0),
		RedisTokenStaleSeconds: getEnvInt("REDIS_TOKEN_STALE_SECONDS", 120),
		RedisTokenTouchSeconds: getEnvInt("REDIS_TOKEN_TOUCH_SECONDS", 30),
	}
}

func getEnvInt(key string, def int) int {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return def
	}
	return n
}

func getEnv(key, def string) string {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	return val
}
