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

		var customerCount, productCount int64
		reader.Table("customers").Count(&customerCount)
		reader.Table("products").Count(&productCount)

		sse.PatchElementTempl(DashboardStats(
			fmt.Sprintf("%d", customerCount),
			fmt.Sprintf("%d", productCount),
		))
	}
}
