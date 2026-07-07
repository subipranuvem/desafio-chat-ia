package model

import (
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	PostgresDSN                  string `env:"POSTGRES_DSN,required"`
	RedisDSN                     string `env:"REDIS_DSN,required"`
	DeepSeekAPIKey               string `env:"DEEPSEEK_API_KEY"`
	GeminiAPIKey                 string `env:"GEMINI_API_KEY"`
	PingDatabaseIntervalInMillis int    `env:"PING_DATABASE_INTERVAL_IN_MILLIS" envDefault:"60000"`
	RedisSessionTTLInMillis      int    `env:"REDIS_SESSION_TTL_IN_MILLIS" envDefault:"180000"`
}

func LoadConfig() (Config, error) {
	// no-op when .env absent — prod containers inject vars directly
	_ = godotenv.Load()

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
