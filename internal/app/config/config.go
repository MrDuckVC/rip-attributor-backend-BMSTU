package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Config struct {
	ServiceHost   string
	ServicePort   int
	JWTSecret     string
	JWTIssuer     string
	JWTTTL        time.Duration
	RedisAddress  string
	RedisPassword string
}

func NewConfig() (*Config, error) {
	configName := "config"
	_ = godotenv.Load()
	if os.Getenv("CONFIG_NAME") != "" {
		configName = os.Getenv("CONFIG_NAME")
	}

	viper.SetConfigName(configName)
	viper.SetConfigType("toml")
	viper.AddConfigPath("config")
	viper.AddConfigPath(".")
	if err := viper.BindEnv("ServicePort", "SERVICE_PORT"); err != nil {
		return nil, err
	}
	viper.WatchConfig()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, err
	}

	cfg.JWTSecret = os.Getenv("JWT_SECRET")
	cfg.JWTIssuer = envDefault("JWT_ISSUER", "attributor")
	cfg.RedisAddress = envDefault("REDIS_ADDR", "localhost:6379")
	cfg.RedisPassword = os.Getenv("REDIS_PASSWORD")
	var err error
	cfg.JWTTTL, err = time.ParseDuration(envDefault("JWT_TTL", "1h"))
	if err != nil || cfg.JWTTTL < time.Second {
		return nil, fmt.Errorf("JWT_TTL must be a positive duration of at least one second")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must contain at least 32 bytes")
	}
	log.Info("config parsed")
	return cfg, nil
}

func envDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
