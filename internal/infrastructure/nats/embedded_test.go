package nats

import (
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestStartEmbedded_InProcess(t *testing.T) {
	ns, nc, err := StartEmbedded("", "", "")
	if err != nil {
		t.Fatalf("start embedded: %v", err)
	}
	defer ns.Shutdown()
	defer nc.Close()

	if !nc.IsConnected() {
		t.Fatal("expected in-process connection to be connected")
	}
}

func TestStartEmbedded_WithListenAddr_ExternalConnect(t *testing.T) {
	ns, nc, err := StartEmbedded("127.0.0.1:0", "", "")
	if err != nil {
		t.Fatalf("start embedded with listen: %v", err)
	}
	defer ns.Shutdown()
	defer nc.Close()

	addr := ns.Addr()
	if addr == nil {
		t.Fatal("expected server to have a bound address")
	}

	extNC, err := nats.Connect(fmt.Sprintf("nats://%s", addr.String()),
		nats.Timeout(2*time.Second),
	)
	if err != nil {
		t.Fatalf("external connect to embedded NATS: %v", err)
	}
	defer extNC.Close()

	if !extNC.IsConnected() {
		t.Fatal("expected external connection to be connected")
	}
}

func TestConnectExternal_InvalidURL(t *testing.T) {
	_, err := ConnectExternal("nats://127.0.0.1:1") // nothing listening
	if err == nil {
		t.Fatal("expected error connecting to nothing")
	}
}

func TestStartEmbedded_PerUserPermissions(t *testing.T) {
	ns, nc, err := StartEmbedded("127.0.0.1:0", "core-secret", "portal-secret")
	if err != nil {
		t.Fatalf("start embedded with per-user auth: %v", err)
	}
	defer ns.Shutdown()
	defer nc.Close()

	if !nc.IsConnected() {
		t.Fatal("core in-process connection should be connected")
	}

	addr := ns.Addr()
	if addr == nil {
		t.Fatal("expected bound address")
	}
	natsURL := fmt.Sprintf("nats://%s", addr.String())

	// Portal user can connect and use allowed subjects.
	portalNC, err := nats.Connect(natsURL,
		nats.UserInfo("portal", "portal-secret"),
		nats.Timeout(2*time.Second),
	)
	if err != nil {
		t.Fatalf("portal user connect: %v", err)
	}
	defer portalNC.Close()

	if !portalNC.IsConnected() {
		t.Fatal("portal connection should be connected")
	}

	// Portal user subscribing to allowed subject should work.
	_, err = portalNC.Subscribe("isp.portal.rpc.test", func(m *nats.Msg) {})
	if err != nil {
		t.Fatalf("portal subscribe to allowed subject: %v", err)
	}

	// Wrong credentials should be rejected.
	_, err = nats.Connect(natsURL,
		nats.UserInfo("portal", "wrong-password"),
		nats.Timeout(2*time.Second),
	)
	if err == nil {
		t.Fatal("expected connection failure with wrong password")
	}

	// Core user has full access.
	coreExtNC, err := nats.Connect(natsURL,
		nats.UserInfo("core", "core-secret"),
		nats.Timeout(2*time.Second),
	)
	if err != nil {
		t.Fatalf("core external connect: %v", err)
	}
	defer coreExtNC.Close()

	_, err = coreExtNC.Subscribe("isp.anything.>", func(m *nats.Msg) {})
	if err != nil {
		t.Fatalf("core subscribe to any subject: %v", err)
	}
}
