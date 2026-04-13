# Customer Module Spec

## Status: Complete

## Domain
- **Aggregate**: Customer (ID, Code, CompanyName, ContactName, Email, Phone, Address fields, TaxID, Status, Notes)
- **Value Objects**: CompanyCode (CLI-00001 format)
- **Statuses**: Active, Suspended, Churned
- **Invariants**: CompanyName required, Code unique and auto-generated

## Code Generation
- Sequential counter in `company_code_sequences` table
- Atomic increment via WriteSerializer (race-free)
- Format: `{PREFIX}-{NNNNN}` e.g. CLI-00001

## Commands
- CreateCustomer(CompanyName, ContactName, Email, Phone) -> publishes isp.customer.created
- UpdateCustomer(ID, all fields) -> publishes isp.customer.updated
- DeleteCustomer(ID) -> publishes isp.customer.deleted

## Queries
- ListCustomers(Search, Status, Page, PageSize) -> paginated results
- GetCustomer(ID) -> single customer

## HTTP Routes
- GET /customers - list page
- GET /customers/new - create form
- GET /customers/{id} - detail page
- GET /customers/{id}/edit - edit form
- GET /api/customers - SSE list data
- POST /api/customers - SSE create
- PUT /api/customers/{id} - SSE update
- DELETE /api/customers/{id} - SSE delete

## Events
- isp.customer.created {id, code, name}
- isp.customer.updated {id, name}
- isp.customer.deleted {id, code}
