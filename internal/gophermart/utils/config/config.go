package config

import (
	"flag"
	"log"

	"github.com/caarlos0/env"
)

type Config struct {
	Address   string `env:"RUN_ADDRESS"`
	DBAddress string `env:"DATABASE_URI"`
	Accrual   string `env:"ACCRUAL_SYSTEM_ADDRESS"`
	JWTSecret string `env:"JWT_SECRET"`
}

func New() *Config {
	var cfg Config

	flag.StringVar(&cfg.Address, "a", "127.0.0.1:8080", "set server listening address")
	flag.StringVar(&cfg.DBAddress, "d", "", "set the DB address")
	flag.StringVar(&cfg.Accrual, "r", "", "accrual system addres")
	flag.StringVar(&cfg.JWTSecret, "js", "secret", "secret token for jwt")
	flag.Parse()

	if err := env.Parse(&cfg); err != nil {
		log.Printf("env parse failed :%s", err)
	}

	return &cfg
}
