package proxmox_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/proxmox"
	proxmoxdomain "github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Speed up task polling for tests.
	proxmox.TaskPollInterval = 5 * time.Millisecond
}

// pveJSON wraps data in the Proxmox {"data": ...} envelope.
func pveJSON(t *testing.T, data any) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{"data": data})
	require.NoError(t, err)
	return b
}

// connFor returns a NodeConn pointing at the given httptest.TLSServer.
func connFor(t *testing.T, srv *httptest.Server) proxmoxdomain.NodeConn {
	t.Helper()
	addr := srv.Listener.Addr().String() // "127.0.0.1:PORT"
	host, portStr, _ := strings.Cut(addr, ":")
	port, _ := strconv.Atoi(portStr)
	return proxmoxdomain.NodeConn{
		NodeName:    "pve",
		Host:        host,
		Port:        port,
		User:        "root@pam",
		TokenID:     "vvs",
		TokenSecret: "secret",
		InsecureTLS: true,
	}
}

func TestNextVMID(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api2/json/cluster/nextid", r.URL.Path)
		w.Write(pveJSON(t, "101"))
	}))
	defer srv.Close()

	id, err := proxmox.New().NextVMID(context.Background(), connFor(t, srv))
	require.NoError(t, err)
	assert.Equal(t, 101, id)
}

func TestWaitForTask_Success(t *testing.T) {
	calls := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		status := "running"
		if calls >= 3 {
			status = "stopped"
		}
		w.Write(pveJSON(t, map[string]string{"status": status, "exitstatus": "OK"}))
	}))
	defer srv.Close()

	upid := "UPID:pve:00001234:00000001:00000001:qmclone:101:root@pam:"
	err := proxmox.New().WaitForTask(context.Background(), connFor(t, srv), upid)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, calls, 3)
}

func TestWaitForTask_TaskFailed(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(pveJSON(t, map[string]string{
			"status":     "stopped",
			"exitstatus": "ERROR: something went wrong",
		}))
	}))
	defer srv.Close()

	upid := "UPID:pve:00001234:00000001:00000001:qmclone:101:root@pam:"
	err := proxmox.New().WaitForTask(context.Background(), connFor(t, srv), upid)
	require.Error(t, err)

	var taskErr proxmoxdomain.ErrTaskFailed
	assert.ErrorAs(t, err, &taskErr)
	assert.Contains(t, taskErr.ExitStatus, "ERROR")
}

func TestWaitForTask_ContextCancelled(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(pveJSON(t, map[string]string{"status": "running"}))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	upid := "UPID:pve:00001234:00000001:00000001:qmclone:101:root@pam:"
	err := proxmox.New().WaitForTask(ctx, connFor(t, srv), upid)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
