package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
)

func TestNewVVSDeployment_Valid(t *testing.T) {
	dep, err := domain.NewVVSDeployment(
		"cluster-1", "node-1",
		domain.VVSComponentPortal, domain.VVSDeployImage,
		"nats://10.0.0.1:4222", 8080,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, dep.ID)
	assert.Equal(t, domain.VVSComponentPortal, dep.Component)
	assert.Equal(t, domain.VVSDeployImage, dep.Source)
	assert.Equal(t, domain.VVSDeploymentPending, dep.Status)
	assert.Equal(t, 8080, dep.Port)
	assert.NotNil(t, dep.EnvVars)
}

func TestNewVVSDeployment_DefaultPort(t *testing.T) {
	dep, err := domain.NewVVSDeployment("c", "n", domain.VVSComponentPortal, domain.VVSDeployImage, "nats://x:4222", 0)
	require.NoError(t, err)
	assert.Equal(t, 8080, dep.Port)

	dep2, _ := domain.NewVVSDeployment("c", "n", domain.VVSComponentSTB, domain.VVSDeployImage, "nats://x:4222", 0)
	assert.Equal(t, 8090, dep2.Port)
}

func TestNewVVSDeployment_MissingNode(t *testing.T) {
	_, err := domain.NewVVSDeployment("c", "", domain.VVSComponentPortal, domain.VVSDeployImage, "nats://x", 0)
	assert.ErrorIs(t, err, domain.ErrDeploymentNodeRequired)
}

func TestNewVVSDeployment_MissingComponent(t *testing.T) {
	_, err := domain.NewVVSDeployment("c", "n", "", domain.VVSDeployImage, "nats://x", 0)
	assert.ErrorIs(t, err, domain.ErrDeploymentComponentRequired)
}

func TestNewVVSDeployment_MissingSource(t *testing.T) {
	_, err := domain.NewVVSDeployment("c", "n", domain.VVSComponentPortal, "", "nats://x", 0)
	assert.ErrorIs(t, err, domain.ErrDeploymentSourceRequired)
}

func TestNewVVSDeployment_MissingNATS(t *testing.T) {
	_, err := domain.NewVVSDeployment("c", "n", domain.VVSComponentPortal, domain.VVSDeployImage, "", 0)
	assert.ErrorIs(t, err, domain.ErrDeploymentNATSRequired)
}

func TestVVSDeployment_StatusTransitions(t *testing.T) {
	dep, _ := domain.NewVVSDeployment("c", "n", domain.VVSComponentPortal, domain.VVSDeployImage, "nats://x", 0)
	assert.Equal(t, domain.VVSDeploymentPending, dep.Status)

	dep.MarkRunning()
	assert.Equal(t, domain.VVSDeploymentRunning, dep.Status)
	assert.NotNil(t, dep.LastDeployedAt)
	assert.Empty(t, dep.ErrorMsg)

	dep.MarkError("connection refused")
	assert.Equal(t, domain.VVSDeploymentError, dep.Status)
	assert.Equal(t, "connection refused", dep.ErrorMsg)

	dep.MarkStopped()
	assert.Equal(t, domain.VVSDeploymentStopped, dep.Status)
}

func TestVVSDeployment_ServiceName(t *testing.T) {
	dep, _ := domain.NewVVSDeployment("c", "n", domain.VVSComponentPortal, domain.VVSDeployImage, "nats://x", 0)
	assert.Equal(t, "vvs-portal", dep.ServiceName())

	dep2, _ := domain.NewVVSDeployment("c", "n", domain.VVSComponentSTB, domain.VVSDeployImage, "nats://x", 0)
	assert.Equal(t, "vvs-stb", dep2.ServiceName())
}

func TestVVSDeployment_ComposePath(t *testing.T) {
	dep, _ := domain.NewVVSDeployment("c", "n", domain.VVSComponentPortal, domain.VVSDeployImage, "nats://x", 0)
	assert.Contains(t, dep.ComposePath(), "/opt/vvs/components/portal/")
	assert.Contains(t, dep.ComposePath(), dep.ID)
	assert.Contains(t, dep.ComposePath(), "docker-compose.yml")
}
