package httpserver

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Server struct {
	router *gin.Engine
}

func New() *Server {
	return &Server{}
}

func (s *Server) Setup() {
	s.router = gin.Default()

	s.router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
}

func (s *Server) Serve(addr string) {
	s.router.Run(addr)
}
