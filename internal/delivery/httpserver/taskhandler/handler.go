package taskhandler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"

	"github.com/gin-gonic/gin"

	"graph-code-challenge/internal/entity"
	"graph-code-challenge/internal/param"
	"graph-code-challenge/internal/validator/taskvalidator"
)

const (
	MsgInvalidBody  = "request body is not valid JSON"
	MsgInvalidQuery = "query parameters are not valid"
	MsgInvalidID    = "id must be a positive number"
	MsgNotFound     = "task not found"
	MsgInvalidInput = "invalid input"
	MsgInternal     = "internal server error"

	MsgWrongType   = "%s must be %s, got %s"
	MsgNotNumber   = "%s must be a whole number"
	MsgNumberRange = "%s is out of range"
)

var numericQueryParams = []string{"cursor", "limit"}

type TaskService interface {
	Create(ctx context.Context, req param.CreateTaskRequest) (param.CreateTaskResponse, error)
	GetByID(ctx context.Context, req param.GetTaskRequest) (param.GetTaskResponse, error)
	List(ctx context.Context, req param.ListTasksRequest) (param.ListTasksResponse, error)
	Update(ctx context.Context, req param.UpdateTaskRequest) (param.UpdateTaskResponse, error)
	Delete(ctx context.Context, req param.DeleteTaskRequest) (param.DeleteTaskResponse, error)
}

type Handler struct {
	taskSvc       TaskService
	taskValidator taskvalidator.Validator
}

func New(taskSvc TaskService, taskValidator taskvalidator.Validator) Handler {
	return Handler{taskSvc: taskSvc, taskValidator: taskValidator}
}

func (h Handler) SetRoutes(rg *gin.RouterGroup) {
	tasks := rg.Group("/tasks")

	tasks.POST("", h.create)
	tasks.GET("", h.list)
	tasks.GET("/:id", h.get)
	tasks.PATCH("/:id", h.update)
	tasks.DELETE("/:id", h.delete)
}

type ErrorResponse struct {
	Message string            `json:"message"`
	Errors  map[string]string `json:"errors,omitempty"`
}

func (h Handler) create(c *gin.Context) {
	var req param.CreateTaskRequest

	if !bindJSON(c, &req) {
		return
	}

	if fields, err := h.taskValidator.ValidateCreateRequest(req); err != nil {
		writeError(c, err, fields)

		return
	}

	resp, err := h.taskSvc.Create(c.Request.Context(), req)
	if err != nil {
		writeError(c, err, nil)

		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h Handler) get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	resp, err := h.taskSvc.GetByID(c.Request.Context(), param.GetTaskRequest{ID: id})
	if err != nil {
		writeError(c, err, nil)

		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h Handler) list(c *gin.Context) {
	var req param.ListTasksRequest

	if !bindQuery(c, &req) {
		return
	}

	if fields, err := h.taskValidator.ValidateListRequest(req); err != nil {
		writeError(c, err, fields)

		return
	}

	resp, err := h.taskSvc.List(c.Request.Context(), req)
	if err != nil {
		writeError(c, err, nil)

		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h Handler) update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req param.UpdateTaskRequest

	if !bindJSON(c, &req) {
		return
	}

	req.ID = id

	if fields, err := h.taskValidator.ValidateUpdateRequest(req); err != nil {
		writeError(c, err, fields)

		return
	}

	resp, err := h.taskSvc.Update(c.Request.Context(), req)
	if err != nil {
		writeError(c, err, nil)

		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h Handler) delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if _, err := h.taskSvc.Delete(c.Request.Context(), param.DeleteTaskRequest{ID: id}); err != nil {
		writeError(c, err, nil)

		return
	}

	c.Status(http.StatusNoContent)
}

func bindJSON(c *gin.Context, req any) bool {
	err := c.ShouldBindJSON(req)
	if err == nil {
		return true
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) && typeErr.Field != "" {
		writeFields(c, map[string]string{
			typeErr.Field: fmt.Sprintf(MsgWrongType, typeErr.Field, jsonTypeName(typeErr.Type), typeErr.Value),
		})

		return false
	}

	writeMessage(c, http.StatusBadRequest, MsgInvalidBody)

	return false
}

func bindQuery(c *gin.Context, req any) bool {
	fields := make(map[string]string)

	for _, name := range numericQueryParams {
		raw := c.Query(name)
		if raw == "" {
			continue
		}

		if _, err := strconv.ParseInt(raw, 10, 64); err != nil {
			fields[name] = numericMessage(name, err)
		}
	}

	if len(fields) > 0 {
		writeFields(c, fields)

		return false
	}

	if err := c.ShouldBindQuery(req); err != nil {
		writeMessage(c, http.StatusBadRequest, MsgInvalidQuery)

		return false
	}

	return true
}

func numericMessage(name string, err error) string {
	if errors.Is(err, strconv.ErrRange) {
		return fmt.Sprintf(MsgNumberRange, name)
	}

	return fmt.Sprintf(MsgNotNumber, name)
}

func jsonTypeName(fieldType reflect.Type) string {
	switch fieldType.Kind() {
	case reflect.String:
		return "a string"
	case reflect.Bool:
		return "a boolean"
	case reflect.Slice, reflect.Array:
		return "an array"
	case reflect.Map, reflect.Struct:
		return "an object"
	default:
		return "a number"
	}
}

func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		writeMessage(c, http.StatusBadRequest, MsgInvalidID)

		return 0, false
	}

	return id, true
}

func writeMessage(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, ErrorResponse{Message: message})
}

func writeFields(c *gin.Context, fields map[string]string) {
	c.AbortWithStatusJSON(http.StatusBadRequest, ErrorResponse{Message: MsgInvalidInput, Errors: fields})
}

func writeError(c *gin.Context, err error, fields map[string]string) {
	switch {
	case errors.Is(err, entity.ErrNotFound):
		c.AbortWithStatusJSON(http.StatusNotFound, ErrorResponse{Message: MsgNotFound})
	case errors.Is(err, entity.ErrValidation):
		c.AbortWithStatusJSON(http.StatusBadRequest, ErrorResponse{Message: MsgInvalidInput, Errors: fields})
	default:
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{Message: MsgInternal})
	}
}
