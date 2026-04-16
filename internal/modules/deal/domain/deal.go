package domain

import (
	"errors"
	"time"
)

// Stage constants for a Deal.
const (
	StageNew         = "new"
	StageQualified   = "qualified"
	StageProposal    = "proposal"
	StageNegotiation = "negotiation"
	StageWon         = "won"
	StageLost        = "lost"
)

var ErrNotFound      = errors.New("deal not found")
var ErrTitleRequired = errors.New("title is required")
var ErrAlreadyClosed = errors.New("deal is already closed (won or lost)")

// Deal represents a sales opportunity for a customer.
type Deal struct {
	ID         string
	CustomerID string
	Title      string
	Value      int64  // cents
	Currency   string
	Stage      string
	Notes      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func NewDeal(id, customerID, title string, value int64, currency, notes string) (*Deal, error) {
	if title == "" {
		return nil, ErrTitleRequired
	}
	if currency == "" {
		currency = "EUR"
	}
	now := time.Now().UTC()
	return &Deal{
		ID:         id,
		CustomerID: customerID,
		Title:      title,
		Value:      value,
		Currency:   currency,
		Stage:      StageNew,
		Notes:      notes,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

func (d *Deal) isTerminal() bool {
	return d.Stage == StageWon || d.Stage == StageLost
}

// Qualify advances a deal from new → qualified.
func (d *Deal) Qualify() error {
	if d.isTerminal() {
		return ErrAlreadyClosed
	}
	if d.Stage != StageNew {
		return errors.New("deal must be in 'new' stage to qualify")
	}
	d.Stage = StageQualified
	d.UpdatedAt = time.Now().UTC()
	return nil
}

// Propose advances a deal from qualified → proposal.
func (d *Deal) Propose() error {
	if d.isTerminal() {
		return ErrAlreadyClosed
	}
	if d.Stage != StageQualified {
		return errors.New("deal must be in 'qualified' stage to propose")
	}
	d.Stage = StageProposal
	d.UpdatedAt = time.Now().UTC()
	return nil
}

// Negotiate advances a deal from proposal → negotiation.
func (d *Deal) Negotiate() error {
	if d.isTerminal() {
		return ErrAlreadyClosed
	}
	if d.Stage != StageProposal {
		return errors.New("deal must be in 'proposal' stage to negotiate")
	}
	d.Stage = StageNegotiation
	d.UpdatedAt = time.Now().UTC()
	return nil
}

// Win closes a deal as won (from any non-terminal stage).
func (d *Deal) Win() error {
	if d.isTerminal() {
		return ErrAlreadyClosed
	}
	d.Stage = StageWon
	d.UpdatedAt = time.Now().UTC()
	return nil
}

// Lose closes a deal as lost (from any non-terminal stage).
func (d *Deal) Lose() error {
	if d.isTerminal() {
		return ErrAlreadyClosed
	}
	d.Stage = StageLost
	d.UpdatedAt = time.Now().UTC()
	return nil
}

// Update modifies the mutable fields of a deal.
func (d *Deal) Update(title string, value int64, currency, notes string) error {
	if title == "" {
		return ErrTitleRequired
	}
	if currency == "" {
		currency = "EUR"
	}
	d.Title = title
	d.Value = value
	d.Currency = currency
	d.Notes = notes
	d.UpdatedAt = time.Now().UTC()
	return nil
}
