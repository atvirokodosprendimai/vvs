package queries

import (
	"context"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
)

// AppReadModel is the query-side view of a DockerApp.
type AppReadModel struct {
	ID            string
	Name          string
	RepoURL       string
	Branch        string
	RegUser       string
	BuildArgs     []domain.KV
	EnvVars       []domain.KV
	Ports         []domain.PortMap
	Volumes       []domain.VolumeMount
	Networks      []string
	RestartPolicy string
	ContainerName string
	ImageRef      string
	Status        string
	ErrorMsg      string
	LastBuiltAt   *time.Time
}

func toAppRM(a *domain.DockerApp) AppReadModel {
	return AppReadModel{
		ID:            a.ID,
		Name:          a.Name,
		RepoURL:       a.RepoURL,
		Branch:        a.Branch,
		RegUser:       a.RegUser,
		BuildArgs:     a.BuildArgs,
		EnvVars:       a.EnvVars,
		Ports:         a.Ports,
		Volumes:       a.Volumes,
		Networks:      a.Networks,
		RestartPolicy: a.RestartPolicy,
		ContainerName: a.ContainerName,
		ImageRef:      a.ImageRef,
		Status:        string(a.Status),
		ErrorMsg:      a.ErrorMsg,
		LastBuiltAt:   a.LastBuiltAt,
	}
}

// ListDockerAppsHandler returns all apps.
type ListDockerAppsHandler struct{ repo domain.DockerAppRepository }

func NewListDockerAppsHandler(repo domain.DockerAppRepository) *ListDockerAppsHandler {
	return &ListDockerAppsHandler{repo: repo}
}

func (h *ListDockerAppsHandler) Handle(ctx context.Context) ([]AppReadModel, error) {
	apps, err := h.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AppReadModel, len(apps))
	for i, a := range apps {
		out[i] = toAppRM(a)
	}
	return out, nil
}

// GetDockerAppHandler returns a single app by ID.
type GetDockerAppHandler struct{ repo domain.DockerAppRepository }

func NewGetDockerAppHandler(repo domain.DockerAppRepository) *GetDockerAppHandler {
	return &GetDockerAppHandler{repo: repo}
}

func (h *GetDockerAppHandler) Handle(ctx context.Context, id string) (AppReadModel, error) {
	a, err := h.repo.FindByID(ctx, id)
	if err != nil {
		return AppReadModel{}, err
	}
	return toAppRM(a), nil
}
