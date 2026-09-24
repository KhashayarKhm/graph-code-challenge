//go:build integration

package postgrestask_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"graph-code-challenge/internal/config"
	"graph-code-challenge/internal/entity"
	"graph-code-challenge/internal/migrator"
	"graph-code-challenge/internal/param"
	"graph-code-challenge/internal/repository/postgres"
	"graph-code-challenge/internal/repository/postgres/postgrestask"
	"graph-code-challenge/internal/service/taskservice"
)

var _ taskservice.Repository = (*postgrestask.DB)(nil)

func requireTestDatabase(t *testing.T, dsn string) {
	t.Helper()

	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("DATABASE_URL is not a valid URL: %v", err)
	}

	name := strings.TrimPrefix(parsed.Path, "/")
	if !strings.HasSuffix(name, "_test") {
		t.Fatalf("refusing to run integration tests against database %q: these tests migrate and delete rows, so the name must end in _test (use ENV_FILE=.env.test, or make test-integration)", name)
	}
}

func newRepo(t *testing.T) (*postgrestask.DB, string) {
	t.Helper()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() = %v", err)
	}

	requireTestDatabase(t, cfg.DatabaseURL)

	m, err := migrator.New(cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("migrator.New() = %v", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrator.ErrNoChange) {
		t.Fatalf("migrator.Up() = %v", err)
	}

	m.Close()

	ctx := context.Background()

	db, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("postgres.New() = %v", err)
	}

	assignee := fmt.Sprintf("it-%d", time.Now().UnixNano())

	t.Cleanup(func() {
		if _, err := db.Pool().Exec(ctx, `DELETE FROM tasks WHERE assignee = $1`, assignee); err != nil {
			t.Errorf("cleanup failed for assignee %s: %v", assignee, err)
		}

		db.Close()
	})

	return postgrestask.New(db), assignee
}

func seed(t *testing.T, repo *postgrestask.DB, assignee, title string, status entity.TaskStatus) entity.Task {
	t.Helper()

	task, err := repo.Create(context.Background(), entity.Task{
		Title:       title,
		Description: "seeded by the integration test",
		Status:      status,
		Assignee:    assignee,
	})
	if err != nil {
		t.Fatalf("Create() = %v", err)
	}

	if task.ID < 1 {
		t.Fatalf("Create() returned id %d, want a positive id from RETURNING", task.ID)
	}

	return task
}

func TestCreateAndGetByIDRoundTrip(t *testing.T) {
	repo, assignee := newRepo(t)
	ctx := context.Background()

	created := seed(t, repo, assignee, "round trip", entity.TaskStatusInProgress)

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID() = %v", err)
	}

	if got != created {
		t.Errorf("GetByID() = %+v, want %+v", got, created)
	}

	if got.Status != entity.TaskStatusInProgress {
		t.Errorf("status round-tripped as %q, want %q", got.Status, entity.TaskStatusInProgress)
	}
}

func TestGetByIDMissingReturnsNotFound(t *testing.T) {
	repo, _ := newRepo(t)

	_, err := repo.GetByID(context.Background(), 999_000_111)
	if !errors.Is(err, entity.ErrNotFound) {
		t.Fatalf("GetByID(missing) = %v, want ErrTaskNotFound", err)
	}
}

func ptr[T any](value T) *T {
	return &value
}

func TestUpdateAppliesOnlyTheSuppliedFields(t *testing.T) {
	repo, assignee := newRepo(t)
	ctx := context.Background()

	created := seed(t, repo, assignee, "before", entity.TaskStatusPending)

	updated, err := repo.Update(ctx, param.UpdateTaskRequest{ID: created.ID, Status: ptr(entity.TaskStatusDone)})
	if err != nil {
		t.Fatalf("Update() = %v", err)
	}

	want := entity.Task{
		ID:          created.ID,
		Title:       created.Title,
		Description: created.Description,
		Status:      entity.TaskStatusDone,
		Assignee:    created.Assignee,
	}

	if updated != want {
		t.Errorf("Update() = %+v, want %+v (only status changed)", updated, want)
	}

	reread, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID() after update = %v", err)
	}

	if reread != want {
		t.Errorf("re-read = %+v, want %+v", reread, want)
	}
}

func TestUpdateCanClearAField(t *testing.T) {
	repo, assignee := newRepo(t)
	ctx := context.Background()

	created := seed(t, repo, assignee, "has a description", entity.TaskStatusPending)

	updated, err := repo.Update(ctx, param.UpdateTaskRequest{ID: created.ID, Description: ptr("")})
	if err != nil {
		t.Fatalf("Update() = %v", err)
	}

	if updated.Description != "" {
		t.Errorf("description = %q, want it cleared by an explicit empty string", updated.Description)
	}

	if updated.Title != created.Title {
		t.Errorf("title = %q, want it untouched at %q", updated.Title, created.Title)
	}
}

func TestUpdateEveryFieldAtOnce(t *testing.T) {
	repo, assignee := newRepo(t)
	ctx := context.Background()

	created := seed(t, repo, assignee, "before", entity.TaskStatusPending)

	updated, err := repo.Update(ctx, param.UpdateTaskRequest{
		ID:          created.ID,
		Title:       ptr("after"),
		Description: ptr("replaced"),
		Status:      ptr(entity.TaskStatusInProgress),
		Assignee:    ptr(assignee),
	})
	if err != nil {
		t.Fatalf("Update() = %v", err)
	}

	want := entity.Task{
		ID:          created.ID,
		Title:       "after",
		Description: "replaced",
		Status:      entity.TaskStatusInProgress,
		Assignee:    assignee,
	}

	if updated != want {
		t.Errorf("Update() = %+v, want %+v", updated, want)
	}
}

func TestUpdateMissingReturnsNotFound(t *testing.T) {
	repo, _ := newRepo(t)

	_, err := repo.Update(context.Background(), param.UpdateTaskRequest{ID: 999_000_222, Title: ptr("x")})
	if !errors.Is(err, entity.ErrNotFound) {
		t.Fatalf("Update(missing) = %v, want ErrTaskNotFound", err)
	}
}

func TestDeleteIsSoftAndHidesTheRow(t *testing.T) {
	repo, assignee := newRepo(t)
	ctx := context.Background()

	created := seed(t, repo, assignee, "to delete", entity.TaskStatusPending)

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() = %v", err)
	}

	if _, err := repo.GetByID(ctx, created.ID); !errors.Is(err, entity.ErrNotFound) {
		t.Errorf("GetByID() after delete = %v, want ErrTaskNotFound", err)
	}

	if err := repo.Delete(ctx, created.ID); !errors.Is(err, entity.ErrNotFound) {
		t.Errorf("second Delete() = %v, want ErrTaskNotFound", err)
	}

	if _, err := repo.Update(ctx, param.UpdateTaskRequest{ID: created.ID, Title: ptr("x")}); !errors.Is(err, entity.ErrNotFound) {
		t.Errorf("Update() after delete = %v, want ErrTaskNotFound", err)
	}

	listed, err := repo.List(ctx, param.ListTasksRequest{Assignee: assignee, Limit: 10})
	if err != nil {
		t.Fatalf("List() = %v", err)
	}

	if len(listed) != 0 {
		t.Errorf("List() returned %d tasks after the only one was deleted, want 0", len(listed))
	}
}

func TestCountExcludesSoftDeleted(t *testing.T) {
	repo, assignee := newRepo(t)
	ctx := context.Background()

	before, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count() = %v", err)
	}

	first := seed(t, repo, assignee, "counted a", entity.TaskStatusPending)
	seed(t, repo, assignee, "counted b", entity.TaskStatusPending)

	after, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count() = %v", err)
	}

	if after != before+2 {
		t.Fatalf("Count() = %d after creating 2, want %d", after, before+2)
	}

	if err := repo.Delete(ctx, first.ID); err != nil {
		t.Fatalf("Delete() = %v", err)
	}

	afterDelete, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count() = %v", err)
	}

	if afterDelete != before+1 {
		t.Errorf("Count() = %d after soft deleting one, want %d", afterDelete, before+1)
	}
}

func TestListFiltersByStatusAndAssignee(t *testing.T) {
	repo, assignee := newRepo(t)
	ctx := context.Background()

	seed(t, repo, assignee, "pending one", entity.TaskStatusPending)
	seed(t, repo, assignee, "done one", entity.TaskStatusDone)
	seed(t, repo, assignee, "pending two", entity.TaskStatusPending)

	pending, err := repo.List(ctx, param.ListTasksRequest{Assignee: assignee, Status: entity.TaskStatusPending, Limit: 10})
	if err != nil {
		t.Fatalf("List() = %v", err)
	}

	if len(pending) != 2 {
		t.Fatalf("List(status=pending) returned %d tasks, want 2", len(pending))
	}

	for _, task := range pending {
		if task.Status != entity.TaskStatusPending {
			t.Errorf("List(status=pending) returned a %q task", task.Status)
		}
	}

	other, err := repo.List(ctx, param.ListTasksRequest{Assignee: assignee + "-nobody", Limit: 10})
	if err != nil {
		t.Fatalf("List() = %v", err)
	}

	if len(other) != 0 {
		t.Errorf("List(assignee=nobody) returned %d tasks, want 0", len(other))
	}
}

func TestListIsKeysetPaginatedNewestFirst(t *testing.T) {
	repo, assignee := newRepo(t)
	ctx := context.Background()

	var ids []int64
	for i := range 5 {
		ids = append(ids, seed(t, repo, assignee, fmt.Sprintf("page %d", i), entity.TaskStatusPending).ID)
	}

	first, err := repo.List(ctx, param.ListTasksRequest{Assignee: assignee, Limit: 2})
	if err != nil {
		t.Fatalf("List() = %v", err)
	}

	if len(first) != 2 {
		t.Fatalf("first page returned %d tasks, want 2", len(first))
	}

	if first[0].ID != ids[4] || first[1].ID != ids[3] {
		t.Fatalf("first page ids = %d,%d, want %d,%d (newest first)", first[0].ID, first[1].ID, ids[4], ids[3])
	}

	second, err := repo.List(ctx, param.ListTasksRequest{Assignee: assignee, Cursor: first[1].ID, Limit: 2})
	if err != nil {
		t.Fatalf("List() = %v", err)
	}

	if len(second) != 2 {
		t.Fatalf("second page returned %d tasks, want 2", len(second))
	}

	if second[0].ID != ids[2] || second[1].ID != ids[1] {
		t.Errorf("second page ids = %d,%d, want %d,%d", second[0].ID, second[1].ID, ids[2], ids[1])
	}
}

func TestServiceListDerivesCursorFromRepository(t *testing.T) {
	repo, assignee := newRepo(t)
	ctx := context.Background()

	var ids []int64
	for i := range 3 {
		ids = append(ids, seed(t, repo, assignee, fmt.Sprintf("svc %d", i), entity.TaskStatusPending).ID)
	}

	svc := taskservice.New(repo)

	page, err := svc.List(ctx, param.ListTasksRequest{Assignee: assignee, Limit: 2})
	if err != nil {
		t.Fatalf("List() = %v", err)
	}

	if len(page.Tasks) != 2 || !page.HasMore {
		t.Fatalf("page = %+v, want 2 tasks and HasMore true", page)
	}

	if page.NextCursor != ids[1] {
		t.Fatalf("NextCursor = %d, want %d", page.NextCursor, ids[1])
	}

	last, err := svc.List(ctx, param.ListTasksRequest{Assignee: assignee, Cursor: page.NextCursor, Limit: 2})
	if err != nil {
		t.Fatalf("List() = %v", err)
	}

	if len(last.Tasks) != 1 || last.HasMore || last.NextCursor != 0 {
		t.Errorf("last page = %+v, want 1 task, HasMore false, NextCursor 0", last)
	}
}
