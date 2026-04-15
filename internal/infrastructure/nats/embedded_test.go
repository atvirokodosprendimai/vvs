package nats

import (
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestStartEmbedded_InProcess(t *testing.T) {
	ns, nc, err := StartEmbedded("")
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
	ns, nc, err := StartEmbedded("127.0.0.1:0")
	if err != nil {
		t.Fatalf("start embedded with listen: %v", err)
	}
	defer ns.Shutdown()
	defer nc.Close()

	// Get the actual port assigned
	addr := ns.Addr()
	if addr == nil {
		t.Fatal("expected server to have a bound address")
	}

	// Connect externally via TCP
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
