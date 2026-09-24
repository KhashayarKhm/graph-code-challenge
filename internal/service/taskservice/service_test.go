package taskservice_test

import (
	"context"
	"errors"
	"testing"

	"graph-code-challenge/internal/entity"
	"graph-code-challenge/internal/param"
	"graph-code-challenge/internal/service/taskservice"
)

type repoStub struct {
	createFn  func(context.Context, entity.Task) (entity.Task, error)
	getByIDFn func(context.Context, int64) (entity.Task, error)
	listFn    func(context.Context, param.ListTasksRequest) ([]entity.Task, error)
	updateFn  func(context.Context, param.UpdateTaskRequest) (entity.Task, error)
	deleteFn  func(context.Context, int64) error
	countFn   func(context.Context) (int64, error)
}

func (r repoStub) Create(ctx context.Context, task entity.Task) (entity.Task, error) {
	if r.createFn == nil {
		panic("unexpected call to Create")
	}

	return r.createFn(ctx, task)
}

func (r repoStub) GetByID(ctx context.Context, id int64) (entity.Task, error) {
	if r.getByIDFn == nil {
		panic("unexpected call to GetByID")
	}

	return r.getByIDFn(ctx, id)
}

func (r repoStub) List(ctx context.Context, req param.ListTasksRequest) ([]entity.Task, error) {
	if r.listFn == nil {
		panic("unexpected call to List")
	}

	return r.listFn(ctx, req)
}

func (r repoStub) Update(ctx context.Context, req param.UpdateTaskRequest) (entity.Task, error) {
	if r.updateFn == nil {
		panic("unexpected call to Update")
	}

	return r.updateFn(ctx, req)
}

func (r repoStub) Delete(ctx context.Context, id int64) error {
	if r.deleteFn == nil {
		panic("unexpected call to Delete")
	}

	return r.deleteFn(ctx, id)
}

func (r repoStub) Count(ctx context.Context) (int64, error) {
	if r.countFn == nil {
		panic("unexpected call to Count")
	}

	return r.countFn(ctx)
}

func ptr[T any](value T) *T {
	return &value
}

func tasksWithIDs(ids ...int64) []entity.Task {
	tasks := make([]entity.Task, 0, len(ids))
	for _, id := range ids {
		tasks = append(tasks, entity.Task{ID: id})
	}

	return tasks
}

func TestCreateDefaultsStatusToPending(t *testing.T) {
	var got entity.Task

	repo := repoStub{createFn: func(_ context.Context, task entity.Task) (entity.Task, error) {
		got = task
		task.ID = 7

		return task, nil
	}}

	resp, err := taskservice.New(repo).Create(context.Background(), param.CreateTaskRequest{Title: "write tests"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if got.Status != entity.TaskStatusPending {
		t.Errorf("repository received status %q, want %q", got.Status, entity.TaskStatusPending)
	}

	if resp.Task.ID != 7 {
		t.Errorf("Create() task id = %d, want 7 (returned by the repository)", resp.Task.ID)
	}

	if resp.Task.Status != string(entity.TaskStatusPending) {
		t.Errorf("Create() task status = %q, want %q", resp.Task.Status, entity.TaskStatusPending)
	}
}

func TestCreateKeepsExplicitStatus(t *testing.T) {
	var got entity.Task

	repo := repoStub{createFn: func(_ context.Context, task entity.Task) (entity.Task, error) {
		got = task

		return task, nil
	}}

	req := param.CreateTaskRequest{Title: "ship it", Description: "d", Status: entity.TaskStatusDone, Assignee: "kh"}

	if _, err := taskservice.New(repo).Create(context.Background(), req); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	want := entity.Task{Title: "ship it", Description: "d", Status: entity.TaskStatusDone, Assignee: "kh"}
	if got != want {
		t.Errorf("repository received %+v, want %+v", got, want)
	}
}

func TestCreatePropagatesRepositoryError(t *testing.T) {
	wantErr := errors.New("connection refused")

	repo := repoStub{createFn: func(context.Context, entity.Task) (entity.Task, error) {
		return entity.Task{}, wantErr
	}}

	_, err := taskservice.New(repo).Create(context.Background(), param.CreateTaskRequest{Title: "x"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Create() error = %v, want it to wrap %v", err, wantErr)
	}
}

func TestGetByIDReturnsTaskInfo(t *testing.T) {
	repo := repoStub{getByIDFn: func(_ context.Context, id int64) (entity.Task, error) {
		return entity.Task{ID: id, Title: "found", Status: entity.TaskStatusInProgress}, nil
	}}

	resp, err := taskservice.New(repo).GetByID(context.Background(), param.GetTaskRequest{ID: 42})
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if resp.Task.ID != 42 || resp.Task.Title != "found" {
		t.Errorf("GetByID() = %+v, want id 42 titled \"found\"", resp.Task)
	}
}

func TestGetByIDPropagatesNotFound(t *testing.T) {
	repo := repoStub{getByIDFn: func(context.Context, int64) (entity.Task, error) {
		return entity.Task{}, entity.ErrTaskNotFound
	}}

	_, err := taskservice.New(repo).GetByID(context.Background(), param.GetTaskRequest{ID: 9})
	if !errors.Is(err, entity.ErrNotFound) {
		t.Fatalf("GetByID() error = %v, want it to wrap ErrNotFound", err)
	}
}

func TestListClampsLimitAndRequestsOneExtraRow(t *testing.T) {
	tests := map[string]struct {
		limit     int
		wantAsked int
	}{
		"unset uses default":     {0, param.DefaultTaskListLimit + 1},
		"negative uses default":  {-5, param.DefaultTaskListLimit + 1},
		"under the cap is kept":  {10, 11},
		"over the cap is capped": {500, param.MaxTaskListLimit + 1},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var asked int

			repo := repoStub{listFn: func(_ context.Context, req param.ListTasksRequest) ([]entity.Task, error) {
				asked = req.Limit

				return nil, nil
			}}

			if _, err := taskservice.New(repo).List(context.Background(), param.ListTasksRequest{Limit: tc.limit}); err != nil {
				t.Fatalf("List() error = %v", err)
			}

			if asked != tc.wantAsked {
				t.Errorf("repository asked for limit %d, want %d", asked, tc.wantAsked)
			}
		})
	}
}

func TestListTrimsExtraRowAndSetsCursor(t *testing.T) {
	repo := repoStub{listFn: func(context.Context, param.ListTasksRequest) ([]entity.Task, error) {
		return tasksWithIDs(9, 8, 7), nil
	}}

	resp, err := taskservice.New(repo).List(context.Background(), param.ListTasksRequest{Limit: 2})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(resp.Tasks) != 2 {
		t.Fatalf("List() returned %d tasks, want 2", len(resp.Tasks))
	}

	if !resp.HasMore {
		t.Error("List() HasMore = false, want true")
	}

	if resp.NextCursor != 8 {
		t.Errorf("List() NextCursor = %d, want 8 (id of the last kept row)", resp.NextCursor)
	}
}

func TestListWithoutExtraRowHasNoCursor(t *testing.T) {
	repo := repoStub{listFn: func(context.Context, param.ListTasksRequest) ([]entity.Task, error) {
		return tasksWithIDs(9, 8), nil
	}}

	resp, err := taskservice.New(repo).List(context.Background(), param.ListTasksRequest{Limit: 2})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(resp.Tasks) != 2 || resp.HasMore || resp.NextCursor != 0 {
		t.Errorf("List() = %+v, want 2 tasks, HasMore false, NextCursor 0", resp)
	}
}

func TestListEmptyResultIsEmptySliceNotNil(t *testing.T) {
	repo := repoStub{listFn: func(context.Context, param.ListTasksRequest) ([]entity.Task, error) {
		return nil, nil
	}}

	resp, err := taskservice.New(repo).List(context.Background(), param.ListTasksRequest{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if resp.Tasks == nil {
		t.Error("List() Tasks = nil, want an empty slice so it marshals as [] not null")
	}
}

func TestListPassesFiltersThrough(t *testing.T) {
	var got param.ListTasksRequest

	repo := repoStub{listFn: func(_ context.Context, req param.ListTasksRequest) ([]entity.Task, error) {
		got = req

		return nil, nil
	}}

	req := param.ListTasksRequest{Status: entity.TaskStatusInProgress, Assignee: "khashayar", Cursor: 31, Limit: 5}

	if _, err := taskservice.New(repo).List(context.Background(), req); err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if got.Status != req.Status || got.Assignee != req.Assignee || got.Cursor != req.Cursor {
		t.Errorf("repository received %+v, want status/assignee/cursor from %+v", got, req)
	}
}

func TestUpdatePassesThePatchThrough(t *testing.T) {
	var got param.UpdateTaskRequest

	repo := repoStub{updateFn: func(_ context.Context, req param.UpdateTaskRequest) (entity.Task, error) {
		got = req

		return entity.Task{ID: req.ID, Title: *req.Title, Status: entity.TaskStatusDone}, nil
	}}

	req := param.UpdateTaskRequest{ID: 3, Title: ptr("only the title")}

	resp, err := taskservice.New(repo).Update(context.Background(), req)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if got.ID != 3 || got.Title == nil || *got.Title != "only the title" {
		t.Errorf("repository received %+v, want id 3 with title set", got)
	}

	if got.Description != nil || got.Status != nil || got.Assignee != nil {
		t.Error("repository received non-nil fields for keys the caller omitted")
	}

	if resp.Task.ID != 3 {
		t.Errorf("Update() task id = %d, want 3", resp.Task.ID)
	}
}

func TestUpdatePropagatesNotFound(t *testing.T) {
	repo := repoStub{updateFn: func(context.Context, param.UpdateTaskRequest) (entity.Task, error) {
		return entity.Task{}, entity.ErrTaskNotFound
	}}

	req := param.UpdateTaskRequest{ID: 99, Title: ptr("x")}

	_, err := taskservice.New(repo).Update(context.Background(), req)
	if !errors.Is(err, entity.ErrNotFound) {
		t.Fatalf("Update() error = %v, want it to wrap ErrNotFound", err)
	}
}

func TestDeleteDelegates(t *testing.T) {
	var got int64

	repo := repoStub{deleteFn: func(_ context.Context, id int64) error {
		got = id

		return nil
	}}

	if _, err := taskservice.New(repo).Delete(context.Background(), param.DeleteTaskRequest{ID: 12}); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if got != 12 {
		t.Errorf("repository received id %d, want 12", got)
	}
}

func TestDeletePropagatesNotFound(t *testing.T) {
	repo := repoStub{deleteFn: func(context.Context, int64) error { return entity.ErrTaskNotFound }}

	_, err := taskservice.New(repo).Delete(context.Background(), param.DeleteTaskRequest{ID: 4})
	if !errors.Is(err, entity.ErrNotFound) {
		t.Fatalf("Delete() error = %v, want it to wrap ErrNotFound", err)
	}
}

func TestCountDelegates(t *testing.T) {
	repo := repoStub{countFn: func(context.Context) (int64, error) { return 5, nil }}

	count, err := taskservice.New(repo).Count(context.Background())
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}

	if count != 5 {
		t.Errorf("Count() = %d, want 5", count)
	}
}

func TestCountPropagatesError(t *testing.T) {
	wantErr := errors.New("boom")

	repo := repoStub{countFn: func(context.Context) (int64, error) { return 0, wantErr }}

	if _, err := taskservice.New(repo).Count(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("Count() error = %v, want it to wrap %v", err, wantErr)
	}
}
