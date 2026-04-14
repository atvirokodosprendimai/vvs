package itaxtlt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// HTTPDebtorClient calls the real itax.lt API.
// TODO: implement once API credentials are available.
// Expected endpoint: GET /api/v2/debtors (Basic Auth)
// Expected response: [{"client_code": "123456789", "over_credit_budget": true}, ...]
type HTTPDebtorClient struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

func NewHTTPDebtorClient(baseURL, username, password string) *HTTPDebtorClient {
	return &HTTPDebtorClient{
		baseURL:    baseURL,
		username:   username,
		password:   password,
		httpClient: &http.Client{},
	}
}

type debtorResponse struct {
	ClientCode       string `json:"client_code"`
	OverCreditBudget bool   `json:"over_credit_budget"`
}

func (c *HTTPDebtorClient) FetchDebtors(ctx context.Context) ([]DebtorRecord, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v2/debtors", nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch debtors: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("itax.lt returned status %d", resp.StatusCode)
	}

	var raw []debtorResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	records := make([]DebtorRecord, len(raw))
	for i, r := range raw {
		records[i] = DebtorRecord{
			ClientCode:       r.ClientCode,
			OverCreditBudget: r.OverCreditBudget,
		}
	}
	return records, nil
}
