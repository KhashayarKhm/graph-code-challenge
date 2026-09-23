package param

import "graph-code-challenge/internal/entity"

const (
	DefaultTaskListLimit = 20
	MaxTaskListLimit     = 50
)

type ListTasksRequest struct {
	Status   entity.TaskStatus `form:"status"`
	Assignee string            `form:"assignee"`
	Cursor   int64             `form:"cursor"`
	Limit    int               `form:"limit"`
}

func (r ListTasksRequest) WithClampedLimit() ListTasksRequest {
	switch {
	case r.Limit <= 0:
		r.Limit = DefaultTaskListLimit
	case r.Limit > MaxTaskListLimit:
		r.Limit = MaxTaskListLimit
	}

	return r
}

type ListTasksResponse struct {
	Tasks      []TaskInfo `json:"tasks"`
	NextCursor int64      `json:"next_cursor"`
	HasMore    bool       `json:"has_more"`
}
