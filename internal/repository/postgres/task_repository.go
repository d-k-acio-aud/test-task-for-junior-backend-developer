package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
		INSERT INTO tasks (
			title, description, status, recurrence_type, recurrence_every_n_days, recurrence_day_of_month,
			recurrence_specific_dates, recurrence_parity, recurrence_start_date, scheduled_for, template_task_id,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, title, description, status, recurrence_type, recurrence_every_n_days, recurrence_day_of_month,
			recurrence_specific_dates, recurrence_parity, recurrence_start_date, scheduled_for, template_task_id,
			created_at, updated_at
	`

	recurrenceType, everyNDays, dayOfMonth, specificDates, parity, startDate := recurrenceToDB(task.Recurrence)
	row := r.pool.QueryRow(
		ctx,
		query,
		task.Title,
		task.Description,
		task.Status,
		recurrenceType,
		everyNDays,
		dayOfMonth,
		specificDates,
		parity,
		startDate,
		task.ScheduledFor,
		task.TemplateTaskID,
		task.CreatedAt,
		task.UpdatedAt,
	)
	created, err := scanTask(row)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	const query = `
		SELECT id, title, description, status, recurrence_type, recurrence_every_n_days, recurrence_day_of_month,
			recurrence_specific_dates, recurrence_parity, recurrence_start_date, scheduled_for, template_task_id,
			created_at, updated_at
		FROM tasks
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	found, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return found, nil
}

func (r *Repository) Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
		UPDATE tasks
		SET title = $1,
			description = $2,
			status = $3,
			recurrence_type = $4,
			recurrence_every_n_days = $5,
			recurrence_day_of_month = $6,
			recurrence_specific_dates = $7,
			recurrence_parity = $8,
			recurrence_start_date = $9,
			updated_at = $10
		WHERE id = $11
		RETURNING id, title, description, status, recurrence_type, recurrence_every_n_days, recurrence_day_of_month,
			recurrence_specific_dates, recurrence_parity, recurrence_start_date, scheduled_for, template_task_id,
			created_at, updated_at
	`

	recurrenceType, everyNDays, dayOfMonth, specificDates, parity, startDate := recurrenceToDB(task.Recurrence)
	row := r.pool.QueryRow(
		ctx,
		query,
		task.Title,
		task.Description,
		task.Status,
		recurrenceType,
		everyNDays,
		dayOfMonth,
		specificDates,
		parity,
		startDate,
		task.UpdatedAt,
		task.ID,
	)
	updated, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return updated, nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM tasks WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return taskdomain.ErrNotFound
	}

	return nil
}

func (r *Repository) List(ctx context.Context) ([]taskdomain.Task, error) {
	const query = `
		SELECT id, title, description, status, recurrence_type, recurrence_every_n_days, recurrence_day_of_month,
			recurrence_specific_dates, recurrence_parity, recurrence_start_date, scheduled_for, template_task_id,
			created_at, updated_at
		FROM tasks
		ORDER BY id DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]taskdomain.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}

		tasks = append(tasks, *task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (r *Repository) ListRecurringTemplates(ctx context.Context) ([]taskdomain.Task, error) {
	const query = `
		SELECT id, title, description, status, recurrence_type, recurrence_every_n_days, recurrence_day_of_month,
			recurrence_specific_dates, recurrence_parity, recurrence_start_date, scheduled_for, template_task_id,
			created_at, updated_at
		FROM tasks
		WHERE recurrence_type IS NOT NULL
		ORDER BY id ASC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	templates := make([]taskdomain.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		templates = append(templates, *task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return templates, nil
}

func (r *Repository) HasGeneratedTask(ctx context.Context, templateTaskID int64, scheduledFor time.Time) (bool, error) {
	const query = `
		SELECT 1
		FROM tasks
		WHERE template_task_id = $1 AND scheduled_for = $2
		LIMIT 1
	`

	var one int
	err := r.pool.QueryRow(ctx, query, templateTaskID, scheduledFor).Scan(&one)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}

	return false, err
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(scanner taskScanner) (*taskdomain.Task, error) {
	var (
		task                    taskdomain.Task
		status                  string
		recurrenceType          *string
		recurrenceEveryNDays    *int
		recurrenceDayOfMonth    *int
		recurrenceSpecificDates []time.Time
		recurrenceParity        *string
		recurrenceStartDate     *time.Time
	)

	if err := scanner.Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&status,
		&recurrenceType,
		&recurrenceEveryNDays,
		&recurrenceDayOfMonth,
		&recurrenceSpecificDates,
		&recurrenceParity,
		&recurrenceStartDate,
		&task.ScheduledFor,
		&task.TemplateTaskID,
		&task.CreatedAt,
		&task.UpdatedAt,
	); err != nil {
		return nil, err
	}

	task.Status = taskdomain.Status(status)
	if recurrenceType != nil {
		recurrence := &taskdomain.Recurrence{Type: taskdomain.RecurrenceType(*recurrenceType)}
		if recurrenceEveryNDays != nil {
			recurrence.EveryNDays = *recurrenceEveryNDays
		}
		if recurrenceDayOfMonth != nil {
			recurrence.DayOfMonth = *recurrenceDayOfMonth
		}
		recurrence.SpecificDates = recurrenceSpecificDates
		if recurrenceParity != nil {
			recurrence.Parity = taskdomain.MonthDayParity(*recurrenceParity)
		}
		if recurrenceStartDate != nil {
			recurrence.StartDate = *recurrenceStartDate
		}
		task.Recurrence = recurrence
	}

	return &task, nil
}

func recurrenceToDB(recurrence *taskdomain.Recurrence) (*string, *int, *int, []time.Time, *string, *time.Time) {
	if recurrence == nil {
		return nil, nil, nil, nil, nil, nil
	}

	recurrenceType := string(recurrence.Type)
	var everyNDays *int
	if recurrence.EveryNDays > 0 {
		everyNDays = &recurrence.EveryNDays
	}
	var dayOfMonth *int
	if recurrence.DayOfMonth > 0 {
		dayOfMonth = &recurrence.DayOfMonth
	}
	var parity *string
	if recurrence.Parity != "" {
		value := string(recurrence.Parity)
		parity = &value
	}
	startDate := recurrence.StartDate

	return &recurrenceType, everyNDays, dayOfMonth, recurrence.SpecificDates, parity, &startDate
}
