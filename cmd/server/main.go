package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/kahook/internal/auth"
	"github.com/kahook/internal/config"
	"github.com/kahook/internal/kafka"
	"github.com/kahook/internal/server"
	"github.com/kahook/internal/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version.String())
		os.Exit(0)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync() //nolint:errcheck // best-effort flush on exit

	logger.Info("starting kahook",
		zap.String("version", version.Version),
		zap.String("git_commit", version.GitCommit),
		zap.String("build_time", version.BuildTime),
	)

	cfg, err := config.Load(getConfigPath())
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	logger.Info("configuration loaded",
		zap.Int("port", cfg.Server.Port),
		zap.Strings("kafka_brokers", cfg.Kafka.Brokers),
	)

	producer, err := kafka.NewProducer(kafka.ProducerConfig{
		ConfigMap: cfg.KafkaConfigMap(),
		Logger:    logger,
	})
	if err != nil {
		logger.Fatal("failed to create kafka producer", zap.Error(err))
	}
	defer producer.Close()

	logger.Info("kafka producer created",
		zap.Strings("brokers", cfg.Kafka.Brokers),
	)

	// Build the user map for basic auth.
	users := make(map[string]string)
	for _, u := range cfg.Auth.Users {
		users[u.Username] = u.Password
	}

	authenticator := auth.NewMultiAuth(users, cfg.Auth.Tokens)

	if authenticator.HasAuth() {
		if len(users) > 0 {
			logger.Info("basic auth enabled", zap.Int("users", len(users)))
		}
		if len(cfg.Auth.Tokens) > 0 {
			logger.Info("bearer auth enabled", zap.Int("tokens", len(cfg.Auth.Tokens)))
		}
	} else {
		logger.Warn("no authentication configured")
	}

	srv := server.NewServer(server.ServerConfig{
		Port:         cfg.Server.Port,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
		Producer:     producer,
		Auth:         authenticator,
		Logger:       logger,
	})

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	sig := <-stop
	logger.Info("shutdown signal received", zap.String("signal", sig.String()))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}

	logger.Info("server stopped gracefully")
}

func getConfigPath() string {
	return os.Getenv("CONFIG_PATH")
}
