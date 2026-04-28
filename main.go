// @title           Ledger Query Service API
// @version         1.0
// @description     Read-side of the CQRS ledger: balances, transaction history, statements.
// @termsOfService  http://example.com/terms/

// @contact.name   Ledger Team
// @contact.email  ledger@example.com

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8081
// @BasePath  /api

//go:generate swag init -g main.go -o docs
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"

	"go-ledger-query-service/config"
	_ "go-ledger-query-service/docs"
	"go-ledger-query-service/internal/api"
	kafkapkg "go-ledger-query-service/internal/kafka"
	"go-ledger-query-service/internal/repository"
	"go-ledger-query-service/internal/services"
)

func main() {
	cfg := config.Load()

	logrus.Infof("Starting %s (env=%s port=%s)", config.ServiceName, cfg.Environment, cfg.Port)

	// ── Database ──────────────────────────────────────────────────────────────
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.DBSSLMode,
	)

	db, err := sqlx.Open("pgx", dsn)
	if err != nil {
		logrus.Fatalf("cannot initialize database connection: %v", err)
	}
	db.SetMaxOpenConns(cfg.DBConns)

	if err := db.PingContext(context.Background()); err != nil {
		logrus.Fatalf("cannot connect to database: %v", err)
	}
	defer db.Close()
	logrus.Info("database: connected")

	// Repositories
	balanceRepo := repository.NewBalanceRepository(db)
	txnRepo := repository.NewTransactionRepository(db)

	// ── Services ──────────────────────────────────────────────────────────────
	querySvc := services.NewQueryService(balanceRepo, txnRepo)

	// ── Kafka Consumer ────────────────────────────────────────────────────────
	processor := kafkapkg.NewProcessor(balanceRepo, txnRepo)
	consumer := kafkapkg.NewConsumer(cfg.KafkaBootstrapServers, cfg.KafkaTopic, cfg.KafkaGroupID, processor)

	// ── API ───────────────────────────────────────────────────────────────────
	apiServer := api.NewAPI(cfg, querySvc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	apiErrCh := apiServer.Run(ctx)
	consumerErrCh := consumer.Run(ctx)

	// Graceful shutdown on SIGINT / SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		logrus.Infof("received signal %s – shutting down", sig)
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), config.GracefulStopTimeout)
		defer shutdownCancel()
		_ = apiServer.GracefulStop(shutdownCtx)
		_ = consumer.GracefulStop(shutdownCtx)
	case err := <-apiErrCh:
		if err != nil {
			logrus.Fatalf("api server error: %v", err)
		}
	case err := <-consumerErrCh:
		if err != nil {
			logrus.Fatalf("kafka consumer error: %v", err)
		}
	}

	logrus.Info("shutdown complete")
}
