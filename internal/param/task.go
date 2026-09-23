package param

import "graph-code-challenge/internal/entity"

type TaskInfo struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Assignee    string `json:"assignee"`
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
