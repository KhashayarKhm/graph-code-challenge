package param

import "graph-code-challenge/internal/entity"

type UpdateTaskRequest struct {
	ID          int64             `json:"-"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Status      entity.TaskStatus `json:"status"`
	Assignee    string            `json:"assignee"`
}

type UpdateTaskResponse struct {
	Task TaskInfo `json:"task"`
}
