package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	stripe "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/webhook"
)

// LineItem is a single product/price line for a Checkout session.
type LineItem struct {
	Name        string
	Description string
	AmountCents int64
	Currency    string
	Quantity    int64
}

// CheckoutParams configures a new Stripe Checkout session.
type CheckoutParams struct {
	SuccessURL    string
	CancelURL     string
	CustomerEmail string
	LineItems     []LineItem
	Metadata      map[string]string
}

// Client wraps the Stripe SDK for checkout session creation and webhook validation.
type Client struct {
	secretKey      string
	webhookSecret  string
	publishableKey string
}

// New creates a Stripe client. secretKey and webhookSecret are required.
func New(secretKey, webhookSecret, publishableKey string) *Client {
	return &Client{
		secretKey:      secretKey,
		webhookSecret:  webhookSecret,
		publishableKey: publishableKey,
	}
}

// PublishableKey returns the Stripe publishable key for frontend use.
func (c *Client) PublishableKey() string { return c.publishableKey }

// CreateCheckoutSession creates a Stripe Checkout session and returns the session ID and redirect URL.
func (c *Client) CreateCheckoutSession(_ context.Context, params CheckoutParams) (sessionID, url string, err error) {
	stripe.Key = c.secretKey

	items := make([]*stripe.CheckoutSessionLineItemParams, len(params.LineItems))
	for i, li := range params.LineItems {
		currency := li.Currency
		if currency == "" {
			currency = "eur"
		}
		qty := li.Quantity
		if qty == 0 {
			qty = 1
		}
		items[i] = &stripe.CheckoutSessionLineItemParams{
			PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
				Currency: stripe.String(currency),
				ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
					Name:        stripe.String(li.Name),
					Description: stripe.String(li.Description),
				},
				UnitAmount: stripe.Int64(li.AmountCents),
			},
			Quantity: stripe.Int64(qty),
		}
	}

	meta := make(map[string]string, len(params.Metadata))
	for k, v := range params.Metadata {
		meta[k] = v
	}

	sp := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		Mode:               stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL:         stripe.String(params.SuccessURL),
		CancelURL:          stripe.String(params.CancelURL),
		LineItems:          items,
		Metadata:           meta,
	}
	if params.CustomerEmail != "" {
		sp.CustomerEmail = stripe.String(params.CustomerEmail)
	}

	s, err := session.New(sp)
	if err != nil {
		return "", "", fmt.Errorf("stripe: create checkout session: %w", err)
	}
	return s.ID, s.URL, nil
}

// ConstructEvent validates the Stripe webhook signature and returns the parsed event.
// Call this with the raw (unread) request body and the Stripe-Signature header.
func (c *Client) ConstructEvent(payload []byte, sigHeader string) (stripe.Event, error) {
	return webhook.ConstructEvent(payload, sigHeader, c.webhookSecret)
}

// ReadCheckoutSessionCompleted parses a checkout.session.completed event payload
// into a CheckoutSessionMetadata for dispatch.
func ReadCheckoutSessionCompleted(event stripe.Event) (sessionID string, metadata map[string]string, customerEmail string, err error) {
	var cs stripe.CheckoutSession
	if err = json.Unmarshal(event.Data.Raw, &cs); err != nil {
		return "", nil, "", fmt.Errorf("stripe: parse checkout session: %w", err)
	}
	return cs.ID, cs.Metadata, cs.CustomerEmail, nil
}

// ReadRawBody reads and returns the full request body without consuming it.
// Must be called before any JSON/form parsing.
func ReadRawBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}
