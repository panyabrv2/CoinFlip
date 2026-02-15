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
		BettingTime:    getEnvInt("BETTING_TIME", 10),
		TimeTillResult: getEnvInt("TIME_TILL_RESULT", 5),
		NextGameDelay:  getEnvInt("NEXT_GAME_DELAY", 2),
		OnlineInterval: getEnvInt("ONLINE_INTERVAL", 5),
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
