package task

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

const dateLayout = "2006-01-02"

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*taskdomain.Task, error) {
	normalized, recurrence, err := validateCreateInput(input)
	if err != nil {
		return nil, err
	}

	now := s.now()
	model := &taskdomain.Task{
		Title:       normalized.Title,
		Description: normalized.Description,
		Status:      normalized.Status,
		Recurrence:  recurrence,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	created, err := s.repo.Create(ctx, model)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	return s.repo.GetByID(ctx, id)
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (*taskdomain.Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	normalized, recurrence, err := validateUpdateInput(input)
	if err != nil {
		return nil, err
	}

	model := &taskdomain.Task{
		ID:          id,
		Title:       normalized.Title,
		Description: normalized.Description,
		Status:      normalized.Status,
		Recurrence:  recurrence,
		UpdatedAt:   s.now(),
	}

	updated, err := s.repo.Update(ctx, model)
	if err != nil {
		return nil, err
	}

	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	return s.repo.Delete(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]taskdomain.Task, error) {
	return s.repo.List(ctx)
}

func (s *Service) GenerateForDate(ctx context.Context, date time.Time) ([]taskdomain.Task, error) {
	templates, err := s.repo.ListRecurringTemplates(ctx)
	if err != nil {
		return nil, err
	}

	day := normalizeDate(date)
	now := s.now()
	generated := make([]taskdomain.Task, 0)
	for i := range templates {
		tpl := templates[i]
		if tpl.Recurrence == nil || !shouldGenerateOnDate(tpl.Recurrence, day) {
			continue
		}

		exists, err := s.repo.HasGeneratedTask(ctx, tpl.ID, day)
		if err != nil {
			return nil, err
		}
		if exists {
			continue
		}

		templateID := tpl.ID
		newTask := &taskdomain.Task{
			Title:          tpl.Title,
			Description:    tpl.Description,
			Status:         taskdomain.StatusNew,
			ScheduledFor:   &day,
			TemplateTaskID: &templateID,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		created, err := s.repo.Create(ctx, newTask)
		if err != nil {
			return nil, err
		}

		generated = append(generated, *created)
	}

	return generated, nil
}

func validateCreateInput(input CreateInput) (CreateInput, *taskdomain.Recurrence, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if input.Title == "" {
		return CreateInput{}, nil, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	if input.Status == "" {
		input.Status = taskdomain.StatusNew
	}

	if !input.Status.Valid() {
		return CreateInput{}, nil, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}

	recurrence, err := normalizeRecurrenceInput(input.Recurrence)
	if err != nil {
		return CreateInput{}, nil, err
	}

	return input, recurrence, nil
}

func validateUpdateInput(input UpdateInput) (UpdateInput, *taskdomain.Recurrence, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if input.Title == "" {
		return UpdateInput{}, nil, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	if !input.Status.Valid() {
		return UpdateInput{}, nil, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}

	recurrence, err := normalizeRecurrenceInput(input.Recurrence)
	if err != nil {
		return UpdateInput{}, nil, err
	}

	return input, recurrence, nil
}

func normalizeRecurrenceInput(input *RecurrenceInput) (*taskdomain.Recurrence, error) {
	if input == nil {
		return nil, nil
	}

	if !input.Type.Valid() {
		return nil, fmt.Errorf("%w: invalid recurrence type", ErrInvalidInput)
	}

	startDate, err := parseDate(input.StartDate)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid recurrence start_date", ErrInvalidInput)
	}

	recurrence := &taskdomain.Recurrence{Type: input.Type, StartDate: startDate}

	switch input.Type {
	case taskdomain.RecurrenceTypeDaily:
		if input.EveryNDays <= 0 {
			return nil, fmt.Errorf("%w: every_n_days must be positive", ErrInvalidInput)
		}
		recurrence.EveryNDays = input.EveryNDays
	case taskdomain.RecurrenceTypeMonthly:
		if input.DayOfMonth < 1 || input.DayOfMonth > 30 {
			return nil, fmt.Errorf("%w: day_of_month must be in range 1..30", ErrInvalidInput)
		}
		recurrence.DayOfMonth = input.DayOfMonth
	case taskdomain.RecurrenceTypeSpecificDates:
		if len(input.SpecificDates) == 0 {
			return nil, fmt.Errorf("%w: specific_dates must not be empty", ErrInvalidInput)
		}
		dates := make([]time.Time, 0, len(input.SpecificDates))
		seen := make(map[string]struct{}, len(input.SpecificDates))
		for _, raw := range input.SpecificDates {
			d, err := parseDate(raw)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid specific_dates value", ErrInvalidInput)
			}
			key := d.Format(dateLayout)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			dates = append(dates, d)
		}
		sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
		recurrence.SpecificDates = dates
	case taskdomain.RecurrenceTypeMonthParity:
		if !input.Parity.Valid() {
			return nil, fmt.Errorf("%w: invalid parity", ErrInvalidInput)
		}
		recurrence.Parity = input.Parity
	}

	if hasConflictingOptions(input) {
		return nil, fmt.Errorf("%w: conflicting recurrence settings", ErrInvalidInput)
	}

	return recurrence, nil
}

func hasConflictingOptions(input *RecurrenceInput) bool {
	switch input.Type {
	case taskdomain.RecurrenceTypeDaily:
		return input.DayOfMonth != 0 || len(input.SpecificDates) > 0 || input.Parity != ""
	case taskdomain.RecurrenceTypeMonthly:
		return input.EveryNDays != 0 || len(input.SpecificDates) > 0 || input.Parity != ""
	case taskdomain.RecurrenceTypeSpecificDates:
		return input.EveryNDays != 0 || input.DayOfMonth != 0 || input.Parity != ""
	case taskdomain.RecurrenceTypeMonthParity:
		return input.EveryNDays != 0 || input.DayOfMonth != 0 || len(input.SpecificDates) > 0
	default:
		return true
	}
}

func shouldGenerateOnDate(recurrence *taskdomain.Recurrence, date time.Time) bool {
	if recurrence == nil {
		return false
	}

	date = normalizeDate(date)
	startDate := normalizeDate(recurrence.StartDate)
	if date.Before(startDate) {
		return false
	}

	switch recurrence.Type {
	case taskdomain.RecurrenceTypeDaily:
		days := int(date.Sub(startDate).Hours() / 24)
		return days%recurrence.EveryNDays == 0
	case taskdomain.RecurrenceTypeMonthly:
		if date.Day() < 1 || date.Day() > 30 {
			return false
		}
		return date.Day() == recurrence.DayOfMonth
	case taskdomain.RecurrenceTypeSpecificDates:
		for _, d := range recurrence.SpecificDates {
			if normalizeDate(d).Equal(date) {
				return true
			}
		}
		return false
	case taskdomain.RecurrenceTypeMonthParity:
		if recurrence.Parity == taskdomain.MonthDayParityEven {
			return date.Day()%2 == 0
		}
		return date.Day()%2 != 0
	default:
		return false
	}
}

func parseDate(raw string) (time.Time, error) {
	parsed, err := time.Parse(dateLayout, strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, err
	}

	return parsed.UTC(), nil
}

func normalizeDate(d time.Time) time.Time {
	y, m, day := d.UTC().Date()
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}
