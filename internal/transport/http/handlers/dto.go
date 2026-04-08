package handlers

import (
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type recurrenceDTO struct {
	Type          taskdomain.RecurrenceType `json:"type"`
	EveryNDays    int                       `json:"every_n_days,omitempty"`
	DayOfMonth    int                       `json:"day_of_month,omitempty"`
	SpecificDates []string                  `json:"specific_dates,omitempty"`
	Parity        taskdomain.MonthDayParity `json:"parity,omitempty"`
	StartDate     string                    `json:"start_date"`
}

type taskMutationDTO struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Status      taskdomain.Status `json:"status"`
	Recurrence  *recurrenceDTO    `json:"recurrence,omitempty"`
}

type taskDTO struct {
	ID             int64             `json:"id"`
	Title          string            `json:"title"`
	Description    string            `json:"description"`
	Status         taskdomain.Status `json:"status"`
	Recurrence     *recurrenceDTO    `json:"recurrence,omitempty"`
	ScheduledFor   *time.Time        `json:"scheduled_for,omitempty"`
	TemplateTaskID *int64            `json:"template_task_id,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

func newTaskDTO(task *taskdomain.Task) taskDTO {
	return taskDTO{
		ID:             task.ID,
		Title:          task.Title,
		Description:    task.Description,
		Status:         task.Status,
		Recurrence:     recurrenceToDTO(task.Recurrence),
		ScheduledFor:   task.ScheduledFor,
		TemplateTaskID: task.TemplateTaskID,
		CreatedAt:      task.CreatedAt,
		UpdatedAt:      task.UpdatedAt,
	}
}

func recurrenceToDTO(recurrence *taskdomain.Recurrence) *recurrenceDTO {
	if recurrence == nil {
		return nil
	}

	dates := make([]string, 0, len(recurrence.SpecificDates))
	for _, d := range recurrence.SpecificDates {
		dates = append(dates, d.Format("2006-01-02"))
	}

	return &recurrenceDTO{
		Type:          recurrence.Type,
		EveryNDays:    recurrence.EveryNDays,
		DayOfMonth:    recurrence.DayOfMonth,
		SpecificDates: dates,
		Parity:        recurrence.Parity,
		StartDate:     recurrence.StartDate.Format("2006-01-02"),
	}
}
