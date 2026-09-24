package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"graph-code-challenge/internal/config"
	"graph-code-challenge/internal/delivery/httpserver/middleware"
	"graph-code-challenge/internal/delivery/httpserver/taskhandler"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
)

type Metrics interface {
	middleware.Recorder
	Handler() http.Handler
}

type Server struct {
	config      config.Config
	taskHandler taskhandler.Handler
	metrics     Metrics
	router      *gin.Engine
	httpServer  *http.Server
}

func New(cfg config.Config, taskHandler taskhandler.Handler, metrics Metrics) *Server {
	return &Server{config: cfg, taskHandler: taskHandler, metrics: metrics}
}

func (s *Server) Setup() {
	if s.config.AppMode == config.AppModeProduction {
		gin.SetMode(gin.ReleaseMode)
	}

	s.router = gin.New()
	s.router.Use(
		gin.LoggerWithConfig(gin.LoggerConfig{
			SkipPaths: []string{"/metrics"},
		}),
		gin.Recovery(),
	)

	s.router.GET("/metrics", gin.WrapH(s.metrics.Handler()))

	s.router.Use(middleware.Metrics(s.metrics))

	s.router.GET("/healthz", s.healthCheck)

	s.taskHandler.SetRoutes(s.router.Group("/api/v1"))

	s.httpServer = &http.Server{
		Addr:              s.config.Addr(),
		Handler:           s.router.Handler(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

func (s *Server) Router() *gin.Engine {
	return s.router
}

func (s *Server) Serve() error {
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("httpserver: serve: %w", err)
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("httpserver: shutdown: %w", err)
	}

	return nil
}

func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
