package entity

import (
	"fmt"
	"time"
)

const (
	MaxTaskTitleLen       = 200
	MaxTaskDescriptionLen = 2000
	MaxTaskAssigneeLen    = 100
)

var (
	ErrTaskNotFound = fmt.Errorf("task: %w", ErrNotFound)

	ErrTaskTitleRequired   = fmt.Errorf("task: title is required: %w", ErrValidation)
	ErrTaskTitleTooLong    = fmt.Errorf("task: title exceeds %d characters: %w", MaxTaskTitleLen, ErrValidation)
	ErrTaskDescTooLong     = fmt.Errorf("task: description exceeds %d characters: %w", MaxTaskDescriptionLen, ErrValidation)
	ErrTaskAssigneeTooLong = fmt.Errorf("task: assignee exceeds %d characters: %w", MaxTaskAssigneeLen, ErrValidation)
	ErrTaskInvalidStatus   = fmt.Errorf("task: status must be one of pending, in_progress, done: %w", ErrValidation)
)

type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusDone       TaskStatus = "done"
)

var TaskStatuses = []TaskStatus{
	TaskStatusPending,
	TaskStatusInProgress,
	TaskStatusDone,
}

func (s TaskStatus) Valid() bool {
	for _, known := range TaskStatuses {
		if s == known {
			return true
		}
	}

	return false
}

type Task struct {
	ID          int64
	Title       string
	Description string
	Status      TaskStatus
	Assignee    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

func (t Task) IsDeleted() bool {
	return t.DeletedAt != nil
}

func (t Task) Validate() error {
	switch {
	case t.Title == "":
		return ErrTaskTitleRequired
	case len(t.Title) > MaxTaskTitleLen:
		return ErrTaskTitleTooLong
	case len(t.Description) > MaxTaskDescriptionLen:
		return ErrTaskDescTooLong
	case len(t.Assignee) > MaxTaskAssigneeLen:
		return ErrTaskAssigneeTooLong
	case !t.Status.Valid():
		return ErrTaskInvalidStatus
	}

	return nil
}
