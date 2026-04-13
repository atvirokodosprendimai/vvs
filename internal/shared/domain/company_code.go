package domain

import "fmt"

type CompanyCode struct {
	Prefix string
	Number int
}

func NewCompanyCode(prefix string, number int) CompanyCode {
	return CompanyCode{Prefix: prefix, Number: number}
}

func (c CompanyCode) String() string {
	return fmt.Sprintf("%s-%05d", c.Prefix, c.Number)
}

func (c CompanyCode) IsZero() bool {
	return c.Prefix == "" && c.Number == 0
}
