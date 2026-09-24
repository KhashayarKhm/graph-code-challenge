package taskhandler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"graph-code-challenge/internal/delivery/httpserver/taskhandler"
	"graph-code-challenge/internal/entity"
	"graph-code-challenge/internal/param"
	"graph-code-challenge/internal/validator/taskvalidator"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

type svcStub struct {
	createFn func(context.Context, param.CreateTaskRequest) (param.CreateTaskResponse, error)
	getFn    func(context.Context, param.GetTaskRequest) (param.GetTaskResponse, error)
	listFn   func(context.Context, param.ListTasksRequest) (param.ListTasksResponse, error)
	updateFn func(context.Context, param.UpdateTaskRequest) (param.UpdateTaskResponse, error)
	deleteFn func(context.Context, param.DeleteTaskRequest) (param.DeleteTaskResponse, error)
}

func (s svcStub) Create(ctx context.Context, req param.CreateTaskRequest) (param.CreateTaskResponse, error) {
	if s.createFn == nil {
		panic("unexpected call to Create")
	}

	return s.createFn(ctx, req)
}

func (s svcStub) GetByID(ctx context.Context, req param.GetTaskRequest) (param.GetTaskResponse, error) {
	if s.getFn == nil {
		panic("unexpected call to GetByID")
	}

	return s.getFn(ctx, req)
}

func (s svcStub) List(ctx context.Context, req param.ListTasksRequest) (param.ListTasksResponse, error) {
	if s.listFn == nil {
		panic("unexpected call to List")
	}

	return s.listFn(ctx, req)
}

func (s svcStub) Update(ctx context.Context, req param.UpdateTaskRequest) (param.UpdateTaskResponse, error) {
	if s.updateFn == nil {
		panic("unexpected call to Update")
	}

	return s.updateFn(ctx, req)
}

func (s svcStub) Delete(ctx context.Context, req param.DeleteTaskRequest) (param.DeleteTaskResponse, error) {
	if s.deleteFn == nil {
		panic("unexpected call to Delete")
	}

	return s.deleteFn(ctx, req)
}

func serve(svc taskhandler.TaskService, method, target, body string) *httptest.ResponseRecorder {
	router := gin.New()
	taskhandler.New(svc, taskvalidator.New()).SetRoutes(router.Group("/api/v1"))

	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, into any) {
	t.Helper()

	if err := json.Unmarshal(rec.Body.Bytes(), into); err != nil {
		t.Fatalf("decoding %s: %v", rec.Body.String(), err)
	}
}

func TestCreateReturns201AndTheTask(t *testing.T) {
	svc := svcStub{createFn: func(_ context.Context, req param.CreateTaskRequest) (param.CreateTaskResponse, error) {
		return param.CreateTaskResponse{Task: param.TaskInfo{ID: 1, Title: req.Title, Status: "pending"}}, nil
	}}

	rec := serve(svc, http.MethodPost, "/api/v1/tasks", `{"title":"write handlers"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}

	var resp param.CreateTaskResponse
	decode(t, rec, &resp)

	if resp.Task.ID != 1 || resp.Task.Title != "write handlers" {
		t.Errorf("body = %+v, want the created task", resp.Task)
	}
}

func TestCreateRejectsMalformedJSON(t *testing.T) {
	rec := serve(svcStub{}, http.MethodPost, "/api/v1/tasks", `{"title":`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}

	var resp taskhandler.ErrorResponse
	decode(t, rec, &resp)

	if resp.Message != taskhandler.MsgInvalidBody {
		t.Errorf("message = %q, want %q", resp.Message, taskhandler.MsgInvalidBody)
	}
}

func TestCreateValidationReturns400WithFieldErrors(t *testing.T) {
	rec := serve(svcStub{}, http.MethodPost, "/api/v1/tasks", `{"title":"","status":"archived"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}

	var resp taskhandler.ErrorResponse
	decode(t, rec, &resp)

	if resp.Errors["title"] == "" || resp.Errors["status"] == "" {
		t.Errorf("errors = %v, want entries for title and status", resp.Errors)
	}
}

func TestCreateServiceFailureReturns500WithoutLeakingDetail(t *testing.T) {
	svc := svcStub{createFn: func(context.Context, param.CreateTaskRequest) (param.CreateTaskResponse, error) {
		return param.CreateTaskResponse{}, errors.New("pq: connection refused on 10.0.0.5")
	}}

	rec := serve(svc, http.MethodPost, "/api/v1/tasks", `{"title":"x"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}

	if strings.Contains(rec.Body.String(), "10.0.0.5") {
		t.Errorf("body leaks internal detail: %s", rec.Body.String())
	}
}

func TestGetReturnsTheTask(t *testing.T) {
	svc := svcStub{getFn: func(_ context.Context, req param.GetTaskRequest) (param.GetTaskResponse, error) {
		return param.GetTaskResponse{Task: param.TaskInfo{ID: req.ID, Title: "found"}}, nil
	}}

	rec := serve(svc, http.MethodGet, "/api/v1/tasks/42", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp param.GetTaskResponse
	decode(t, rec, &resp)

	if resp.Task.ID != 42 {
		t.Errorf("task id = %d, want 42 (taken from the path)", resp.Task.ID)
	}
}

func TestGetNotFoundReturns404(t *testing.T) {
	svc := svcStub{getFn: func(context.Context, param.GetTaskRequest) (param.GetTaskResponse, error) {
		return param.GetTaskResponse{}, entity.ErrTaskNotFound
	}}

	rec := serve(svc, http.MethodGet, "/api/v1/tasks/7", "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestBadPathIDReturns400WithoutCallingTheService(t *testing.T) {
	for _, target := range []string{"/api/v1/tasks/abc", "/api/v1/tasks/0", "/api/v1/tasks/-3"} {
		rec := serve(svcStub{}, http.MethodGet, target, "")

		if rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s status = %d, want 400", target, rec.Code)
		}
	}
}

func TestListPassesQueryParameters(t *testing.T) {
	var got param.ListTasksRequest

	svc := svcStub{listFn: func(_ context.Context, req param.ListTasksRequest) (param.ListTasksResponse, error) {
		got = req

		return param.ListTasksResponse{Tasks: []param.TaskInfo{}}, nil
	}}

	rec := serve(svcStub(svc), http.MethodGet, "/api/v1/tasks?status=done&assignee=kh&cursor=9&limit=5", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	want := param.ListTasksRequest{Status: entity.TaskStatusDone, Assignee: "kh", Cursor: 9, Limit: 5}
	if got != want {
		t.Errorf("service received %+v, want %+v", got, want)
	}
}

func TestListRejectsUnknownStatus(t *testing.T) {
	rec := serve(svcStub{}, http.MethodGet, "/api/v1/tasks?status=archived", "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}

	var resp taskhandler.ErrorResponse
	decode(t, rec, &resp)

	if resp.Errors["status"] == "" {
		t.Errorf("errors = %v, want an entry for status", resp.Errors)
	}
}

func TestListEmptyMarshalsAsArrayNotNull(t *testing.T) {
	svc := svcStub{listFn: func(context.Context, param.ListTasksRequest) (param.ListTasksResponse, error) {
		return param.ListTasksResponse{Tasks: []param.TaskInfo{}}, nil
	}}

	rec := serve(svc, http.MethodGet, "/api/v1/tasks", "")

	if !strings.Contains(rec.Body.String(), `"tasks":[]`) {
		t.Errorf("body = %s, want tasks as [] rather than null", rec.Body.String())
	}
}

func TestUpdateTakesIDFromPathNotBody(t *testing.T) {
	var got param.UpdateTaskRequest

	svc := svcStub{updateFn: func(_ context.Context, req param.UpdateTaskRequest) (param.UpdateTaskResponse, error) {
		got = req

		return param.UpdateTaskResponse{Task: param.TaskInfo{ID: req.ID}}, nil
	}}

	rec := serve(svc, http.MethodPatch, "/api/v1/tasks/42", `{"id":99,"title":"patched"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	if got.ID != 42 {
		t.Errorf("service received id %d, want 42: the body must not override the path", got.ID)
	}
}

func TestUpdateOmittedFieldsStayNil(t *testing.T) {
	var got param.UpdateTaskRequest

	svc := svcStub{updateFn: func(_ context.Context, req param.UpdateTaskRequest) (param.UpdateTaskResponse, error) {
		got = req

		return param.UpdateTaskResponse{}, nil
	}}

	serve(svc, http.MethodPatch, "/api/v1/tasks/1", `{"status":"done"}`)

	if got.Status == nil || *got.Status != entity.TaskStatusDone {
		t.Errorf("status = %v, want done", got.Status)
	}

	if got.Title != nil || got.Description != nil || got.Assignee != nil {
		t.Error("omitted fields reached the service as non-nil")
	}
}

func TestUpdateCanClearAFieldExplicitly(t *testing.T) {
	var got param.UpdateTaskRequest

	svc := svcStub{updateFn: func(_ context.Context, req param.UpdateTaskRequest) (param.UpdateTaskResponse, error) {
		got = req

		return param.UpdateTaskResponse{}, nil
	}}

	serve(svc, http.MethodPatch, "/api/v1/tasks/1", `{"description":""}`)

	if got.Description == nil {
		t.Fatal("description = nil, want a pointer to the empty string")
	}

	if *got.Description != "" {
		t.Errorf("description = %q, want empty", *got.Description)
	}
}

func TestUpdateWithNoFieldsReturns400(t *testing.T) {
	rec := serve(svcStub{}, http.MethodPatch, "/api/v1/tasks/1", `{}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}

	var resp taskhandler.ErrorResponse
	decode(t, rec, &resp)

	if resp.Errors["body"] == "" {
		t.Errorf("errors = %v, want an entry explaining nothing was supplied", resp.Errors)
	}
}

func TestDeleteReturns204WithEmptyBody(t *testing.T) {
	var got int64

	svc := svcStub{deleteFn: func(_ context.Context, req param.DeleteTaskRequest) (param.DeleteTaskResponse, error) {
		got = req.ID

		return param.DeleteTaskResponse{}, nil
	}}

	rec := serve(svc, http.MethodDelete, "/api/v1/tasks/8", "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}

	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty for 204", rec.Body.String())
	}

	if got != 8 {
		t.Errorf("service received id %d, want 8", got)
	}
}

func TestDeleteNotFoundReturns404(t *testing.T) {
	svc := svcStub{deleteFn: func(context.Context, param.DeleteTaskRequest) (param.DeleteTaskResponse, error) {
		return param.DeleteTaskResponse{}, entity.ErrTaskNotFound
	}}

	rec := serve(svc, http.MethodDelete, "/api/v1/tasks/8", "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
