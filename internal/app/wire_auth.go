package app

import (
	"context"
	"log"

	infrahttp "github.com/atvirokodosprendimai/vvs/internal/infrastructure/http"
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	authhttp "github.com/atvirokodosprendimai/vvs/internal/modules/auth/adapters/http"
	authpersistence "github.com/atvirokodosprendimai/vvs/internal/modules/auth/adapters/persistence"
	authcommands "github.com/atvirokodosprendimai/vvs/internal/modules/auth/app/commands"
	authqueries "github.com/atvirokodosprendimai/vvs/internal/modules/auth/app/queries"
)

type authWired struct {
	permRepo    *authpersistence.GormRolePermissionsRepository
	listUsers   *authqueries.ListUsersHandler
	currentUser *authqueries.GetCurrentUserHandler
	createUser  *authcommands.CreateUserHandler
	deleteUser  *authcommands.DeleteUserHandler
	routes      infrahttp.ModuleRoutes
}

func wireAuth(gdb *gormsqlite.DB, cfg Config) *authWired {
	userRepo    := authpersistence.NewGormUserRepository(gdb)
	sessionRepo := authpersistence.NewGormSessionRepository(gdb)
	permRepo    := authpersistence.NewGormRolePermissionsRepository(gdb)
	roleRepo    := authpersistence.NewGormRoleRepository(gdb)

	if err := sessionRepo.PruneExpired(context.Background()); err != nil {
		log.Printf("warn: prune sessions on startup: %v", err)
	}

	loginCmd             := authcommands.NewLoginHandler(userRepo, sessionRepo)
	logoutCmd            := authcommands.NewLogoutHandler(sessionRepo)
	createUserCmd        := authcommands.NewCreateUserHandler(userRepo, roleRepo)
	deleteUserCmd        := authcommands.NewDeleteUserHandler(userRepo, sessionRepo)
	changeSelfPasswordCmd := authcommands.NewChangeSelfPasswordHandler(userRepo)
	updateUserCmd        := authcommands.NewUpdateUserHandler(userRepo, roleRepo)
	createRoleCmd        := authcommands.NewCreateRoleHandler(roleRepo, permRepo)
	deleteRoleCmd        := authcommands.NewDeleteRoleHandler(roleRepo)
	createSessionCmd     := authcommands.NewCreateSessionHandler(sessionRepo)
	listUsersQuery       := authqueries.NewListUsersHandler(userRepo)
	getCurrentUserQuery  := authqueries.NewGetCurrentUserHandler(userRepo, sessionRepo)

	routes := authhttp.NewHandlers(
		loginCmd, logoutCmd, createUserCmd, deleteUserCmd,
		changeSelfPasswordCmd, updateUserCmd,
		listUsersQuery, getCurrentUserQuery,
	).
		WithPermRepo(permRepo).
		WithRoleHandlers(createRoleCmd, deleteRoleCmd, roleRepo).
		WithMaxAge(cfg.SessionLifetime()).
		WithSecureCookie(cfg.SecureCookie).
		WithTOTPUsers(userRepo).
		WithCreateSession(createSessionCmd)

	if cfg.AdminUser != "" && cfg.AdminPassword != "" {
		if err := seedAdmin(context.Background(), userRepo, cfg.AdminUser, cfg.AdminPassword); err != nil {
			log.Printf("warn: seed admin: %v", err)
		}
	}

	return &authWired{
		permRepo:    permRepo,
		listUsers:   listUsersQuery,
		currentUser: getCurrentUserQuery,
		createUser:  createUserCmd,
		deleteUser:  deleteUserCmd,
		routes:      routes,
	}
}
