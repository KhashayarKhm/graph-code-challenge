package param

import "graph-code-challenge/internal/entity"

type CreateTaskRequest struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Status      entity.TaskStatus `json:"status"`
	Assignee    string            `json:"assignee"`
}

type CreateTaskResponse struct {
	Task TaskInfo `json:"task"`
}
