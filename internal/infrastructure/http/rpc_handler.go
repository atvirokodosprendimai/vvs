package http

import (
	"context"
	"io"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/vvs/isp/internal/infrastructure/http/jsonapi"
)

// RPCDispatcher is implemented by natsrpc.Server.
type RPCDispatcher interface {
	Dispatch(ctx context.Context, subject string, payload []byte) (any, error)
}

// rpcHandler returns an HTTP handler that routes POST /api/v1/rpc/{subject}
// to the RPC server's Dispatch method. Used as the HTTP fallback transport for the CLI.
func rpcHandler(rpc RPCDispatcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subject := "isp.rpc." + chi.URLParam(r, "*")
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			jsonapi.WriteBadRequest(w, "cannot read body")
			return
		}
		result, err := rpc.Dispatch(r.Context(), subject, payload)
		if err != nil {
			log.Printf("rpc dispatch %s: %v", subject, err)
			jsonapi.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		jsonapi.WriteJSON(w, http.StatusOK, result)
	}
}
