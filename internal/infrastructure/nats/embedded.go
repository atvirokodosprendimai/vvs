package nats

import (
	"fmt"
	"net"
	"strconv"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// StartEmbedded starts an in-process NATS server.
// If listenAddr is non-empty (e.g. ":4222" or "127.0.0.1:4222"), the server
// also exposes a TCP port so external clients can connect.
// If listenAddr is empty the server runs silently in-process only.
// authToken, if non-empty, requires remote clients to authenticate with this token.
func StartEmbedded(listenAddr string, authToken ...string) (*natsserver.Server, *nats.Conn, error) {
	opts := &natsserver.Options{
		DontListen: true,
	}

	if len(authToken) > 0 && authToken[0] != "" {
		opts.Authorization = authToken[0]
	}

	if listenAddr != "" {
		host, portStr, err := net.SplitHostPort(listenAddr)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid nats listen addr %q: %w", listenAddr, err)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid nats listen port %q: %w", portStr, err)
		}
		if host == "" {
			host = "0.0.0.0"
		}
		opts.DontListen = false
		opts.Host = host
		opts.Port = port
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

// ConnectExternal connects to an existing NATS server at the given URL.
// Returns a nil *natsserver.Server (caller manages external server lifecycle).
func ConnectExternal(url string) (*nats.Conn, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("connect to nats %s: %w", url, err)
	}
	return nc, nil
}
