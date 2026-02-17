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
}

func Load() *Config {
	return &Config{
		BettingTime:    getEnvInt("BETTING_TIME", 3),
		TimeTillResult: getEnvInt("TIME_TILL_RESULT", 3),
		NextGameDelay:  getEnvInt("NEXT_GAME_DELAY", 3),
		OnlineInterval: getEnvInt("ONLINE_INTERVAL", 3),
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
