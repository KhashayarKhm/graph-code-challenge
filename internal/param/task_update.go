package param

import "graph-code-challenge/internal/entity"

type UpdateTaskRequest struct {
	ID          int64              `json:"-"`
	Title       *string            `json:"title" minLength:"1" maxLength:"200" example:"Publish the API docs"`
	Description *string            `json:"description" maxLength:"2000" example:"Review and publish the generated specification"`
	Status      *entity.TaskStatus `json:"status" enums:"pending,in_progress,done" example:"done"`
	Assignee    *string            `json:"assignee" maxLength:"100" example:"sam"`
}

func (r UpdateTaskRequest) IsEmpty() bool {
	return r.Title == nil && r.Description == nil && r.Status == nil && r.Assignee == nil
}

type UpdateTaskResponse struct {
	Task TaskInfo `json:"task"`
}
