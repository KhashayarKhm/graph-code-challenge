package entity

import "fmt"

const (
	MaxTaskTitleLen       = 200
	MaxTaskDescriptionLen = 2000
	MaxTaskAssigneeLen    = 100
)

var ErrTaskNotFound = fmt.Errorf("task: %w", ErrNotFound)

type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusDone       TaskStatus = "done"
)

func TaskStatusList() []TaskStatus {
	return []TaskStatus{TaskStatusPending, TaskStatusInProgress, TaskStatusDone}
}

func (s TaskStatus) IsValid() bool {
	switch s {
	case TaskStatusPending, TaskStatusInProgress, TaskStatusDone:
		return true
	}

	return false
}

func (s TaskStatus) String() string {
	return string(s)
}

type Task struct {
	ID          int64
	Title       string
	Description string
	Status      TaskStatus
	Assignee    string
}
