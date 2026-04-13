package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/shared/domain"
)

var (
	ErrCustomerIDRequired       = errors.New("customer ID is required")
	ErrCustomerNameRequired     = errors.New("customer name is required")
	ErrInvalidFrequency         = errors.New("frequency must be monthly, quarterly, or yearly")
	ErrInvalidDayOfMonth        = errors.New("day of month must be between 1 and 28")
	ErrRecurringNotFound        = errors.New("recurring invoice not found")
	ErrAlreadyPaused            = errors.New("recurring invoice is already paused")
	ErrAlreadyActive            = errors.New("recurring invoice is already active")
	ErrAlreadyCancelled         = errors.New("recurring invoice is cancelled")
	ErrNoLines                  = errors.New("recurring invoice must have at least one line")
	ErrInvalidQuantity          = errors.New("quantity must be greater than zero")
	ErrProductNameRequired      = errors.New("product name is required")
)

type RecurringStatus string

const (
	StatusActive    RecurringStatus = "active"
	StatusPaused    RecurringStatus = "paused"
	StatusCancelled RecurringStatus = "cancelled"
)

type Frequency string

const (
	FrequencyMonthly   Frequency = "monthly"
	FrequencyQuarterly Frequency = "quarterly"
	FrequencyYearly    Frequency = "yearly"
)

type Schedule struct {
	Frequency  Frequency
	DayOfMonth int
}

func NewSchedule(frequency string, dayOfMonth int) (Schedule, error) {
	freq := Frequency(strings.ToLower(strings.TrimSpace(frequency)))
	switch freq {
	case FrequencyMonthly, FrequencyQuarterly, FrequencyYearly:
	default:
		return Schedule{}, ErrInvalidFrequency
	}

	if dayOfMonth < 1 || dayOfMonth > 28 {
		return Schedule{}, ErrInvalidDayOfMonth
	}

	return Schedule{Frequency: freq, DayOfMonth: dayOfMonth}, nil
}

func (s Schedule) FrequencyLabel() string {
	switch s.Frequency {
	case FrequencyMonthly:
		return "Monthly"
	case FrequencyQuarterly:
		return "Quarterly"
	case FrequencyYearly:
		return "Yearly"
	default:
		return string(s.Frequency)
	}
}

type RecurringLine struct {
	ID          string
	ProductID   string
	ProductName string
	Description string
	Quantity    int
	UnitPrice   domain.Money
	SortOrder   int
}

type RecurringInvoice struct {
	ID           string
	CustomerID   string
	CustomerName string
	Lines        []RecurringLine
	Schedule     Schedule
	NextRunDate  time.Time
	LastRunDate  *time.Time
	Status       RecurringStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NewRecurringInvoice(customerID, customerName string, frequency string, dayOfMonth int) (*RecurringInvoice, error) {
	customerID = strings.TrimSpace(customerID)
	if customerID == "" {
		return nil, ErrCustomerIDRequired
	}

	customerName = strings.TrimSpace(customerName)
	if customerName == "" {
		return nil, ErrCustomerNameRequired
	}

	schedule, err := NewSchedule(frequency, dayOfMonth)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	nextRun := calculateNextRunDate(now, schedule)

	return &RecurringInvoice{
		ID:           uuid.Must(uuid.NewV7()).String(),
		CustomerID:   customerID,
		CustomerName: customerName,
		Schedule:     schedule,
		NextRunDate:  nextRun,
		Status:       StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (r *RecurringInvoice) AddLine(productID, productName, description string, quantity int, unitPrice domain.Money) error {
	productName = strings.TrimSpace(productName)
	if productName == "" {
		return ErrProductNameRequired
	}
	if quantity < 1 {
		return ErrInvalidQuantity
	}

	line := RecurringLine{
		ID:          uuid.Must(uuid.NewV7()).String(),
		ProductID:   strings.TrimSpace(productID),
		ProductName: productName,
		Description: strings.TrimSpace(description),
		Quantity:    quantity,
		UnitPrice:   unitPrice,
		SortOrder:   len(r.Lines),
	}

	r.Lines = append(r.Lines, line)
	r.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *RecurringInvoice) Pause() error {
	if r.Status == StatusCancelled {
		return ErrAlreadyCancelled
	}
	if r.Status == StatusPaused {
		return ErrAlreadyPaused
	}
	r.Status = StatusPaused
	r.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *RecurringInvoice) Resume() error {
	if r.Status == StatusCancelled {
		return ErrAlreadyCancelled
	}
	if r.Status == StatusActive {
		return ErrAlreadyActive
	}
	r.Status = StatusActive
	r.NextRunDate = calculateNextRunDate(time.Now().UTC(), r.Schedule)
	r.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *RecurringInvoice) Cancel() error {
	if r.Status == StatusCancelled {
		return ErrAlreadyCancelled
	}
	r.Status = StatusCancelled
	r.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *RecurringInvoice) IsDue(asOf time.Time) bool {
	if r.Status != StatusActive {
		return false
	}
	return !asOf.Before(r.NextRunDate)
}

func (r *RecurringInvoice) Total() domain.Money {
	total := domain.EUR(0)
	for _, line := range r.Lines {
		lineTotal := line.UnitPrice.Multiply(int64(line.Quantity))
		total, _ = total.Add(lineTotal)
	}
	return total
}

func calculateNextRunDate(from time.Time, schedule Schedule) time.Time {
	year, month, _ := from.Date()
	candidate := time.Date(year, month, schedule.DayOfMonth, 0, 0, 0, 0, time.UTC)

	if !candidate.After(from) {
		switch schedule.Frequency {
		case FrequencyMonthly:
			candidate = candidate.AddDate(0, 1, 0)
		case FrequencyQuarterly:
			candidate = candidate.AddDate(0, 3, 0)
		case FrequencyYearly:
			candidate = candidate.AddDate(1, 0, 0)
		}
	}

	return candidate
}
