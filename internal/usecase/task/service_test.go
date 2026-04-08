package task

import (
	"context"
	"errors"
	"testing"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type inMemoryRepo struct {
	tasks  map[int64]*taskdomain.Task
	nextID int64
}

func newInMemoryRepo() *inMemoryRepo {
	return &inMemoryRepo{tasks: map[int64]*taskdomain.Task{}, nextID: 1}
}

func (r *inMemoryRepo) Create(_ context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	id := r.nextID
	r.nextID++
	copy := *task
	copy.ID = id
	r.tasks[id] = &copy
	return &copy, nil
}
func (r *inMemoryRepo) GetByID(_ context.Context, id int64) (*taskdomain.Task, error) {
	t, ok := r.tasks[id]
	if !ok {
		return nil, taskdomain.ErrNotFound
	}
	copy := *t
	return &copy, nil
}
func (r *inMemoryRepo) Update(_ context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	existing, ok := r.tasks[task.ID]
	if !ok {
		return nil, taskdomain.ErrNotFound
	}
	existing.Title = task.Title
	existing.Description = task.Description
	existing.Status = task.Status
	existing.Recurrence = task.Recurrence
	existing.UpdatedAt = task.UpdatedAt
	copy := *existing
	return &copy, nil
}
func (r *inMemoryRepo) Delete(_ context.Context, id int64) error {
	if _, ok := r.tasks[id]; !ok {
		return taskdomain.ErrNotFound
	}
	delete(r.tasks, id)
	for taskID, task := range r.tasks {
		if task.TemplateTaskID != nil && *task.TemplateTaskID == id {
			delete(r.tasks, taskID)
		}
	}
	return nil
}
func (r *inMemoryRepo) List(_ context.Context) ([]taskdomain.Task, error) {
	result := make([]taskdomain.Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		result = append(result, *task)
	}
	return result, nil
}
func (r *inMemoryRepo) ListRecurringTemplates(_ context.Context) ([]taskdomain.Task, error) {
	result := make([]taskdomain.Task, 0)
	for _, task := range r.tasks {
		if task.Recurrence != nil {
			result = append(result, *task)
		}
	}
	return result, nil
}
func (r *inMemoryRepo) HasGeneratedTask(_ context.Context, templateTaskID int64, scheduledFor time.Time) (bool, error) {
	for _, task := range r.tasks {
		if task.TemplateTaskID == nil || task.ScheduledFor == nil {
			continue
		}
		if *task.TemplateTaskID == templateTaskID && task.ScheduledFor.Equal(scheduledFor) {
			return true, nil
		}
	}
	return false, nil
}

func TestCreateWithoutRecurrence(t *testing.T) {
	svc := NewService(newInMemoryRepo())
	task, err := svc.Create(context.Background(), CreateInput{Title: "task"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.Recurrence != nil {
		t.Fatalf("expected nil recurrence")
	}
}

func TestCreateWithEachRecurrenceType(t *testing.T) {
	svc := NewService(newInMemoryRepo())

	cases := []RecurrenceInput{
		{Type: taskdomain.RecurrenceTypeDaily, EveryNDays: 2, StartDate: "2026-04-01"},
		{Type: taskdomain.RecurrenceTypeMonthly, DayOfMonth: 15, StartDate: "2026-04-01"},
		{Type: taskdomain.RecurrenceTypeSpecificDates, SpecificDates: []string{"2026-04-02", "2026-04-02", "2026-04-05"}, StartDate: "2026-04-01"},
		{Type: taskdomain.RecurrenceTypeMonthParity, Parity: taskdomain.MonthDayParityEven, StartDate: "2026-04-01"},
	}

	for _, tc := range cases {
		created, err := svc.Create(context.Background(), CreateInput{Title: "task", Recurrence: &tc})
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", tc.Type, err)
		}
		if created.Recurrence == nil {
			t.Fatalf("expected recurrence for %s", tc.Type)
		}
	}
}

func TestCreateValidation(t *testing.T) {
	svc := NewService(newInMemoryRepo())
	cases := []CreateInput{
		{Title: "task", Recurrence: &RecurrenceInput{Type: "unknown", StartDate: "2026-04-01"}},
		{Title: "task", Recurrence: &RecurrenceInput{Type: taskdomain.RecurrenceTypeDaily, EveryNDays: 0, StartDate: "2026-04-01"}},
		{Title: "task", Recurrence: &RecurrenceInput{Type: taskdomain.RecurrenceTypeMonthly, DayOfMonth: 31, StartDate: "2026-04-01"}},
		{Title: "task", Recurrence: &RecurrenceInput{Type: taskdomain.RecurrenceTypeSpecificDates, StartDate: "2026-04-01"}},
		{Title: "task", Recurrence: &RecurrenceInput{Type: taskdomain.RecurrenceTypeSpecificDates, SpecificDates: []string{"bad-date"}, StartDate: "2026-04-01"}},
		{Title: "task", Recurrence: &RecurrenceInput{Type: taskdomain.RecurrenceTypeMonthParity, Parity: "", StartDate: "2026-04-01"}},
		{Title: "task", Recurrence: &RecurrenceInput{Type: taskdomain.RecurrenceTypeDaily, EveryNDays: 1, DayOfMonth: 10, StartDate: "2026-04-01"}},
	}

	for _, input := range cases {
		_, err := svc.Create(context.Background(), input)
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	}
}

func TestShouldGenerateOnDate(t *testing.T) {
	date := time.Date(2026, 4, 10, 8, 0, 0, 0, time.UTC)
	if !shouldGenerateOnDate(&taskdomain.Recurrence{Type: taskdomain.RecurrenceTypeDaily, EveryNDays: 2, StartDate: time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC)}, date) {
		t.Fatalf("expected daily recurrence to match")
	}
	if !shouldGenerateOnDate(&taskdomain.Recurrence{Type: taskdomain.RecurrenceTypeMonthly, DayOfMonth: 10, StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, date) {
		t.Fatalf("expected monthly recurrence to match")
	}
	if shouldGenerateOnDate(&taskdomain.Recurrence{Type: taskdomain.RecurrenceTypeMonthly, DayOfMonth: 30, StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("did not expect monthly recurrence to match day 31")
	}
	if !shouldGenerateOnDate(&taskdomain.Recurrence{Type: taskdomain.RecurrenceTypeSpecificDates, SpecificDates: []time.Time{time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)}, StartDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)}, date) {
		t.Fatalf("expected specific dates recurrence to match")
	}
	if !shouldGenerateOnDate(&taskdomain.Recurrence{Type: taskdomain.RecurrenceTypeMonthParity, Parity: taskdomain.MonthDayParityEven, StartDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)}, date) {
		t.Fatalf("expected parity recurrence to match")
	}
}

func TestGenerateForDateIsIdempotent(t *testing.T) {
	repo := newInMemoryRepo()
	svc := NewService(repo)

	_, err := svc.Create(context.Background(), CreateInput{
		Title:      "Daily template",
		Recurrence: &RecurrenceInput{Type: taskdomain.RecurrenceTypeDaily, EveryNDays: 1, StartDate: "2026-04-01"},
	})
	if err != nil {
		t.Fatalf("create template failed: %v", err)
	}

	first, err := svc.GenerateForDate(context.Background(), time.Date(2026, 4, 10, 5, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("first generate failed: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("expected one generated task, got %d", len(first))
	}

	second, err := svc.GenerateForDate(context.Background(), time.Date(2026, 4, 10, 18, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("second generate failed: %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("expected no duplicates, got %d", len(second))
	}
}

func TestUpdateAndDeleteRecurringTask(t *testing.T) {
	repo := newInMemoryRepo()
	svc := NewService(repo)

	created, err := svc.Create(context.Background(), CreateInput{
		Title:      "template",
		Status:     taskdomain.StatusNew,
		Recurrence: &RecurrenceInput{Type: taskdomain.RecurrenceTypeDaily, EveryNDays: 1, StartDate: "2026-04-01"},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	updated, err := svc.Update(context.Background(), created.ID, UpdateInput{
		Title:      "template-updated",
		Status:     taskdomain.StatusInProgress,
		Recurrence: &RecurrenceInput{Type: taskdomain.RecurrenceTypeMonthParity, Parity: taskdomain.MonthDayParityOdd, StartDate: "2026-04-01"},
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if updated.Recurrence == nil || updated.Recurrence.Type != taskdomain.RecurrenceTypeMonthParity {
		t.Fatalf("expected updated recurrence")
	}

	if err := svc.Delete(context.Background(), created.ID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if _, err := svc.GetByID(context.Background(), created.ID); !errors.Is(err, taskdomain.ErrNotFound) {
		t.Fatalf("expected deleted task to be missing")
	}
}
