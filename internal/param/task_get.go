package param

type GetTaskRequest struct {
	ID int64 `json:"-"`
}

type GetTaskResponse struct {
	Task TaskInfo `json:"task"`
}
