package importers

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/payment/domain"
	shareddomain "github.com/vvs/isp/internal/shared/domain"
)

type SepaCSVImporter struct{}

func NewSepaCSVImporter() *SepaCSVImporter {
	return &SepaCSVImporter{}
}

func (s *SepaCSVImporter) Format() string {
	return "sepa_csv"
}

func (s *SepaCSVImporter) Parse(_ context.Context, reader io.Reader) ([]*domain.Payment, error) {
	csvReader := csv.NewReader(reader)
	csvReader.Comma = ';'
	csvReader.LazyQuotes = true

	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, nil // Empty file or header only
	}

	batchID := uuid.Must(uuid.NewV7()).String()
	var payments []*domain.Payment

	// Skip header row
	for i, record := range records[1:] {
		if len(record) < 5 {
			return nil, fmt.Errorf("row %d: expected 5 columns, got %d", i+2, len(record))
		}

		// Date
		bookingDate, err := time.Parse("2006-01-02", strings.TrimSpace(record[0]))
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid date %q: %w", i+2, record[0], err)
		}

		// Amount (decimal e.g. "150.00" -> 15000 cents)
		amountStr := strings.TrimSpace(record[1])
		amountFloat, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid amount %q: %w", i+2, record[1], err)
		}
		amountCents := int64(math.Round(amountFloat * 100))

		reference := strings.TrimSpace(record[2])
		payerName := strings.TrimSpace(record[3])
		payerIBAN := strings.TrimSpace(record[4])

		payment := domain.NewPayment(
			shareddomain.EUR(amountCents),
			reference,
			payerName,
			payerIBAN,
			bookingDate,
			batchID,
		)

		payments = append(payments, payment)
	}

	return payments, nil
}
