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
//
// If listenAddr is non-empty (e.g. ":4222" or "127.0.0.1:4222"), the server
// also exposes a TCP port so external clients can connect.
//
// When corePass and portalPass are both non-empty, per-user permissions are
// enforced on TCP connections:
//
//	core   — full access (">" publish + subscribe)
//	portal — isp.portal.rpc.> and _INBOX.> only
//
// When only corePass is set (portalPass empty), legacy single-token auth is
// used (opts.Authorization = corePass) for backward compatibility.
//
// Empty corePass = no authentication (development mode only).
func StartEmbedded(listenAddr, corePass, portalPass string) (*natsserver.Server, *nats.Conn, error) {
	opts := &natsserver.Options{
		DontListen: true,
	}

	// Build connect options for the in-process core connection.
	var inProcConnOpts []nats.Option

	switch {
	case corePass != "" && portalPass != "":
		// Per-user mode: isolate portal to its own subject namespace.
		opts.Users = []*natsserver.User{
			{
				Username: "core",
				Password: corePass,
				Permissions: &natsserver.Permissions{
					Publish:   &natsserver.SubjectPermission{Allow: []string{">"}},
					Subscribe: &natsserver.SubjectPermission{Allow: []string{">"}},
				},
			},
			{
				Username: "portal",
				Password: portalPass,
				Permissions: &natsserver.Permissions{
					Publish:   &natsserver.SubjectPermission{Allow: []string{"isp.portal.rpc.>", "_INBOX.>"}},
					Subscribe: &natsserver.SubjectPermission{Allow: []string{"isp.portal.rpc.>", "_INBOX.>"}},
				},
			},
		}
		inProcConnOpts = append(inProcConnOpts, nats.UserInfo("core", corePass))

	case corePass != "":
		// Legacy single-token mode (backward compat).
		opts.Authorization = corePass
		inProcConnOpts = append(inProcConnOpts, nats.Token(corePass))
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

	connectOpts := append([]nats.Option{nats.InProcessServer(ns)}, inProcConnOpts...)
	nc, err := nats.Connect(nats.DefaultURL, connectOpts...)
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
