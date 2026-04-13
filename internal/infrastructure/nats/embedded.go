package nats

import (
	"fmt"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func StartEmbedded() (*natsserver.Server, *nats.Conn, error) {
	opts := &natsserver.Options{
		DontListen: true,
	}

	ns, err := natsserver.NewServer(opts)
	if err != nil {
		return nil, nil, fmt.Errorf("create nats server: %w", err)
	}

	ns.Start()

	if !ns.ReadyForConnections(5 * time.Second) {
		return nil, nil, fmt.Errorf("nats server not ready")
	}

	nc, err := nats.Connect(
		nats.DefaultURL,
		nats.InProcessServer(ns),
	)
	if err != nil {
		ns.Shutdown()
		return nil, nil, fmt.Errorf("connect to embedded nats: %w", err)
	}

	return ns, nc, nil
}
