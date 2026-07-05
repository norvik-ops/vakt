package vaktcomply

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/matharnica/vakt/internal/db"
)

func (r *Repository) GetMyTaskControls(ctx context.Context, orgID, ownerDisplayName string) ([]MyTask, error) {
	rows, err := r.q.ListCKMyTaskControls(ctx, db.ListCKMyTaskControlsParams{
		OrgID: orgID,
		Owner: ownerDisplayName,
	})
	if err != nil {
		return nil, fmt.Errorf("list my task controls: %w", err)
	}
	tasks := make([]MyTask, 0, len(rows))
	for _, row := range rows {
		tasks = append(tasks, MyTask{
			ID:          row.ID,
			Title:       row.Title,
			Type:        "control",
			Status:      row.ManualStatus,
			FrameworkID: row.FrameworkID,
		})
	}
	return tasks, nil
}

// GetMyTaskRisks returns risks owned by a user in an org (by display name).
func (r *Repository) GetMyTaskRisks(ctx context.Context, orgID, ownerDisplayName string) ([]MyTask, error) {
	rows, err := r.q.ListCKMyTaskRisks(ctx, db.ListCKMyTaskRisksParams{
		OrgID: orgID,
		Owner: ownerDisplayName,
	})
	if err != nil {
		return nil, fmt.Errorf("list my task risks: %w", err)
	}
	tasks := make([]MyTask, 0, len(rows))
	for _, row := range rows {
		tasks = append(tasks, MyTask{
			ID:     row.ID,
			Title:  row.Title,
			Type:   "risk",
			Status: row.Status,
		})
	}
	return tasks, nil
}

func taskFromCk(r db.CkTasks) Task {
	return Task{
		ID:            r.ID,
		OrgID:         r.OrgID,
		EntityType:    r.EntityType,
		EntityID:      r.EntityID,
		Title:         r.Title,
		Description:   r.Description,
		AssigneeEmail: r.AssigneeEmail,
		DueDate:       ckDateToTimePtr(r.DueDate),
		Status:        r.Status,
		Priority:      r.Priority,
		CreatedBy:     r.CreatedBy,
		CreatedAt:     ckTsToTime(r.CreatedAt),
		UpdatedAt:     ckTsToTime(r.UpdatedAt),
	}
}

// ListTasks returns all tasks for the given entity, ordered newest first.
func (r *Repository) ListTasks(ctx context.Context, orgID, entityType, entityID string) ([]Task, error) {
	rows, err := r.q.ListCKTasks(ctx, db.ListCKTasksParams{
		OrgID:      orgID,
		EntityType: entityType,
		EntityID:   entityID,
	})
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	out := make([]Task, 0, len(rows))
	for _, row := range rows {
		out = append(out, taskFromCk(row))
	}
	return out, nil
}

// CreateTask inserts a new task and returns the created row.
func (r *Repository) CreateTask(ctx context.Context, orgID, entityType, entityID string, in CreateTaskInput) (Task, error) {
	dueDate := pgtype.Date{}
	if in.DueDate != nil && *in.DueDate != "" {
		t, err := time.Parse("2006-01-02", *in.DueDate)
		if err != nil {
			return Task{}, fmt.Errorf("invalid due_date format (expected YYYY-MM-DD): %w", err)
		}
		dueDate = pgtype.Date{Time: t, Valid: true}
	}
	status := in.Status
	if status == "" {
		status = "open"
	}
	priority := in.Priority
	if priority == "" {
		priority = "medium"
	}
	row, err := r.q.CreateCKTask(ctx, db.CreateCKTaskParams{
		OrgID:         orgID,
		EntityType:    entityType,
		EntityID:      entityID,
		Title:         in.Title,
		Description:   in.Description,
		AssigneeEmail: in.AssigneeEmail,
		DueDate:       dueDate,
		Status:        status,
		Priority:      priority,
	})
	if err != nil {
		return Task{}, fmt.Errorf("create task: %w", err)
	}
	return taskFromCk(row), nil
}

// UpdateTask applies partial updates to a task via COALESCE.
func (r *Repository) UpdateTask(ctx context.Context, orgID, taskID string, in UpdateTaskInput) (Task, error) {
	dueDate := pgtype.Date{}
	if in.DueDate != nil && *in.DueDate != "" {
		t, err := time.Parse("2006-01-02", *in.DueDate)
		if err != nil {
			return Task{}, fmt.Errorf("invalid due_date format (expected YYYY-MM-DD): %w", err)
		}
		dueDate = pgtype.Date{Time: t, Valid: true}
	}
	row, err := r.q.UpdateCKTask(ctx, db.UpdateCKTaskParams{
		ID:            taskID,
		OrgID:         orgID,
		Title:         optTextStrPtr(in.Title),
		Description:   optTextStrPtr(in.Description),
		AssigneeEmail: optTextStrPtr(in.AssigneeEmail),
		DueDate:       dueDate,
		Status:        optTextStrPtr(in.Status),
		Priority:      optTextStrPtr(in.Priority),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Task{}, fmt.Errorf("task not found")
		}
		return Task{}, fmt.Errorf("update task: %w", err)
	}
	return taskFromCk(row), nil
}

// DeleteTask removes a task.
func (r *Repository) DeleteTask(ctx context.Context, orgID, taskID string) error {
	n, err := r.q.DeleteCKTask(ctx, db.DeleteCKTaskParams{ID: taskID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("task not found")
	}
	return nil
}

// ListOverdueTasks returns tasks with due_date in the past that are not done.
func (r *Repository) ListOverdueTasks(ctx context.Context, orgID string) ([]Task, error) {
	rows, err := r.q.ListCKOverdueTasks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list overdue tasks: %w", err)
	}
	out := make([]Task, 0, len(rows))
	for _, row := range rows {
		out = append(out, taskFromCk(row))
	}
	return out, nil
}

// --- Comments ---

func commentFromCk(r db.CkComments) Comment {
	return Comment{
		ID:          r.ID,
		OrgID:       r.OrgID,
		EntityType:  r.EntityType,
		EntityID:    r.EntityID,
		AuthorEmail: r.AuthorEmail,
		Body:        r.Body,
		CreatedAt:   ckTsToTime(r.CreatedAt),
	}
}

// ListComments returns all comments for an entity ordered chronologically.
func (r *Repository) ListComments(ctx context.Context, orgID, entityType, entityID string) ([]Comment, error) {
	rows, err := r.q.ListCKComments(ctx, db.ListCKCommentsParams{
		OrgID:      orgID,
		EntityType: entityType,
		EntityID:   entityID,
	})
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	out := make([]Comment, 0, len(rows))
	for _, row := range rows {
		out = append(out, commentFromCk(row))
	}
	return out, nil
}

// CreateComment inserts a new comment and returns the created row.
func (r *Repository) CreateComment(ctx context.Context, orgID, entityType, entityID string, in CreateCommentInput) (Comment, error) {
	row, err := r.q.CreateCKComment(ctx, db.CreateCKCommentParams{
		OrgID:       orgID,
		EntityType:  entityType,
		EntityID:    entityID,
		AuthorEmail: in.AuthorEmail,
		Body:        in.Body,
	})
	if err != nil {
		return Comment{}, fmt.Errorf("create comment: %w", err)
	}
	return commentFromCk(row), nil
}

// DeleteComment removes a comment.
func (r *Repository) DeleteComment(ctx context.Context, orgID, commentID string) error {
	n, err := r.q.DeleteCKComment(ctx, db.DeleteCKCommentParams{ID: commentID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("comment not found")
	}
	return nil
}
