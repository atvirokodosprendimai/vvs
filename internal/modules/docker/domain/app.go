package domain

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrAppNameRequired    = errors.New("docker app name is required")
	ErrAppRepoURLRequired = errors.New("docker app repo URL is required")
	ErrAppNotFound        = errors.New("docker app not found")
)

// KV is a key/value pair used for env vars and build args.
type KV struct {
	Key   string
	Value string
}

// PortMap maps a host port to a container port.
type PortMap struct {
	Host      string // e.g. "8080"
	Container string // e.g. "80"
	Proto     string // "tcp" | "udp"
}

// VolumeMount is a bind mount entry.
type VolumeMount struct {
	Host      string
	Container string
}

type AppStatus string

const (
	AppStatusIdle      AppStatus = "idle"
	AppStatusBuilding  AppStatus = "building"
	AppStatusPushing   AppStatus = "pushing"
	AppStatusDeploying AppStatus = "deploying"
	AppStatusRunning   AppStatus = "running"
	AppStatusError     AppStatus = "error"
	AppStatusStopped   AppStatus = "stopped"
)

// DockerApp is a git-sourced app deployed via local docker.sock.
// The repo must contain a Dockerfile at root. All runtime config
// (env vars, ports, volumes, networks) is defined in VVS — not in the repo.
type DockerApp struct {
	ID            string
	Name          string
	RepoURL       string
	Branch        string
	RegUser       string
	RegPass       string // AES-256-GCM encrypted at rest
	BuildArgs     []KV
	EnvVars       []KV
	Ports         []PortMap
	Volumes       []VolumeMount
	Networks      []string
	RestartPolicy string
	ContainerName string // slugified Name; used as docker container name
	ImageRef      string // last successfully pushed image ref
	Status        AppStatus
	ErrorMsg      string
	LastBuiltAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func NewDockerApp(name, repoURL, branch, regUser, regPass string) (*DockerApp, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrAppNameRequired
	}
	repoURL = strings.TrimSpace(repoURL)
	if repoURL == "" {
		return nil, ErrAppRepoURLRequired
	}
	if branch == "" {
		branch = "main"
	}
	now := time.Now().UTC()
	return &DockerApp{
		ID:            uuid.Must(uuid.NewV7()).String(),
		Name:          name,
		RepoURL:       repoURL,
		Branch:        branch,
		RegUser:       strings.TrimSpace(regUser),
		RegPass:       regPass,
		BuildArgs:     []KV{},
		EnvVars:       []KV{},
		Ports:         []PortMap{},
		Volumes:       []VolumeMount{},
		Networks:      []string{},
		RestartPolicy: "unless-stopped",
		ContainerName: slugify(name),
		Status:        AppStatusIdle,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (a *DockerApp) Update(name, repoURL, branch, regUser, regPass string,
	buildArgs []KV, envVars []KV, ports []PortMap, volumes []VolumeMount,
	networks []string, restartPolicy string,
) {
	if name != "" {
		a.Name = strings.TrimSpace(name)
		a.ContainerName = slugify(a.Name)
	}
	if repoURL != "" {
		a.RepoURL = strings.TrimSpace(repoURL)
	}
	if branch != "" {
		a.Branch = branch
	}
	a.RegUser = strings.TrimSpace(regUser)
	if regPass != "" {
		a.RegPass = regPass
	}
	if buildArgs != nil {
		a.BuildArgs = buildArgs
	}
	if envVars != nil {
		a.EnvVars = envVars
	}
	if ports != nil {
		a.Ports = ports
	}
	if volumes != nil {
		a.Volumes = volumes
	}
	if networks != nil {
		a.Networks = networks
	}
	if restartPolicy != "" {
		a.RestartPolicy = restartPolicy
	}
	a.UpdatedAt = time.Now().UTC()
}

func (a *DockerApp) MarkBuilding() {
	a.Status = AppStatusBuilding
	a.ErrorMsg = ""
	a.UpdatedAt = time.Now().UTC()
}

func (a *DockerApp) MarkPushing() {
	a.Status = AppStatusPushing
	a.UpdatedAt = time.Now().UTC()
}

func (a *DockerApp) MarkDeploying(imageRef string) {
	a.Status = AppStatusDeploying
	a.ImageRef = imageRef
	a.UpdatedAt = time.Now().UTC()
}

func (a *DockerApp) MarkRunning() {
	now := time.Now().UTC()
	a.Status = AppStatusRunning
	a.ErrorMsg = ""
	a.LastBuiltAt = &now
	a.UpdatedAt = now
}

func (a *DockerApp) MarkError(msg string) {
	a.Status = AppStatusError
	a.ErrorMsg = msg
	a.UpdatedAt = time.Now().UTC()
}

func (a *DockerApp) MarkStopped() {
	a.Status = AppStatusStopped
	a.UpdatedAt = time.Now().UTC()
}

// DockerAppRepository is the persistence port for DockerApp.
type DockerAppRepository interface {
	Save(ctx context.Context, app *DockerApp) error
	FindByID(ctx context.Context, id string) (*DockerApp, error)
	FindAll(ctx context.Context) ([]*DockerApp, error)
	Delete(ctx context.Context, id string) error
}
