package entity_test

import (
	"errors"
	"testing"

	"graph-code-challenge/internal/entity"
)

func TestTaskStatusIsValid(t *testing.T) {
	tests := map[string]struct {
		status entity.TaskStatus
		want   bool
	}{
		"pending":     {entity.TaskStatusPending, true},
		"in_progress": {entity.TaskStatusInProgress, true},
		"done":        {entity.TaskStatusDone, true},
		"empty":       {entity.TaskStatus(""), false},
		"unknown":     {entity.TaskStatus("archived"), false},
		"wrong case":  {entity.TaskStatus("Pending"), false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := tc.status.IsValid(); got != tc.want {
				t.Errorf("IsValid() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTaskStatusListIsComplete(t *testing.T) {
	statuses := entity.TaskStatusList()

	if len(statuses) != 3 {
		t.Fatalf("TaskStatusList() returned %d statuses, want 3", len(statuses))
	}

	for _, status := range statuses {
		if !status.IsValid() {
			t.Errorf("TaskStatusList() contains %q, which IsValid() rejects", status)
		}
	}
}

func TestErrTaskNotFoundWrapsErrNotFound(t *testing.T) {
	if !errors.Is(entity.ErrTaskNotFound, entity.ErrNotFound) {
		t.Error("ErrTaskNotFound does not wrap ErrNotFound")
	}

	if errors.Is(entity.ErrTaskNotFound, entity.ErrValidation) {
		t.Error("ErrTaskNotFound must not wrap ErrValidation")
	}
}
