package testutil

import (
	"testing"

	infraNats "github.com/atvirokodosprendimai/vvs/internal/infrastructure/nats"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

// NewTestNATS starts an embedded NATS server for testing.
// Returns an EventPublisher and EventSubscriber that implement the shared interfaces.
// The server is automatically stopped via t.Cleanup.
func NewTestNATS(t *testing.T) (events.EventPublisher, events.EventSubscriber) {
	t.Helper()

	ns, nc, err := infraNats.StartEmbedded("", "", "")
	if err != nil {
		t.Fatalf("testutil: start embedded nats: %v", err)
	}

	pub := infraNats.NewPublisher(nc)
	sub := infraNats.NewSubscriber(nc)

	t.Cleanup(func() {
		_ = sub.Close()
		nc.Close()
		ns.Shutdown()
	})

	return pub, sub
}
