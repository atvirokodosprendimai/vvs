# Recurring Invoice Module Spec

## Status: In Progress

## Domain
- **Aggregate**: RecurringInvoice (ID, CustomerID/Name, Lines, Schedule, NextRunDate, LastRunDate, Status)
- **Value Object**: Schedule (Frequency + DayOfMonth)
- **Frequencies**: Monthly, Quarterly, Yearly
- **DayOfMonth**: 1-28 (avoids month-end edge cases)
- **Statuses**: Active, Paused, Cancelled

## Schedule Logic
- NextRunDate calculated based on frequency + day of month
- Monthly: same day each month
- Quarterly: every 3 months
- Yearly: every 12 months
- IsDue(asOf) = NextRunDate <= asOf && Status == Active

## Commands
- CreateRecurring(CustomerID, CustomerName, Frequency, DayOfMonth, Lines)
- UpdateRecurring(ID, fields)
- ToggleRecurring(ID) - pause/resume

## Cross-Module Integration
- When recurring invoice is due, generates Invoice via isp.recurring.generated event
- Invoice module listens and creates a new Draft invoice

## Events
- isp.recurring.created, isp.recurring.updated, isp.recurring.paused, isp.recurring.resumed
