package param

import "graph-code-challenge/internal/entity"

type TaskInfo struct {
	ID          int64  `json:"id" example:"42"`
	Title       string `json:"title" example:"Document the API"`
	Description string `json:"description" example:"Generate and verify the Swagger specification"`
	Status      string `json:"status" enums:"pending,in_progress,done" example:"in_progress"`
	Assignee    string `json:"assignee" example:"alex"`
}

func NewTaskInfo(task entity.Task) TaskInfo {
	return TaskInfo{
		ID:          task.ID,
		Title:       task.Title,
		Description: task.Description,
		Status:      task.Status.String(),
		Assignee:    task.Assignee,
	}
}
