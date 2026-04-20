// Package hetzner provides a minimal Hetzner Cloud API client for server lifecycle.
package hetzner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const apiBase = "https://api.hetzner.cloud/v1"

// ServerStatus values returned by the Hetzner API.
type ServerStatus string

const (
	ServerRunning      ServerStatus = "running"
	ServerInitializing ServerStatus = "initializing"
	ServerStarting     ServerStatus = "starting"
	ServerOff          ServerStatus = "off"
)

// CreateServerRequest is the body for POST /v1/servers.
type CreateServerRequest struct {
	Name       string `json:"name"`
	ServerType string `json:"server_type"`
	Image      string `json:"image"`
	Location   string `json:"location"`
	SSHKeys    []int  `json:"ssh_keys,omitempty"`
	UserData   string `json:"user_data,omitempty"`
}

// ServerResponse represents the server object from Hetzner API.
type ServerResponse struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	PublicNet struct {
		IPv4 struct {
			IP string `json:"ip"`
		} `json:"ipv4"`
	} `json:"public_net"`
}

type createServerResp struct {
	Server ServerResponse `json:"server"`
}

type getServerResp struct {
	Server ServerResponse `json:"server"`
}

// CreateServer creates a new Hetzner VPS and returns its server ID.
// SSH key is injected at creation time via sshKeyID.
func CreateServer(ctx context.Context, apiKey string, req CreateServerRequest) (int, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return 0, fmt.Errorf("marshal create server request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"/servers", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("create server request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errBody struct {
			Error struct {
				Message string `json:"message"`
				Code    string `json:"code"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		return 0, fmt.Errorf("hetzner API error %d: %s (%s)", resp.StatusCode, errBody.Error.Message, errBody.Error.Code)
	}

	var result createServerResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode create server response: %w", err)
	}
	return result.Server.ID, nil
}

// GetServer fetches the current state of a server by ID.
func GetServer(ctx context.Context, apiKey string, serverID int) (*ServerResponse, error) {
	url := fmt.Sprintf("%s/servers/%d", apiBase, serverID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("get server request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hetzner get server %d: status %d", serverID, resp.StatusCode)
	}

	var result getServerResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode get server response: %w", err)
	}
	return &result.Server, nil
}

// PollUntilRunning polls the server every 5 s until status=running or timeout.
// Returns the server's public IPv4 address.
func PollUntilRunning(ctx context.Context, apiKey string, serverID int, timeout time.Duration, progress func(string)) (string, error) {
	deadline := time.Now().Add(timeout)
	interval := 5 * time.Second
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		srv, err := GetServer(ctx, apiKey, serverID)
		if err != nil {
			return "", fmt.Errorf("poll server: %w", err)
		}
		if progress != nil {
			progress(fmt.Sprintf("Server status: %s", srv.Status))
		}
		if srv.Status == string(ServerRunning) {
			ip := srv.PublicNet.IPv4.IP
			if ip == "" {
				return "", fmt.Errorf("server running but no IPv4 assigned")
			}
			return ip, nil
		}
		time.Sleep(interval)
	}
	return "", fmt.Errorf("server not running after %s", timeout)
}

// EnsureSSHKey creates the SSH key in Hetzner if it doesn't exist yet and
// returns its numeric ID. If a key with the same public key already exists
// (uniqueness_error), it fetches and returns the existing key's ID.
func EnsureSSHKey(ctx context.Context, apiKey, name, publicKey string) (int, error) {
	body, _ := json.Marshal(map[string]string{"name": name, "public_key": publicKey})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"/ssh_keys", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("create ssh key: %w", err)
	}
	defer resp.Body.Close()

	var raw struct {
		SSHKey struct {
			ID int `json:"id"`
		} `json:"ssh_key"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&raw)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return raw.SSHKey.ID, nil
	}
	if raw.Error.Code == "uniqueness_error" {
		// Key already registered — find it by listing and matching public key
		return findSSHKeyByPublicKey(ctx, apiKey, publicKey)
	}
	return 0, fmt.Errorf("hetzner create ssh key %d: %s (%s)", resp.StatusCode, raw.Error.Message, raw.Error.Code)
}

func findSSHKeyByPublicKey(ctx context.Context, apiKey, publicKey string) (int, error) {
	keys, err := ListSSHKeys(ctx, apiKey)
	if err != nil {
		return 0, err
	}
	for _, k := range keys {
		if sshKeyMaterial(k.PublicKey) == sshKeyMaterial(publicKey) {
			return k.ID, nil
		}
	}
	return 0, fmt.Errorf("ssh key not found in Hetzner account")
}

// SSHKey is a key registered in the Hetzner account.
type SSHKey struct {
	ID        int    `json:"id"`
	PublicKey string `json:"public_key"`
}

// ListSSHKeys returns all SSH keys registered in the account.
func ListSSHKeys(ctx context.Context, apiKey string) ([]SSHKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+"/ssh_keys", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list ssh keys: %w", err)
	}
	defer resp.Body.Close()

	var list struct {
		SSHKeys []SSHKey `json:"ssh_keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("decode ssh keys: %w", err)
	}
	return list.SSHKeys, nil
}

// sshKeyMaterial returns only the algorithm + base64 part (drops optional comment).
func sshKeyMaterial(key string) string {
	parts := strings.SplitN(strings.TrimSpace(key), " ", 3)
	if len(parts) >= 2 {
		return parts[0] + " " + parts[1]
	}
	return key
}

// ServerType is a Hetzner server type.
type ServerType struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Cores       int    `json:"cores"`
	Memory      float64 `json:"memory"`
	Disk        int    `json:"disk"`
}

// Location is a Hetzner datacenter location.
type Location struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Country     string `json:"country"`
}

// ListServerTypes returns all available server types for the account.
func ListServerTypes(ctx context.Context, apiKey string) ([]ServerType, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+"/server_types?per_page=50", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list server types: %w", err)
	}
	defer resp.Body.Close()
	var list struct {
		ServerTypes []ServerType `json:"server_types"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("decode server types: %w", err)
	}
	return list.ServerTypes, nil
}

// ListLocations returns all available datacenter locations.
func ListLocations(ctx context.Context, apiKey string) ([]Location, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+"/locations", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list locations: %w", err)
	}
	defer resp.Body.Close()
	var list struct {
		Locations []Location `json:"locations"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("decode locations: %w", err)
	}
	return list.Locations, nil
}

// DeleteServer deletes a Hetzner server by ID.
func DeleteServer(ctx context.Context, apiKey string, serverID int) error {
	url := fmt.Sprintf("%s/servers/%d", apiBase, serverID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("delete server request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hetzner delete server %d: status %d", serverID, resp.StatusCode)
	}
	return nil
}
