package commands_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/contact/adapters/persistence"
	"github.com/atvirokodosprendimai/vvs/internal/modules/contact/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/contact/domain"
	contactmigrations "github.com/atvirokodosprendimai/vvs/internal/modules/contact/migrations"
	customermigrations "github.com/atvirokodosprendimai/vvs/internal/modules/customer/migrations"
	"github.com/atvirokodosprendimai/vvs/internal/testutil"
)

// insertCustomer inserts a bare-minimum customer row to satisfy the FK constraint.
func insertCustomer(t *testing.T, db *gormsqlite.DB, id string) {
	t.Helper()
	err := db.W.Exec(
		"INSERT INTO customers (id, code, company_name, status) VALUES (?, ?, ?, ?)",
		id, "CLI-00001", "Test Corp", "active",
	).Error
	require.NoError(t, err)
}

func setupContactDB(t *testing.T) (*gormsqlite.DB, string) {
	t.Helper()
	db := testutil.NewTestDB(t)
	// Customer table must exist because contacts.customer_id FK references it.
	testutil.RunMigrations(t, db, customermigrations.FS, "goose_customer")
	testutil.RunMigrations(t, db, contactmigrations.FS, "goose_contact")
	customerID := "cust-integration-001"
	insertCustomer(t, db, customerID)
	return db, customerID
}

func TestAddContactHandler_HappyPath(t *testing.T) {
	db, customerID := setupContactDB(t)
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormContactRepository(db)
	handler := commands.NewAddContactHandler(repo, pub)

	cmd := commands.AddContactCommand{
		CustomerID: customerID,
		FirstName:  "Jane",
		LastName:   "Doe",
		Email:      "jane@example.com",
		Phone:      "+37061234567",
		Role:       "billing",
	}

	contact, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	require.NotNil(t, contact)

	assert.Equal(t, customerID, contact.CustomerID)
	assert.Equal(t, "Jane", contact.FirstName)
	assert.Equal(t, "Doe", contact.LastName)
	assert.Equal(t, "jane@example.com", contact.Email)
	assert.Equal(t, "+37061234567", contact.Phone)
	assert.Equal(t, "billing", contact.Role)
	assert.NotEmpty(t, contact.ID)

	// Verify persisted — read back from DB.
	found, err := repo.FindByID(context.Background(), contact.ID)
	require.NoError(t, err)
	assert.Equal(t, contact.FirstName, found.FirstName)
	assert.Equal(t, contact.Email, found.Email)
}

func TestAddContactHandler_EmptyFirstName(t *testing.T) {
	db, customerID := setupContactDB(t)
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormContactRepository(db)
	handler := commands.NewAddContactHandler(repo, pub)

	contact, err := handler.Handle(context.Background(), commands.AddContactCommand{
		CustomerID: customerID,
		FirstName:  "",
		LastName:   "Doe",
	})
	assert.ErrorIs(t, err, domain.ErrFirstNameRequired)
	assert.Nil(t, contact)
}

func TestUpdateContactHandler_HappyPath(t *testing.T) {
	db, customerID := setupContactDB(t)
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormContactRepository(db)
	addHandler := commands.NewAddContactHandler(repo, pub)
	updateHandler := commands.NewUpdateContactHandler(repo, pub)

	// Create first.
	contact, err := addHandler.Handle(context.Background(), commands.AddContactCommand{
		CustomerID: customerID,
		FirstName:  "Alice",
		LastName:   "Smith",
		Email:      "alice@example.com",
		Phone:      "+37069999999",
		Role:       "tech",
	})
	require.NoError(t, err)

	// Update.
	err = updateHandler.Handle(context.Background(), commands.UpdateContactCommand{
		ID:        contact.ID,
		FirstName: "Alice",
		LastName:  "Jones",
		Email:     "alice.jones@example.com",
		Phone:     "+37061111111",
		Role:      "manager",
		Notes:     "Updated contact",
	})
	require.NoError(t, err)

	// Verify new values persisted.
	found, err := repo.FindByID(context.Background(), contact.ID)
	require.NoError(t, err)
	assert.Equal(t, "Jones", found.LastName)
	assert.Equal(t, "alice.jones@example.com", found.Email)
	assert.Equal(t, "manager", found.Role)
	assert.Equal(t, "Updated contact", found.Notes)
}

func TestUpdateContactHandler_NotFound(t *testing.T) {
	db, _ := setupContactDB(t)
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormContactRepository(db)
	handler := commands.NewUpdateContactHandler(repo, pub)

	err := handler.Handle(context.Background(), commands.UpdateContactCommand{
		ID:        "nonexistent-id",
		FirstName: "X",
		LastName:  "Y",
	})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestDeleteContactHandler_NotFound(t *testing.T) {
	db, _ := setupContactDB(t)
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormContactRepository(db)
	handler := commands.NewDeleteContactHandler(repo, pub)

	err := handler.Handle(context.Background(), commands.DeleteContactCommand{ID: "nonexistent-id"})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestDeleteContactHandler_HappyPath(t *testing.T) {
	db, customerID := setupContactDB(t)
	pub, _ := testutil.NewTestNATS(t)

	repo := persistence.NewGormContactRepository(db)
	addHandler := commands.NewAddContactHandler(repo, pub)
	deleteHandler := commands.NewDeleteContactHandler(repo, pub)

	// Create first.
	contact, err := addHandler.Handle(context.Background(), commands.AddContactCommand{
		CustomerID: customerID,
		FirstName:  "Bob",
	})
	require.NoError(t, err)

	// Delete.
	err = deleteHandler.Handle(context.Background(), commands.DeleteContactCommand{ID: contact.ID})
	require.NoError(t, err)

	// Verify gone — FindByID must return ErrNotFound.
	_, err = repo.FindByID(context.Background(), contact.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
