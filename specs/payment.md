# Payment Module Spec

## Status: In Progress

## Domain
- **Aggregate**: Payment (ID, Amount, Reference, PayerName, PayerIBAN, BookingDate, InvoiceID, CustomerID, Status, ImportBatchID)
- **Statuses**: Imported, Matched, Unmatched, ManuallyMatched

## Import Pipeline
1. User uploads CSV file
2. ImporterRegistry selects parser by format
3. Parser returns []Payment with Status=Imported
4. Batch saved to database
5. Auto-matching attempted (by reference containing invoice number)
6. Unmatched payments shown for manual matching

## SEPA CSV Format
```
Date;Amount;Reference;PayerName;PayerIBAN
2026-01-15;150.00;INV-2026-00001;ACME Corp;DE89370400440532013000
```

## Importer Interface
- PaymentImporter: Format() string, Parse(ctx, reader) ([]*Payment, error)
- Registry: Register, Get(format), Available()
- Extensible: add MT940, OFX, etc. by implementing interface

## Commands
- RecordPayment(Amount, Reference, PayerName, PayerIBAN, BookingDate)
- ImportPayments(Format, FileReader)
- MatchPayment(PaymentID, InvoiceID, CustomerID)

## Cross-Module Integration
- isp.payment.matched -> Invoice module marks invoice as Paid

## Events
- isp.payment.recorded, isp.payment.imported, isp.payment.matched
