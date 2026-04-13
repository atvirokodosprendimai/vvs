# Product Module Spec

## Status: In Progress

## Domain
- **Aggregate**: Product (ID, Name, Description, Type, Price, BillingPeriod, IsActive)
- **Types**: Internet, VoIP, Hosting, Custom
- **Billing Periods**: Monthly, Quarterly, Yearly
- **Price**: Money value object (int64 cents + currency)

## Commands
- CreateProduct(Name, Description, Type, PriceAmount, PriceCurrency, BillingPeriod)
- UpdateProduct(ID, all fields)
- DeleteProduct(ID)

## Queries
- ListProducts(Search, Type, Page, PageSize)
- GetProduct(ID)

## Events
- isp.product.created
- isp.product.updated
- isp.product.deleted
