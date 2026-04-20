package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	proxmoxdomain "github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
)

// TaskPollInterval is the delay between Proxmox task status polls.
// Override in tests to speed them up.
var TaskPollInterval = 2 * time.Second

// WaitForTask polls a Proxmox UPID task until completion or context cancellation.
// Returns nil on success (exitstatus "OK"), ErrTaskFailed on non-OK exit,
// or ctx.Err() if the context is cancelled before the task completes.
func (c *Client) WaitForTask(ctx context.Context, conn proxmoxdomain.NodeConn, upid string) error {
	nodeName, path, err := upidTaskPath(upid)
	if err != nil {
		return fmt.Errorf("proxmox: parse upid: %w", err)
	}
	// Use UPID's own node name if available, otherwise fall back to conn.NodeName.
	if nodeName == "" {
		nodeName = conn.NodeName
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(TaskPollInterval):
		}

		apiPath := fmt.Sprintf("/nodes/%s/tasks/%s/status", nodeName, path)
		data, err := c.do(ctx, conn, http.MethodGet, apiPath, nil)
		if err != nil {
			return fmt.Errorf("proxmox: poll task: %w", err)
		}

		var status struct {
			Status     string `json:"status"`
			ExitStatus string `json:"exitstatus"`
		}
		if err := parseData(data, &status); err != nil {
			return fmt.Errorf("proxmox: parse task status: %w", err)
		}

		if status.Status != "stopped" {
			continue // still running
		}
		if status.ExitStatus == "OK" {
			return nil
		}
		return proxmoxdomain.ErrTaskFailed{ExitStatus: status.ExitStatus}
	}
}

// upidTaskPath extracts the node name and URL-encoded UPID suitable for the
// tasks API path from a raw Proxmox UPID string.
// UPID format: UPID:{node}:{pid}:{pstart}:{starttime}:{type}:{id}:{user}:
func upidTaskPath(upid string) (nodeName, encoded string, err error) {
	parts := strings.SplitN(upid, ":", 3)
	if len(parts) < 2 || parts[0] != "UPID" {
		// Not a standard UPID — just URL-encode the whole thing
		return "", url.PathEscape(upid), nil
	}
	return parts[1], url.PathEscape(upid), nil
}

// parseUPID unmarshals a Proxmox API response data field that is a UPID string.
func parseUPID(data json.RawMessage) (string, error) {
	var upid string
	if err := json.Unmarshal(data, &upid); err != nil {
		return "", fmt.Errorf("proxmox: parse upid: %w", err)
	}
	return upid, nil
}

// parseData unmarshals a Proxmox API response data field into dst.
func parseData(data json.RawMessage, dst any) error {
	return json.Unmarshal(data, dst)
}
