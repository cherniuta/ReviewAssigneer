package main

import (
	"flag"
	"github.com/ilyakaznacheev/cleanenv"
	"log"
	"log/slog"
	"net/http"
	"os"
	"review-assigner/adapters/db"
	"review-assigner/adapters/rest"
	"review-assigner/config"
	"review-assigner/core"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "server configuration file")
	flag.Parse()

	var cfg config.Config
	if _, err := os.Stat(configPath); err == nil {
		cfg = config.MustLoad(configPath)
	} else {
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			log.Fatalf("cannot read config from env: %s", err)
		}
	}

	log := mustMakeLogger(cfg.LogLevel)

	log.Info("starting server")
	log.Debug("debug messages are enabled")

	storage, err := db.New(log, cfg.DBAddress)
	if err != nil {
		log.Error("failed to create storage", "error", err)
		return
	}

	if err := storage.CleanMigrations(); err != nil {
		log.Warn("failed to clean migrations, continuing...", "error", err)
	}

	if err := storage.Migrate(); err != nil {
		log.Error("failed to migrate db: %v", err)
		return
	}

	service, err := core.NewService(log, storage)
	if err != nil {
		log.Error("failed to create service", "error", err)
		return
	}

	// Инициализация HTTP сервера
	handler := rest.NewHandler(service, log)
	server := &http.Server{
		Addr:    cfg.HTTPConfig.Address,
		Handler: handler,
	}

	log.Info("server running", "address", cfg.HTTPConfig.Address)
	if err := server.ListenAndServe(); err != nil {
		log.Error("server failed", "error", err)
	}
}

func mustMakeLogger(logLevel string) *slog.Logger {
	var level slog.Level
	switch logLevel {
	case "DEBUG":
		level = slog.LevelDebug
	case "INFO":
		level = slog.LevelInfo
	case "ERROR":
		level = slog.LevelError
	default:
		panic("unknown log level: " + logLevel)
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level, AddSource: true})
	return slog.New(handler)
}
