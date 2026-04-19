package bot

import (
	"fmt"
	"strings"
)

// BuildSystemPrompt builds the LLM system prompt with injected customer context.
func BuildSystemPrompt(customer *CustomerInfo, services []*ServiceInfo, invoices []InvoiceInfo) string {
	var sb strings.Builder
	sb.WriteString("You are a helpful and concise support assistant for an ISP (Internet Service Provider). ")
	sb.WriteString("Answer questions politely and briefly. If you cannot help, say so and offer to connect the customer with a staff member.\n\n")

	if customer != nil {
		sb.WriteString(fmt.Sprintf("Customer: %s", customer.CompanyName))
		if customer.Email != "" {
			sb.WriteString(fmt.Sprintf(" <%s>", customer.Email))
		}
		sb.WriteString("\n")
		if customer.IPAddress != "" {
			sb.WriteString(fmt.Sprintf("IP Address: %s\n", customer.IPAddress))
		}
	}

	active := activeServices(services)
	if len(active) > 0 {
		var names []string
		for _, s := range active {
			names = append(names, fmt.Sprintf("%s (%s, %s/mo)", s.ProductName, s.Status, formatCents(s.PriceAmountCents)))
		}
		sb.WriteString(fmt.Sprintf("Active services: %s\n", strings.Join(names, ", ")))
	} else {
		sb.WriteString("Active services: none\n")
	}

	overdue := 0
	var overdueTotal int64
	for _, inv := range invoices {
		if inv.Status == "finalized" {
			overdue++
			overdueTotal += inv.TotalAmount
		}
	}
	if overdue > 0 {
		sb.WriteString(fmt.Sprintf("Overdue invoices: %d (total %s)\n", overdue, formatCents(overdueTotal)))
	} else {
		sb.WriteString("Overdue invoices: none\n")
	}

	sb.WriteString("\nKeep your answers short (1-3 sentences). Do not make up information not provided above.")
	return sb.String()
}
