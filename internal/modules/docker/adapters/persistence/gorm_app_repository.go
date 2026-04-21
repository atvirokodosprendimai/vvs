package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
	"github.com/atvirokodosprendimai/vvs/internal/modules/email/emailcrypto"
	"gorm.io/gorm"
)

// ── DockerApp model ───────────────────────────────────────────────────────────

type DockerAppModel struct {
	ID            string     `gorm:"primaryKey;type:text"`
	Name          string     `gorm:"type:text;not null;default:''"`
	RepoURL       string     `gorm:"column:repo_url;type:text;not null;default:''"`
	Branch        string     `gorm:"type:text;not null;default:'main'"`
	RegUser       string     `gorm:"column:reg_user;type:text;not null;default:''"`
	RegPass       []byte     `gorm:"column:reg_pass"`
	BuildArgs     string     `gorm:"column:build_args;type:text;not null;default:'[]'"`
	EnvVars       string     `gorm:"column:env_vars;type:text;not null;default:'[]'"`
	Ports         string     `gorm:"type:text;not null;default:'[]'"`
	Volumes       string     `gorm:"type:text;not null;default:'[]'"`
	Networks      string     `gorm:"type:text;not null;default:'[]'"`
	RestartPolicy string     `gorm:"column:restart_policy;type:text;not null;default:'unless-stopped'"`
	ContainerName string     `gorm:"column:container_name;type:text;not null;default:''"`
	ImageRef      string     `gorm:"column:image_ref;type:text;not null;default:''"`
	Status        string     `gorm:"type:text;not null;default:'idle'"`
	ErrorMsg      string     `gorm:"column:error_msg;type:text;not null;default:''"`
	LastBuiltAt   *time.Time `gorm:"column:last_built_at"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (DockerAppModel) TableName() string { return "docker_apps" }

func toAppModel(a *domain.DockerApp) *DockerAppModel {
	buildArgsJSON, _ := json.Marshal(a.BuildArgs)
	envVarsJSON, _ := json.Marshal(a.EnvVars)
	portsJSON, _ := json.Marshal(a.Ports)
	volumesJSON, _ := json.Marshal(a.Volumes)
	networksJSON, _ := json.Marshal(a.Networks)
	return &DockerAppModel{
		ID:            a.ID,
		Name:          a.Name,
		RepoURL:       a.RepoURL,
		Branch:        a.Branch,
		RegUser:       a.RegUser,
		BuildArgs:     string(buildArgsJSON),
		EnvVars:       string(envVarsJSON),
		Ports:         string(portsJSON),
		Volumes:       string(volumesJSON),
		Networks:      string(networksJSON),
		RestartPolicy: a.RestartPolicy,
		ContainerName: a.ContainerName,
		ImageRef:      a.ImageRef,
		Status:        string(a.Status),
		ErrorMsg:      a.ErrorMsg,
		LastBuiltAt:   a.LastBuiltAt,
		CreatedAt:     a.CreatedAt,
		UpdatedAt:     a.UpdatedAt,
	}
}

func toAppDomain(m *DockerAppModel) *domain.DockerApp {
	a := &domain.DockerApp{
		ID:            m.ID,
		Name:          m.Name,
		RepoURL:       m.RepoURL,
		Branch:        m.Branch,
		RegUser:       m.RegUser,
		BuildArgs:     []domain.KV{},
		EnvVars:       []domain.KV{},
		Ports:         []domain.PortMap{},
		Volumes:       []domain.VolumeMount{},
		Networks:      []string{},
		RestartPolicy: m.RestartPolicy,
		ContainerName: m.ContainerName,
		ImageRef:      m.ImageRef,
		Status:        domain.AppStatus(m.Status),
		ErrorMsg:      m.ErrorMsg,
		LastBuiltAt:   m.LastBuiltAt,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
	_ = json.Unmarshal([]byte(m.BuildArgs), &a.BuildArgs)
	_ = json.Unmarshal([]byte(m.EnvVars), &a.EnvVars)
	_ = json.Unmarshal([]byte(m.Ports), &a.Ports)
	_ = json.Unmarshal([]byte(m.Volumes), &a.Volumes)
	_ = json.Unmarshal([]byte(m.Networks), &a.Networks)
	return a
}

// ── GormDockerAppRepository ───────────────────────────────────────────────────

type GormDockerAppRepository struct {
	db     *gormsqlite.DB
	encKey []byte
}

func NewGormDockerAppRepository(db *gormsqlite.DB, encKey []byte) *GormDockerAppRepository {
	return &GormDockerAppRepository{db: db, encKey: encKey}
}

func (r *GormDockerAppRepository) Save(ctx context.Context, app *domain.DockerApp) error {
	model := toAppModel(app)
	if app.RegPass != "" {
		enc, err := emailcrypto.EncryptPassword(r.encKey, []byte(app.RegPass))
		if err != nil {
			return fmt.Errorf("encrypt app reg_pass: %w", err)
		}
		model.RegPass = enc
	}
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(model).Error
	})
}

func (r *GormDockerAppRepository) FindByID(ctx context.Context, id string) (*domain.DockerApp, error) {
	var m DockerAppModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&m).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrAppNotFound
		}
		return nil, err
	}
	return r.decryptApp(&m)
}

func (r *GormDockerAppRepository) FindAll(ctx context.Context) ([]*domain.DockerApp, error) {
	var result []*domain.DockerApp
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		var models []DockerAppModel
		if err := tx.Order("name ASC").Find(&models).Error; err != nil {
			return err
		}
		result = make([]*domain.DockerApp, len(models))
		for i := range models {
			app, err := r.decryptApp(&models[i])
			if err != nil {
				return err
			}
			result[i] = app
		}
		return nil
	})
	return result, err
}

func (r *GormDockerAppRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&DockerAppModel{}).Error
	})
}

func (r *GormDockerAppRepository) decryptApp(m *DockerAppModel) (*domain.DockerApp, error) {
	app := toAppDomain(m)
	if len(m.RegPass) > 0 {
		dec, err := emailcrypto.DecryptPassword(r.encKey, m.RegPass)
		if err != nil {
			return nil, fmt.Errorf("decrypt app reg_pass for %s: %w", m.ID, err)
		}
		app.RegPass = string(dec)
	}
	return app, nil
}
