package bot

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ServiceInfo is a minimal service view used by the bot.
type ServiceInfo struct {
	ID               string
	ProductName      string
	PriceAmountCents int64
	Status           string
	NextBillingDate  *time.Time
}

// CustomerInfo is a minimal customer view used by the bot.
type CustomerInfo struct {
	CompanyName string
	Email       string
	IPAddress   string
}

// InvoiceInfo is a minimal invoice view used by the bot.
type InvoiceInfo struct {
	Code        string
	Status      string
	TotalAmount int64
	IssueDate   time.Time
	PaidAt      *time.Time
}

// RuleContext provides data lookups for rule evaluations.
type RuleContext struct {
	CustomerID   string
	Services     []*ServiceInfo
	Customer     *CustomerInfo
	Invoices     []InvoiceInfo
	OverdueCount int
	OverdueTotal int64
}

// MatchRules checks the user message against all built-in rules.
// Returns (reply, matched, suggestHandoff).
// ctx is unused but kept for future async rule evaluation.
func MatchRules(_ context.Context, msg string, rc *RuleContext) (string, bool, bool) {
	low := strings.ToLower(msg)

	// Handoff trigger — must be checked first.
	if containsAny(low, "human", "agent", "person", "staff", "operator", "talk to", "speak to", "real person") {
		return "Connecting you to a staff member. Please hold for a moment.", true, true
	}

	// Balance / overdue invoices.
	if containsAny(low, "balance", "how much", "owe", "unpaid", "overdue", "outstanding") {
		if rc.OverdueCount == 0 {
			return "Great news — you have no overdue invoices!", true, false
		}
		return fmt.Sprintf("You have %d overdue invoice(s) totalling %s.",
			rc.OverdueCount, formatCents(rc.OverdueTotal)), true, false
	}

	// Invoice list.
	if containsAny(low, "invoice", "bill", "receipt", "payment history") {
		if len(rc.Invoices) == 0 {
			return "You don't have any invoices yet.", true, false
		}
		last := rc.Invoices[0]
		status := last.Status
		return fmt.Sprintf("Your most recent invoice is %s (%s) for %s, issued on %s.",
			last.Code, status, formatCents(last.TotalAmount), last.IssueDate.Format("Jan 2, 2006")), true, false
	}

	// Services.
	if containsAny(low, "service", "internet", "subscription", "plan", "active", "connected") {
		active := activeServices(rc.Services)
		if len(active) == 0 {
			return "You don't have any active services at the moment. Contact us to get started.", true, false
		}
		var names []string
		for _, s := range active {
			names = append(names, s.ProductName)
		}
		return fmt.Sprintf("Your active service(s): %s.", strings.Join(names, ", ")), true, false
	}

	// IP / connection info.
	if containsAny(low, " ip ", "ip address", "address", "connection detail", "my ip") {
		if rc.Customer == nil || rc.Customer.IPAddress == "" {
			return "I don't have an IP address on file for your account. Please contact support.", true, false
		}
		return fmt.Sprintf("Your IP address is %s.", rc.Customer.IPAddress), true, false
	}

	// Last payment.
	if containsAny(low, "pay", "paid", "last payment", "when did i pay") {
		for _, inv := range rc.Invoices {
			if inv.Status == "paid" && inv.PaidAt != nil {
				return fmt.Sprintf("Your last payment was on %s for %s.",
					inv.PaidAt.Format("Jan 2, 2006"), formatCents(inv.TotalAmount)), true, false
			}
		}
		return "I couldn't find a recent payment in your account.", true, false
	}

	// Next billing date.
	if containsAny(low, "next billing", "billing date", "when will i be charged", "renewal") {
		for _, s := range rc.Services {
			if s.Status == "active" && s.NextBillingDate != nil {
				return fmt.Sprintf("Your next billing date is %s for %s.",
					s.NextBillingDate.Format("Jan 2, 2006"), s.ProductName), true, false
			}
		}
		return "I couldn't find a billing date. Please contact support.", true, false
	}

	// Greetings.
	if containsAny(low, "hello", "hi ", "hey", "good morning", "good afternoon") {
		name := ""
		if rc.Customer != nil {
			name = " " + rc.Customer.CompanyName
		}
		return fmt.Sprintf("Hello%s! How can I help you today?", name), true, false
	}

	return "", false, false
}

func containsAny(s string, keywords ...string) bool {
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func activeServices(svcs []*ServiceInfo) []*ServiceInfo {
	var out []*ServiceInfo
	for _, s := range svcs {
		if s.Status == "active" {
			out = append(out, s)
		}
	}
	return out
}

func formatCents(cents int64) string {
	euros := cents / 100
	c := cents % 100
	if c < 0 {
		c = -c
	}
	return fmt.Sprintf("€%d.%02d", euros, c)
}

