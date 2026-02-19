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

	HouseEdge  float64
	PricesFile string
}

func Load() *Config {
	return &Config{
		BettingTime:    getEnvInt("BETTING_TIME", 3),
		TimeTillResult: getEnvInt("TIME_TILL_RESULT", 3),
		NextGameDelay:  getEnvInt("NEXT_GAME_DELAY", 3),
		OnlineInterval: getEnvInt("ONLINE_INTERVAL", 3),

		HouseEdge:  getEnvFloat("HOUSE_EDGE", 0.05),
		PricesFile: os.Getenv("PRICES_FILE"),
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

func getEnvFloat(key string, def float64) float64 {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	n, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return def
	}
	return n
}
