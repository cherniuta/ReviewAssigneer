package config

import (
	"github.com/ilyakaznacheev/cleanenv"
	"log"
	"time"
)

type HTTPConfig struct {
	Address string        `yaml:"address" env:"API_ADDRESS" env-default:"localhost:80"`
	Timeout time.Duration `yaml:"timeout" env:"API_TIMEOUT" env-default:"1s"`
}

type Config struct {
	LogLevel   string     `yaml:"log_level" env:"LOG_LEVEL" env-default:"DEBUG"`
	DBAddress  string     `yaml:"db_address" env:"DB_ADDRESS" env-default:"localhost:82"`
	HTTPConfig HTTPConfig `yaml:"api_server"`
}

func MustLoad(configPath string) Config {
	var cfg Config
	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		log.Fatalf("cannot read config %q: %s", configPath, err)
	}
	return cfg
}
