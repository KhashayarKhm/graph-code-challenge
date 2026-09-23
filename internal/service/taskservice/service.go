package taskservice

import (
	"context"
	"fmt"

	"graph-code-challenge/internal/entity"
	"graph-code-challenge/internal/param"
)

type Repository interface {
	Create(ctx context.Context, task entity.Task) (entity.Task, error)
	GetByID(ctx context.Context, id int64) (entity.Task, error)
	List(ctx context.Context, req param.ListTasksRequest) ([]entity.Task, error)
	Update(ctx context.Context, task entity.Task) (entity.Task, error)
	Delete(ctx context.Context, id int64) error
	Count(ctx context.Context) (int64, error)
}

type Service struct {
	repo Repository
}

func New(repo Repository) Service {
	return Service{repo: repo}
}

func (s Service) Create(ctx context.Context, req param.CreateTaskRequest) (param.CreateTaskResponse, error) {
	task := entity.Task{
		Title:       req.Title,
		Description: req.Description,
		Status:      req.Status,
		Assignee:    req.Assignee,
	}

	if task.Status == "" {
		task.Status = entity.TaskStatusPending
	}

	created, err := s.repo.Create(ctx, task)
	if err != nil {
		return param.CreateTaskResponse{}, fmt.Errorf("taskservice: create task: %w", err)
	}

	return param.CreateTaskResponse{Task: param.NewTaskInfo(created)}, nil
}

func (s Service) GetByID(ctx context.Context, req param.GetTaskRequest) (param.GetTaskResponse, error) {
	task, err := s.repo.GetByID(ctx, req.ID)
	if err != nil {
		return param.GetTaskResponse{}, fmt.Errorf("taskservice: get task: %w", err)
	}

	return param.GetTaskResponse{Task: param.NewTaskInfo(task)}, nil
}

func (s Service) List(ctx context.Context, req param.ListTasksRequest) (param.ListTasksResponse, error) {
	req = req.WithClampedLimit()

	probe := req
	probe.Limit = req.Limit + 1

	tasks, err := s.repo.List(ctx, probe)
	if err != nil {
		return param.ListTasksResponse{}, fmt.Errorf("taskservice: list tasks: %w", err)
	}

	resp := param.ListTasksResponse{}

	if len(tasks) > req.Limit {
		tasks = tasks[:req.Limit]
		resp.HasMore = true
	}

	resp.Tasks = make([]param.TaskInfo, 0, len(tasks))
	for _, task := range tasks {
		resp.Tasks = append(resp.Tasks, param.NewTaskInfo(task))
	}

	if resp.HasMore && len(tasks) > 0 {
		resp.NextCursor = tasks[len(tasks)-1].ID
	}

	return resp, nil
}

func (s Service) Update(ctx context.Context, req param.UpdateTaskRequest) (param.UpdateTaskResponse, error) {
	task := entity.Task{
		ID:          req.ID,
		Title:       req.Title,
		Description: req.Description,
		Status:      req.Status,
		Assignee:    req.Assignee,
	}

	updated, err := s.repo.Update(ctx, task)
	if err != nil {
		return param.UpdateTaskResponse{}, fmt.Errorf("taskservice: update task: %w", err)
	}

	return param.UpdateTaskResponse{Task: param.NewTaskInfo(updated)}, nil
}

func (s Service) Delete(ctx context.Context, req param.DeleteTaskRequest) (param.DeleteTaskResponse, error) {
	if err := s.repo.Delete(ctx, req.ID); err != nil {
		return param.DeleteTaskResponse{}, fmt.Errorf("taskservice: delete task: %w", err)
	}

	return param.DeleteTaskResponse{}, nil
}

func (s Service) Count(ctx context.Context) (int64, error) {
	count, err := s.repo.Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("taskservice: count tasks: %w", err)
	}

	return count, nil
}
