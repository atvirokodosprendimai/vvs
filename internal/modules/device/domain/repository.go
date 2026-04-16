package domain

import "context"

// DeviceRepository is the persistence port for the device aggregate.
type DeviceRepository interface {
	Save(ctx context.Context, d *Device) error
	FindByID(ctx context.Context, id string) (*Device, error)
	Delete(ctx context.Context, id string) error
}
