package param

import "graph-code-challenge/internal/entity"

type CreateTaskRequest struct {
	Title       string            `json:"title" validate:"required" minLength:"1" maxLength:"200" example:"Document the API"`
	Description string            `json:"description" maxLength:"2000" example:"Generate and verify the Swagger specification"`
	Status      entity.TaskStatus `json:"status" enums:"pending,in_progress,done" example:"pending"`
	Assignee    string            `json:"assignee" maxLength:"100" example:"alex"`
}

type CreateTaskResponse struct {
	Task TaskInfo `json:"task"`
}
