# Invoice Module Spec

## Status: In Progress

## Domain
- **Aggregate**: Invoice (ID, Number, CustomerID/Name, Lines, Subtotal, TaxRate, TaxAmount, Total, Status, Dates)
- **Entity**: InvoiceLine (ProductID/Name, Description, Quantity, UnitPrice, Total)
- **Number Format**: INV-{YEAR}-{NNNNN} e.g. INV-2026-00001
- **Statuses**: Draft -> Finalized -> Sent -> Paid | Overdue | Void

## Status Transitions
- Draft -> Finalized (locks line items)
- Finalized -> Sent (email sent)
- Finalized/Sent -> Paid (payment received)
- Finalized/Sent -> Overdue (past due date)
- Draft/Finalized -> Void (cancelled)

## Tax Calculation
- TaxRate as integer percentage (e.g. 21 = 21%)
- TaxAmount = Subtotal * TaxRate / 100
- Total = Subtotal + TaxAmount

## Commands
- CreateInvoice(CustomerID, CustomerName, Lines, IssueDate, DueDate, TaxRate)
- FinalizeInvoice(ID)
- VoidInvoice(ID)

## Events
- isp.invoice.created, isp.invoice.finalized, isp.invoice.voided, isp.invoice.paid
