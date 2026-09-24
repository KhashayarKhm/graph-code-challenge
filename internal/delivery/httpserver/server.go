package httpserver

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"graph-code-challenge/internal/config"
	"graph-code-challenge/internal/delivery/httpserver/taskhandler"
)

type Server struct {
	config      config.Config
	taskHandler taskhandler.Handler
	router      *gin.Engine
}

func New(cfg config.Config, taskHandler taskhandler.Handler) *Server {
	return &Server{config: cfg, taskHandler: taskHandler}
}

func (s *Server) Setup() {
	if s.config.AppMode == config.AppModeProduction {
		gin.SetMode(gin.ReleaseMode)
	}

	s.router = gin.New()
	s.router.Use(gin.Logger(), gin.Recovery())

	s.router.GET("/healthz", s.healthCheck)

	s.taskHandler.SetRoutes(s.router.Group("/api/v1"))
}

func (s *Server) Router() *gin.Engine {
	return s.router
}

func (s *Server) Serve() error {
	return s.router.Run(s.config.Addr())
}

func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
