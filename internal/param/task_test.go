package param_test

import (
	"encoding/json"
	"testing"

	"graph-code-challenge/internal/entity"
	"graph-code-challenge/internal/param"
)

func TestNewTaskInfo(t *testing.T) {
	task := entity.Task{ID: 3, Title: "t", Description: "d", Status: entity.TaskStatusInProgress, Assignee: "kh"}

	got := param.NewTaskInfo(task)

	want := param.TaskInfo{ID: 3, Title: "t", Description: "d", Status: "in_progress", Assignee: "kh"}
	if got != want {
		t.Errorf("NewTaskInfo() = %+v, want %+v", got, want)
	}
}

func TestTaskInfoJSONKeys(t *testing.T) {
	encoded, err := json.Marshal(param.NewTaskInfo(entity.Task{ID: 1, Status: entity.TaskStatusDone}))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	for _, key := range []string{"id", "title", "description", "status", "assignee"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("marshalled TaskInfo is missing key %q: %s", key, encoded)
		}
	}

	if decoded["status"] != "done" {
		t.Errorf("status = %v, want \"done\" (a plain string, not a nested object)", decoded["status"])
	}
}

func TestListTasksRequestWithClampedLimit(t *testing.T) {
	tests := map[string]struct {
		limit int
		want  int
	}{
		"unset":        {0, param.DefaultTaskListLimit},
		"negative":     {-1, param.DefaultTaskListLimit},
		"one":          {1, 1},
		"under cap":    {param.MaxTaskListLimit - 1, param.MaxTaskListLimit - 1},
		"at cap":       {param.MaxTaskListLimit, param.MaxTaskListLimit},
		"over cap":     {param.MaxTaskListLimit + 1, param.MaxTaskListLimit},
		"absurdly big": {10_000, param.MaxTaskListLimit},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := (param.ListTasksRequest{Limit: tc.limit}).WithClampedLimit().Limit; got != tc.want {
				t.Errorf("WithClampedLimit() limit = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestWithClampedLimitKeepsOtherFields(t *testing.T) {
	req := param.ListTasksRequest{Status: entity.TaskStatusPending, Assignee: "kh", Cursor: 7}

	got := req.WithClampedLimit()

	if got.Status != req.Status || got.Assignee != req.Assignee || got.Cursor != req.Cursor {
		t.Errorf("WithClampedLimit() = %+v, want other fields unchanged from %+v", got, req)
	}
}
