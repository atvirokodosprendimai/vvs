package http

import (
	"net/http"
	"time"

	"github.com/starfederation/datastar-go/datastar"
)

func clockSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Send immediately
	sse.PatchElementTempl(ClockFragment(time.Now().Format("15:04:05")))

	for {
		select {
		case <-r.Context().Done():
			return
		case t := <-ticker.C:
			if sse.IsClosed() {
				return
			}
			sse.PatchElementTempl(ClockFragment(t.Format("15:04:05")))
		}
	}
}
