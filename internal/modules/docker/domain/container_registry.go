package domain

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrRegistryNameRequired = errors.New("registry name is required")
	ErrRegistryURLRequired  = errors.New("registry URL is required")
	ErrRegistryNotFound     = errors.New("container registry not found")
)

// ContainerRegistry holds credentials for a private Docker registry.
// Password is stored AES-256-GCM encrypted at rest.
type ContainerRegistry struct {
	ID        string
	Name      string
	URL       string // registry hostname, e.g. registry.example.com
	Username  string
	Password  string // encrypted at rest
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewContainerRegistry(name, url, username, password string) (*ContainerRegistry, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrRegistryNameRequired
	}
	url = strings.TrimSpace(url)
	if url == "" {
		return nil, ErrRegistryURLRequired
	}
	now := time.Now().UTC()
	return &ContainerRegistry{
		ID:        uuid.Must(uuid.NewV7()).String(),
		Name:      name,
		URL:       url,
		Username:  strings.TrimSpace(username),
		Password:  password,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (r *ContainerRegistry) Update(name, url, username, password string) {
	r.Name = strings.TrimSpace(name)
	r.URL = strings.TrimSpace(url)
	r.Username = strings.TrimSpace(username)
	if password != "" {
		r.Password = password
	}
	r.UpdatedAt = time.Now().UTC()
}

type ContainerRegistryRepository interface {
	Save(ctx context.Context, r *ContainerRegistry) error
	FindByID(ctx context.Context, id string) (*ContainerRegistry, error)
	FindAll(ctx context.Context) ([]*ContainerRegistry, error)
	Delete(ctx context.Context, id string) error
}
