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
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"

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

	sqlDB := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	sqlDB.SetMaxOpenConns(cfg.DBConns)

	db := bun.NewDB(sqlDB, pgdialect.New())
	if cfg.Environment == "local" || cfg.Environment == "dev" {
		db.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
	}

	if err := db.PingContext(context.Background()); err != nil {
		logrus.Fatalf("cannot connect to database: %v", err)
	}
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
