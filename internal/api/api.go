package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go-ledger-query-service/config"
	"go-ledger-query-service/internal/services"
)

// API is the main HTTP server struct for the query service.
type API struct {
	cfg      *config.Config
	router   *gin.Engine
	server   *http.Server
	querySvc services.QueryService
}

// NewAPI creates and configures the query API server.
func NewAPI(cfg *config.Config, querySvc services.QueryService) *API {
	a := &API{cfg: cfg, querySvc: querySvc}
	a.initialize()
	return a
}

// Title satisfies the Module interface.
func (a *API) Title() string { return "HTTP REST API (query)" }

// GracefulStop shuts down the HTTP server.
func (a *API) GracefulStop(ctx context.Context) error {
	return a.server.Shutdown(ctx)
}

// Run starts the server and returns a channel that receives any fatal error.
func (a *API) Run(_ context.Context) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		addr := fmt.Sprintf("%s:%s", a.cfg.Host, a.cfg.Port)
		logrus.Infof("query service: listening on %s", addr)
		a.server = &http.Server{Addr: addr, Handler: a.router}
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()
	return errCh
}

func (a *API) initialize() {
	if a.cfg.Environment == "production" || a.cfg.Environment == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "OPTIONS"},
		AllowHeaders: []string{"Authorization", "Content-Type"},
	}))

	// Swagger
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	base := router.Group("/api")
	base.GET("/ping", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	base.GET("/livez", func(c *gin.Context) { c.Status(http.StatusOK) })
	base.GET("/readyz", func(c *gin.Context) { c.Status(http.StatusOK) })
	base.GET("/info", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"service": config.ServiceName, "version": "1.0.0"})
	})

	v1 := base.Group("/v1")

	ah := NewAccountQueryHandler(a.querySvc)
	v1.GET("/accounts", ah.ListAccountsByOwner)
	v1.GET("/accounts/:accountId/balance", ah.GetBalance)
	v1.GET("/accounts/:accountId/transactions", ah.ListTransactions)
	v1.GET("/accounts/:accountId/statement", ah.GetStatement)

	a.router = router
}
