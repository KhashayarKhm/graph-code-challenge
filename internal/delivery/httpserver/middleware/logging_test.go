package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"graph-code-challenge/internal/delivery/httpserver/middleware"
	"graph-code-challenge/internal/requestid"
)

type logEntry struct {
	level      string
	message    string
	attributes []any
	requestID  string
}

type loggerStub struct {
	entries []logEntry
}

func (l *loggerStub) Debug(ctx context.Context, message string, attributes ...any) {
	l.add("debug", ctx, message, attributes)
}

func (l *loggerStub) Info(ctx context.Context, message string, attributes ...any) {
	l.add("info", ctx, message, attributes)
}

func (l *loggerStub) Warn(ctx context.Context, message string, attributes ...any) {
	l.add("warn", ctx, message, attributes)
}

func (l *loggerStub) Error(ctx context.Context, message string, attributes ...any) {
	l.add("error", ctx, message, attributes)
}

func (l *loggerStub) add(level string, ctx context.Context, message string, attributes []any) {
	l.entries = append(l.entries, logEntry{
		level:      level,
		message:    message,
		attributes: attributes,
		requestID:  requestid.FromContext(ctx),
	})
}

func TestRequestIDPropagatesExistingID(t *testing.T) {
	router := gin.New()
	router.Use(middleware.RequestID())
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, requestid.FromContext(c.Request.Context()))
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(requestid.Header, "upstream-42")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if got := recorder.Header().Get(requestid.Header); got != "upstream-42" {
		t.Errorf("response request ID = %q, want %q", got, "upstream-42")
	}
	if got := recorder.Body.String(); got != "upstream-42" {
		t.Errorf("context request ID = %q, want %q", got, "upstream-42")
	}
}

func TestRequestIDReplacesInvalidID(t *testing.T) {
	router := gin.New()
	router.Use(middleware.RequestID())
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(requestid.Header, "line one\nline two")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	id := recorder.Header().Get(requestid.Header)
	if id == "" || strings.ContainsAny(id, " \r\n") {
		t.Fatalf("generated request ID = %q, want a safe non-empty value", id)
	}
}

func TestRequestLoggerRecordsRouteAndRequestID(t *testing.T) {
	log := &loggerStub{}
	router := gin.New()
	router.Use(middleware.RequestID(), middleware.RequestLogger(log))
	router.GET("/tasks/:id", func(c *gin.Context) { c.Status(http.StatusAccepted) })

	request := httptest.NewRequest(http.MethodGet, "/tasks/7", nil)
	request.Header.Set(requestid.Header, "request-7")
	router.ServeHTTP(httptest.NewRecorder(), request)

	if len(log.entries) != 1 {
		t.Fatalf("log entries = %d, want 1", len(log.entries))
	}
	if log.entries[0].level != "info" || log.entries[0].message != "http request" {
		t.Errorf("entry = %+v, want info HTTP request", log.entries[0])
	}
	if log.entries[0].requestID != "request-7" {
		t.Errorf("request ID = %q, want %q", log.entries[0].requestID, "request-7")
	}
	if !attributesContain(log.entries[0].attributes, "path", "/tasks/:id") {
		t.Errorf("attributes = %v, want normalized route", log.entries[0].attributes)
	}
}

func TestRecoveryLogsPanicAndReturnsJSON(t *testing.T) {
	log := &loggerStub{}
	router := gin.New()
	router.Use(middleware.RequestID(), middleware.RequestLogger(log), middleware.Recovery(log))
	router.GET("/panic", func(*gin.Context) { panic("boom") })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Errorf("Content-Type = %q, want JSON", contentType)
	}
	if len(log.entries) != 2 || log.entries[0].level != "error" || log.entries[1].level != "info" {
		t.Fatalf("entries = %+v, want error followed by request info", log.entries)
	}
}

func attributesContain(attributes []any, key string, value any) bool {
	for index := 0; index+1 < len(attributes); index += 2 {
		if attributes[index] == key && attributes[index+1] == value {
			return true
		}
	}

	return false
}
