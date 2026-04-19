package commands_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	customermigrations "github.com/atvirokodosprendimai/vvs/internal/modules/customer/migrations"
	"github.com/atvirokodosprendimai/vvs/internal/modules/ticket/adapters/persistence"
	"github.com/atvirokodosprendimai/vvs/internal/modules/ticket/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/ticket/domain"
	"github.com/atvirokodosprendimai/vvs/internal/modules/ticket/migrations"
	"github.com/atvirokodosprendimai/vvs/internal/testutil"
)

const testCustomerID = "cust-test-001"

// setupFull initialises a fresh test DB, runs migrations (customer first for FK
// satisfaction, then ticket), seeds a dummy customer row and returns all five
// command handlers together with the repository for read-back assertions.
func setupFull(t *testing.T) (
	domain.TicketRepository,
	*commands.OpenTicketHandler,
	*commands.UpdateTicketHandler,
	*commands.DeleteTicketHandler,
	*commands.AddCommentHandler,
	*commands.ChangeTicketStatusHandler,
) {
	t.Helper()

	db := testutil.NewTestDB(t)

	// Ticket rows reference customers(id) — run customer migrations first so the
	// FK target table exists, then run ticket migrations.
	testutil.RunMigrations(t, db, customermigrations.FS, "goose_customer")
	testutil.RunMigrations(t, db, migrations.FS, "goose_ticket")

	// Seed a minimal customer row so FK checks pass when we insert tickets.
	err := db.W.Exec(
		`INSERT INTO customers (id, code, company_name, status) VALUES (?, ?, ?, ?)`,
		testCustomerID, "CLI-99999", "Test Customer", "active",
	).Error
	require.NoError(t, err)

	pub, _ := testutil.NewTestNATS(t)
	repo := persistence.NewGormTicketRepository(db)

	return repo,
		commands.NewOpenTicketHandler(repo, pub),
		commands.NewUpdateTicketHandler(repo, pub),
		commands.NewDeleteTicketHandler(repo, pub),
		commands.NewAddCommentHandler(repo, pub),
		commands.NewChangeTicketStatusHandler(repo, pub)
}

// ---- OpenTicket ----

func TestOpenTicketHandler_HappyPath(t *testing.T) {
	repo, open, _, _, _, _ := setupFull(t)

	cmd := commands.OpenTicketCommand{
		CustomerID: testCustomerID,
		Subject:    "Internet is down",
		Body:       "No connectivity since this morning",
		Priority:   domain.PriorityHigh,
	}

	tk, err := open.Handle(context.Background(), cmd)
	require.NoError(t, err)
	require.NotNil(t, tk)

	assert.NotEmpty(t, tk.ID)
	assert.Equal(t, testCustomerID, tk.CustomerID)
	assert.Equal(t, "Internet is down", tk.Subject)
	assert.Equal(t, "No connectivity since this morning", tk.Body)
	assert.Equal(t, domain.PriorityHigh, tk.Priority)
	assert.Equal(t, domain.StatusOpen, tk.Status)

	// Verify persisted — read back from DB.
	found, err := repo.FindByID(context.Background(), tk.ID)
	require.NoError(t, err)
	assert.Equal(t, tk.Subject, found.Subject)
	assert.Equal(t, domain.StatusOpen, found.Status)
	assert.Equal(t, domain.PriorityHigh, found.Priority)
}

func TestOpenTicketHandler_DefaultPriority(t *testing.T) {
	_, open, _, _, _, _ := setupFull(t)

	tk, err := open.Handle(context.Background(), commands.OpenTicketCommand{
		CustomerID: testCustomerID,
		Subject:    "Missing router",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.PriorityNormal, tk.Priority)
}

func TestOpenTicketHandler_EmptySubject(t *testing.T) {
	_, open, _, _, _, _ := setupFull(t)

	tk, err := open.Handle(context.Background(), commands.OpenTicketCommand{
		CustomerID: testCustomerID,
		Subject:    "",
	})
	assert.ErrorIs(t, err, domain.ErrSubjectRequired)
	assert.Nil(t, tk)
}

func TestOpenTicketHandler_EmptyCustomerID(t *testing.T) {
	_, open, _, _, _, _ := setupFull(t)

	tk, err := open.Handle(context.Background(), commands.OpenTicketCommand{
		CustomerID: "",
		Subject:    "Some subject",
	})
	require.Error(t, err)
	assert.Nil(t, tk)
}

// ---- UpdateTicket ----

func TestUpdateTicketHandler_HappyPath(t *testing.T) {
	repo, open, update, _, _, _ := setupFull(t)

	created, err := open.Handle(context.Background(), commands.OpenTicketCommand{
		CustomerID: testCustomerID,
		Subject:    "Original subject",
		Body:       "Original body",
		Priority:   domain.PriorityNormal,
	})
	require.NoError(t, err)

	err = update.Handle(context.Background(), commands.UpdateTicketCommand{
		ID:       created.ID,
		Subject:  "Updated subject",
		Body:     "Updated body",
		Priority: domain.PriorityUrgent,
	})
	require.NoError(t, err)

	// Verify persisted.
	found, err := repo.FindByID(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated subject", found.Subject)
	assert.Equal(t, "Updated body", found.Body)
	assert.Equal(t, domain.PriorityUrgent, found.Priority)
}

func TestUpdateTicketHandler_EmptySubject(t *testing.T) {
	_, open, update, _, _, _ := setupFull(t)

	created, err := open.Handle(context.Background(), commands.OpenTicketCommand{
		CustomerID: testCustomerID,
		Subject:    "Valid subject",
	})
	require.NoError(t, err)

	err = update.Handle(context.Background(), commands.UpdateTicketCommand{
		ID:      created.ID,
		Subject: "",
	})
	assert.ErrorIs(t, err, domain.ErrSubjectRequired)
}

func TestUpdateTicketHandler_NotFound(t *testing.T) {
	_, _, update, _, _, _ := setupFull(t)

	err := update.Handle(context.Background(), commands.UpdateTicketCommand{
		ID:      "nonexistent-id",
		Subject: "Something",
	})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ---- DeleteTicket ----

func TestDeleteTicketHandler_HappyPath(t *testing.T) {
	repo, open, _, del, _, _ := setupFull(t)

	created, err := open.Handle(context.Background(), commands.OpenTicketCommand{
		CustomerID: testCustomerID,
		Subject:    "To be deleted",
	})
	require.NoError(t, err)

	err = del.Handle(context.Background(), commands.DeleteTicketCommand{ID: created.ID})
	require.NoError(t, err)

	// Verify gone — FindByID must return ErrNotFound.
	_, err = repo.FindByID(context.Background(), created.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestDeleteTicketHandler_NotFound(t *testing.T) {
	_, _, _, del, _, _ := setupFull(t)

	err := del.Handle(context.Background(), commands.DeleteTicketCommand{ID: "nonexistent-id"})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ---- AddComment ----

func TestAddCommentHandler_HappyPath(t *testing.T) {
	repo, open, _, _, addComment, _ := setupFull(t)

	tk, err := open.Handle(context.Background(), commands.OpenTicketCommand{
		CustomerID: testCustomerID,
		Subject:    "Need comment",
	})
	require.NoError(t, err)

	comment, err := addComment.Handle(context.Background(), commands.AddCommentCommand{
		TicketID: tk.ID,
		Body:     "Technician on the way",
		AuthorID: "user-42",
	})
	require.NoError(t, err)
	require.NotNil(t, comment)

	assert.NotEmpty(t, comment.ID)
	assert.Equal(t, tk.ID, comment.TicketID)
	assert.Equal(t, "Technician on the way", comment.Body)
	assert.Equal(t, "user-42", comment.AuthorID)

	// Verify persisted — list comments from DB.
	comments, err := repo.ListComments(context.Background(), tk.ID)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	assert.Equal(t, comment.ID, comments[0].ID)
	assert.Equal(t, "Technician on the way", comments[0].Body)
}

func TestAddCommentHandler_EmptyBody(t *testing.T) {
	_, open, _, _, addComment, _ := setupFull(t)

	tk, err := open.Handle(context.Background(), commands.OpenTicketCommand{
		CustomerID: testCustomerID,
		Subject:    "Ticket for empty comment test",
	})
	require.NoError(t, err)

	comment, err := addComment.Handle(context.Background(), commands.AddCommentCommand{
		TicketID: tk.ID,
		Body:     "",
	})
	require.Error(t, err)
	assert.Nil(t, comment)
}

func TestAddCommentHandler_TicketNotFound(t *testing.T) {
	_, _, _, _, addComment, _ := setupFull(t)

	comment, err := addComment.Handle(context.Background(), commands.AddCommentCommand{
		TicketID: "nonexistent-id",
		Body:     "Hello",
	})
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.Nil(t, comment)
}

// ---- ChangeTicketStatus ----

func TestChangeTicketStatusHandler_FullLifecycle(t *testing.T) {
	repo, open, _, _, _, changeStatus := setupFull(t)

	created, err := open.Handle(context.Background(), commands.OpenTicketCommand{
		CustomerID: testCustomerID,
		Subject:    "Lifecycle ticket",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusOpen, created.Status)

	// open → in_progress
	err = changeStatus.Handle(context.Background(), commands.ChangeTicketStatusCommand{
		ID:     created.ID,
		Action: "start",
	})
	require.NoError(t, err)

	found, err := repo.FindByID(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusInProgress, found.Status)

	// in_progress → resolved
	err = changeStatus.Handle(context.Background(), commands.ChangeTicketStatusCommand{
		ID:     created.ID,
		Action: "resolve",
	})
	require.NoError(t, err)

	found, err = repo.FindByID(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusResolved, found.Status)
}

func TestChangeTicketStatusHandler_CloseReopen(t *testing.T) {
	repo, open, _, _, _, changeStatus := setupFull(t)

	created, err := open.Handle(context.Background(), commands.OpenTicketCommand{
		CustomerID: testCustomerID,
		Subject:    "Close then reopen",
	})
	require.NoError(t, err)

	// open → closed
	err = changeStatus.Handle(context.Background(), commands.ChangeTicketStatusCommand{
		ID:     created.ID,
		Action: "close",
	})
	require.NoError(t, err)

	found, err := repo.FindByID(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusClosed, found.Status)

	// closed → open
	err = changeStatus.Handle(context.Background(), commands.ChangeTicketStatusCommand{
		ID:     created.ID,
		Action: "reopen",
	})
	require.NoError(t, err)

	found, err = repo.FindByID(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusOpen, found.Status)
}

func TestChangeTicketStatusHandler_InvalidAction(t *testing.T) {
	_, open, _, _, _, changeStatus := setupFull(t)

	created, err := open.Handle(context.Background(), commands.OpenTicketCommand{
		CustomerID: testCustomerID,
		Subject:    "Action test",
	})
	require.NoError(t, err)

	err = changeStatus.Handle(context.Background(), commands.ChangeTicketStatusCommand{
		ID:     created.ID,
		Action: "fly",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown action")
}

func TestChangeTicketStatusHandler_InvalidTransition(t *testing.T) {
	_, open, _, _, _, changeStatus := setupFull(t)

	created, err := open.Handle(context.Background(), commands.OpenTicketCommand{
		CustomerID: testCustomerID,
		Subject:    "Transition test",
	})
	require.NoError(t, err)

	// Cannot resolve an open ticket — must be in_progress first.
	err = changeStatus.Handle(context.Background(), commands.ChangeTicketStatusCommand{
		ID:     created.ID,
		Action: "resolve",
	})
	assert.ErrorIs(t, err, domain.ErrInvalidTransition)
}
