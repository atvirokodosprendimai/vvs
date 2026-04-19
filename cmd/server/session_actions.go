package main

import (
	"context"
	"log"

	authpersistence "github.com/atvirokodosprendimai/vvs/internal/modules/auth/adapters/persistence"
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
)

// RegisterSessionActions wires the session pruning cron action.
func RegisterSessionActions(gdb *gormsqlite.DB) {
	repo := authpersistence.NewGormSessionRepository(gdb)

	RegisterAction("prune-sessions", func(ctx context.Context) error {
		if err := repo.PruneExpired(ctx); err != nil {
			log.Printf("prune-sessions: %v", err)
			return err
		}
		log.Printf("prune-sessions: expired sessions pruned")
		return nil
	})
}
