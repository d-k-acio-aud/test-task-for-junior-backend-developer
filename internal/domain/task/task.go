package task

import "time"

type Status string

type RecurrenceType string

type MonthDayParity string

const (
	StatusNew        Status = "new"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

const (
	RecurrenceTypeDaily         RecurrenceType = "daily"
	RecurrenceTypeMonthly       RecurrenceType = "monthly"
	RecurrenceTypeSpecificDates RecurrenceType = "specific_dates"
	RecurrenceTypeMonthParity   RecurrenceType = "month_day_parity"
)

const (
	MonthDayParityEven MonthDayParity = "even"
	MonthDayParityOdd  MonthDayParity = "odd"
)

type Recurrence struct {
	Type          RecurrenceType `json:"type"`
	EveryNDays    int            `json:"every_n_days,omitempty"`
	DayOfMonth    int            `json:"day_of_month,omitempty"`
	SpecificDates []time.Time    `json:"specific_dates,omitempty"`
	Parity        MonthDayParity `json:"parity,omitempty"`
	StartDate     time.Time      `json:"start_date"`
}

type Task struct {
	ID             int64       `json:"id"`
	Title          string      `json:"title"`
	Description    string      `json:"description"`
	Status         Status      `json:"status"`
	Recurrence     *Recurrence `json:"recurrence,omitempty"`
	ScheduledFor   *time.Time  `json:"scheduled_for,omitempty"`
	TemplateTaskID *int64      `json:"template_task_id,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

func (s Status) Valid() bool {
	switch s {
	case StatusNew, StatusInProgress, StatusDone:
		return true
	default:
		return false
	}
}

func (t RecurrenceType) Valid() bool {
	switch t {
	case RecurrenceTypeDaily, RecurrenceTypeMonthly, RecurrenceTypeSpecificDates, RecurrenceTypeMonthParity:
		return true
	default:
		return false
	}
}

func (p MonthDayParity) Valid() bool {
	switch p {
	case MonthDayParityEven, MonthDayParityOdd:
		return true
	default:
		return false
	}
}
