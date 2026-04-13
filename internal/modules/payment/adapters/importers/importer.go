package importers

import (
	"context"
	"fmt"
	"io"

	"github.com/vvs/isp/internal/modules/payment/domain"
)

type PaymentImporter interface {
	Format() string
	Parse(ctx context.Context, reader io.Reader) ([]*domain.Payment, error)
}

type Registry struct {
	importers map[string]PaymentImporter
}

func NewRegistry() *Registry {
	return &Registry{
		importers: make(map[string]PaymentImporter),
	}
}

func (r *Registry) Register(i PaymentImporter) {
	r.importers[i.Format()] = i
}

func (r *Registry) Get(format string) (PaymentImporter, error) {
	i, ok := r.importers[format]
	if !ok {
		return nil, fmt.Errorf("unknown import format: %s", format)
	}
	return i, nil
}

func (r *Registry) Available() []string {
	formats := make([]string, 0, len(r.importers))
	for f := range r.importers {
		formats = append(formats, f)
	}
	return formats
}
