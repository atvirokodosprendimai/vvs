package app

import (
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"

	billingpersistence "github.com/atvirokodosprendimai/vvs/internal/modules/billing/adapters/persistence"
	billingcommands    "github.com/atvirokodosprendimai/vvs/internal/modules/billing/app/commands"
	billingqueries     "github.com/atvirokodosprendimai/vvs/internal/modules/billing/app/queries"
)

type billingWired struct {
	topUpBalance    *billingcommands.TopUpBalanceHandler
	deductBalance   *billingcommands.DeductBalanceHandler
	adjustBalance   *billingcommands.AdjustBalanceHandler
	getBalance      *billingqueries.GetCustomerBalanceHandler
}

func wireBilling(
	gdb *gormsqlite.DB,
	pub events.EventPublisher,
) *billingWired {
	balanceRepo := billingpersistence.NewGormBalanceRepository(gdb)

	topUp  := billingcommands.NewTopUpBalanceHandler(balanceRepo, pub)
	deduct := billingcommands.NewDeductBalanceHandler(balanceRepo, pub)
	adjust := billingcommands.NewAdjustBalanceHandler(topUp, deduct)
	getBalance := billingqueries.NewGetCustomerBalanceHandler(balanceRepo)

	return &billingWired{
		topUpBalance:  topUp,
		deductBalance: deduct,
		adjustBalance: adjust,
		getBalance:    getBalance,
	}
}
