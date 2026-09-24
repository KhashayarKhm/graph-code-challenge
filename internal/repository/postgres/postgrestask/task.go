package postgrestask

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"graph-code-challenge/internal/entity"
	"graph-code-challenge/internal/param"
	"graph-code-challenge/internal/repository/postgres"
)

type DB struct {
	conn *postgres.DB
}

func New(conn *postgres.DB) *DB {
	return &DB{conn: conn}
}

func (d *DB) Create(ctx context.Context, task entity.Task) (entity.Task, error) {
	const query = `INSERT INTO tasks (title, description, status, assignee)
		VALUES ($1, $2, $3, $4)
		RETURNING id`

	row := d.conn.Pool().QueryRow(ctx, query, task.Title, task.Description, task.Status, task.Assignee)

	if err := row.Scan(&task.ID); err != nil {
		return entity.Task{}, fmt.Errorf("postgrestask: create: %w", err)
	}

	return task, nil
}

func (d *DB) GetByID(ctx context.Context, id int64) (entity.Task, error) {
	const query = `SELECT id, title, description, status, assignee FROM tasks WHERE id = $1 AND deleted_at IS NULL`

	task, err := scanTask(d.conn.Pool().QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Task{}, entity.ErrTaskNotFound
		}

		return entity.Task{}, fmt.Errorf("postgrestask: get by id: %w", err)
	}

	return task, nil
}

func (d *DB) List(ctx context.Context, req param.ListTasksRequest) ([]entity.Task, error) {
	var query strings.Builder

	query.WriteString(`SELECT id, title, description, status, assignee FROM tasks WHERE deleted_at IS NULL`)

	args := make([]any, 0, 4)

	if req.Status != "" {
		args = append(args, req.Status)
		fmt.Fprintf(&query, " AND status = $%d", len(args))
	}

	if req.Assignee != "" {
		args = append(args, req.Assignee)
		fmt.Fprintf(&query, " AND assignee = $%d", len(args))
	}

	if req.Cursor > 0 {
		args = append(args, req.Cursor)
		fmt.Fprintf(&query, " AND id < $%d", len(args))
	}

	args = append(args, req.Limit)
	fmt.Fprintf(&query, " ORDER BY id DESC LIMIT $%d", len(args))

	rows, err := d.conn.Pool().Query(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("postgrestask: list: %w", err)
	}
	defer rows.Close()

	tasks := make([]entity.Task, 0, req.Limit)

	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("postgrestask: list scan: %w", err)
		}

		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgrestask: list rows: %w", err)
	}

	return tasks, nil
}

func (d *DB) Update(ctx context.Context, req param.UpdateTaskRequest) (entity.Task, error) {
	var query strings.Builder

	query.WriteString(`UPDATE tasks SET updated_at = now()`)

	args := make([]any, 0, 5)
	args = append(args, req.ID)

	if req.Title != nil {
		args = append(args, *req.Title)
		fmt.Fprintf(&query, ", title = $%d", len(args))
	}

	if req.Description != nil {
		args = append(args, *req.Description)
		fmt.Fprintf(&query, ", description = $%d", len(args))
	}

	if req.Status != nil {
		args = append(args, *req.Status)
		fmt.Fprintf(&query, ", status = $%d", len(args))
	}

	if req.Assignee != nil {
		args = append(args, *req.Assignee)
		fmt.Fprintf(&query, ", assignee = $%d", len(args))
	}

	query.WriteString(` WHERE id = $1 AND deleted_at IS NULL RETURNING id, title, description, status, assignee`)

	updated, err := scanTask(d.conn.Pool().QueryRow(ctx, query.String(), args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Task{}, entity.ErrTaskNotFound
		}

		return entity.Task{}, fmt.Errorf("postgrestask: update: %w", err)
	}

	return updated, nil
}

func (d *DB) Delete(ctx context.Context, id int64) error {
	const query = `UPDATE tasks SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`

	tag, err := d.conn.Pool().Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("postgrestask: delete: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return entity.ErrTaskNotFound
	}

	return nil
}

func (d *DB) Count(ctx context.Context) (int64, error) {
	const query = `SELECT count(id) FROM tasks WHERE deleted_at IS NULL`

	var count int64

	if err := d.conn.Pool().QueryRow(ctx, query).Scan(&count); err != nil {
		return 0, fmt.Errorf("postgrestask: count: %w", err)
	}

	return count, nil
}

func scanTask(row pgx.Row) (entity.Task, error) {
	var task entity.Task

	err := row.Scan(&task.ID, &task.Title, &task.Description, &task.Status, &task.Assignee)

	return task, err
}
