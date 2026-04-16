package queries

import "time"

type ContactReadModel struct {
	ID         string    `gorm:"column:id"`
	CustomerID string    `gorm:"column:customer_id"`
	FirstName  string    `gorm:"column:first_name"`
	LastName   string    `gorm:"column:last_name"`
	Email      string    `gorm:"column:email"`
	Phone      string    `gorm:"column:phone"`
	Role       string    `gorm:"column:role"`
	Notes      string    `gorm:"column:notes"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (m ContactReadModel) FullName() string {
	if m.LastName == "" {
		return m.FirstName
	}
	return m.FirstName + " " + m.LastName
}
