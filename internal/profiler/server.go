package profiler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	"time"
)

const readHeaderTimeout = 5 * time.Second

type Server struct {
	httpServer *http.Server
}

func New(addr string) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:              addr,
			Handler:           newHandler(),
			ReadHeaderTimeout: readHeaderTimeout,
		},
	}
}

func (s *Server) Serve() error {
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("profiler: serve: %w", err)
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("profiler: shutdown: %w", err)
	}

	return nil
}

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	for _, name := range []string{"allocs", "block", "goroutine", "heap", "mutex", "threadcreate"} {
		mux.Handle("/debug/pprof/"+name, pprof.Handler(name))
	}

	return mux
}
