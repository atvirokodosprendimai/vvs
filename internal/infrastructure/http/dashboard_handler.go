package http

import (
	"fmt"
	"net/http"

	"github.com/starfederation/datastar-go/datastar"
	"gorm.io/gorm"
)

func newDashboardStatsHandler(reader *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sse := datastar.NewSSE(w, r)

		var customerCount, productCount, invoiceCount, paymentCount int64
		reader.Table("customers").Count(&customerCount)
		reader.Table("products").Count(&productCount)
		reader.Table("invoices").Where("status NOT IN ?", []string{"paid", "void"}).Count(&invoiceCount)
		reader.Table("payments").Where("status IN ?", []string{"imported", "unmatched"}).Count(&paymentCount)

		sse.PatchElementTempl(DashboardStats(
			fmt.Sprintf("%d", customerCount),
			fmt.Sprintf("%d", productCount),
			fmt.Sprintf("%d", invoiceCount),
			fmt.Sprintf("%d", paymentCount),
		))
	}
}
