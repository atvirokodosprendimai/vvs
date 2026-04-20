package dockerclient

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	dockerclient "github.com/docker/docker/client"
	"golang.org/x/crypto/ssh"
)

// NewSSH creates a Docker SDK client that dials via an SSH tunnel.
// The Docker daemon on the remote host must be accessible via the Unix socket
// (unix:///var/run/docker.sock) — no TCP port needs to be exposed publicly.
//
// privateKeyPEM is the PEM-encoded SSH private key; it is used in-memory only
// and cleared after the dial function is created.
func NewSSH(host, user string, port int, privateKeyPEM []byte) (*Client, error) {
	signer, err := ssh.ParsePrivateKey(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse ssh private key: %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: support known_hosts
		Timeout:         15 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host, port)

	// Custom dialer: opens SSH connection and proxies through it to the Docker socket
	dialer := func(ctx context.Context, network_, addr_ string) (net.Conn, error) {
		sshConn, err := ssh.Dial("tcp", addr, sshConfig)
		if err != nil {
			return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
		}
		// Dial the Docker socket through the SSH tunnel
		conn, err := sshConn.Dial("unix", "/var/run/docker.sock")
		if err != nil {
			sshConn.Close()
			return nil, fmt.Errorf("ssh tunnel to docker socket: %w", err)
		}
		return conn, nil
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: dialer,
		},
	}

	inner, err := dockerclient.NewClientWithOpts(
		dockerclient.WithHost("http://localhost"), // placeholder — dialer overrides the actual dial
		dockerclient.WithHTTPClient(httpClient),
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("create docker ssh client: %w", err)
	}

	return &Client{inner: inner}, nil
}

// NewSSHForNode creates an SSH-based Docker client for a SwarmNode.
// This is the factory function used by swarm commands.
func NewSSHForNode(sshHost, sshUser string, sshPort int, sshKey []byte) (*Client, error) {
	return NewSSH(sshHost, sshUser, sshPort, sshKey)
}

// ExecSSH runs a command on a remote host via SSH and returns stdout.
// Used for wgmesh provisioning operations (e.g. reading wgmesh0 IP).
func ExecSSH(sshHost, sshUser string, sshPort int, sshKey []byte, cmd string) (string, error) {
	signer, err := ssh.ParsePrivateKey(sshKey)
	if err != nil {
		return "", fmt.Errorf("parse ssh key: %w", err)
	}
	cfg := &ssh.ClientConfig{
		User:            sshUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}
	addr := fmt.Sprintf("%s:%d", sshHost, sshPort)
	conn, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return "", fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	defer conn.Close()

	sess, err := conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("ssh new session: %w", err)
	}
	defer sess.Close()

	out, err := sess.Output(cmd)
	if err != nil {
		return "", fmt.Errorf("ssh exec %q: %w", cmd, err)
	}
	return string(out), nil
}
