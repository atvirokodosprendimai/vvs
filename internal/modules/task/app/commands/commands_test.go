package commands_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vvs/isp/internal/modules/task/adapters/persistence"
	"github.com/vvs/isp/internal/modules/task/app/commands"
	"github.com/vvs/isp/internal/modules/task/domain"
	"github.com/vvs/isp/internal/modules/task/migrations"
	"github.com/vvs/isp/internal/testutil"
)

func setup(t *testing.T) (domain.TaskRepository, commands.CreateTaskCommand) {
	t.Helper()
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_task")
	pub, _ := testutil.NewTestNATS(t)
	repo := persistence.NewGormTaskRepository(db)
	// Return repo and a reusable base command; tests can override fields.
	_ = pub
	return repo, commands.CreateTaskCommand{
		CustomerID:  "cust-001",
		Title:       "Fix the router",
		Description: "Router keeps dropping packets",
		Priority:    domain.PriorityHigh,
	}
}

func setupFull(t *testing.T) (domain.TaskRepository, *commands.CreateTaskHandler, *commands.UpdateTaskHandler, *commands.DeleteTaskHandler, *commands.ChangeTaskStatusHandler) {
	t.Helper()
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_task")
	pub, _ := testutil.NewTestNATS(t)
	repo := persistence.NewGormTaskRepository(db)
	return repo,
		commands.NewCreateTaskHandler(repo, pub),
		commands.NewUpdateTaskHandler(repo, pub),
		commands.NewDeleteTaskHandler(repo, pub),
		commands.NewChangeTaskStatusHandler(repo, pub)
}

// ---- CreateTask ----

func TestCreateTaskHandler_HappyPath(t *testing.T) {
	repo, create, _, _, _ := setupFull(t)

	due := time.Now().UTC().Add(24 * time.Hour)
	cmd := commands.CreateTaskCommand{
		CustomerID:  "cust-001",
		Title:       "Lay fibre cable",
		Description: "Main street",
		Priority:    domain.PriorityHigh,
		DueDate:     &due,
		AssigneeID:  "tech-42",
	}

	task, err := create.Handle(context.Background(), cmd)
	require.NoError(t, err)
	require.NotNil(t, task)

	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "Lay fibre cable", task.Title)
	assert.Equal(t, "Main street", task.Description)
	assert.Equal(t, domain.PriorityHigh, task.Priority)
	assert.Equal(t, domain.StatusTodo, task.Status)
	assert.Equal(t, "cust-001", task.CustomerID)
	assert.Equal(t, "tech-42", task.AssigneeID)

	// Verify persisted — read back from DB.
	found, err := repo.FindByID(context.Background(), task.ID)
	require.NoError(t, err)
	assert.Equal(t, task.Title, found.Title)
	assert.Equal(t, task.Status, found.Status)
	assert.Equal(t, task.Priority, found.Priority)
}

func TestCreateTaskHandler_DefaultPriority(t *testing.T) {
	_, create, _, _, _ := setupFull(t)

	task, err := create.Handle(context.Background(), commands.CreateTaskCommand{
		Title: "No priority given",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.PriorityNormal, task.Priority)
}

func TestCreateTaskHandler_EmptyTitle(t *testing.T) {
	_, create, _, _, _ := setupFull(t)

	task, err := create.Handle(context.Background(), commands.CreateTaskCommand{
		CustomerID: "cust-001",
		Title:      "",
	})
	assert.ErrorIs(t, err, domain.ErrTitleRequired)
	assert.Nil(t, task)
}

// ---- UpdateTask ----

func TestUpdateTaskHandler_HappyPath(t *testing.T) {
	repo, create, update, _, _ := setupFull(t)

	created, err := create.Handle(context.Background(), commands.CreateTaskCommand{
		Title:    "Original title",
		Priority: domain.PriorityLow,
	})
	require.NoError(t, err)

	updated, err := update.Handle(context.Background(), commands.UpdateTaskCommand{
		ID:          created.ID,
		Title:       "Updated title",
		Description: "New description",
		Priority:    domain.PriorityHigh,
	})
	require.NoError(t, err)
	require.NotNil(t, updated)

	assert.Equal(t, "Updated title", updated.Title)
	assert.Equal(t, "New description", updated.Description)
	assert.Equal(t, domain.PriorityHigh, updated.Priority)

	// Verify persisted.
	found, err := repo.FindByID(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated title", found.Title)
	assert.Equal(t, domain.PriorityHigh, found.Priority)
}

func TestUpdateTaskHandler_EmptyTitle(t *testing.T) {
	_, create, update, _, _ := setupFull(t)

	created, err := create.Handle(context.Background(), commands.CreateTaskCommand{
		Title: "Initial",
	})
	require.NoError(t, err)

	_, err = update.Handle(context.Background(), commands.UpdateTaskCommand{
		ID:    created.ID,
		Title: "",
	})
	assert.ErrorIs(t, err, domain.ErrTitleRequired)
}

// ---- DeleteTask ----

func TestDeleteTaskHandler_HappyPath(t *testing.T) {
	repo, create, _, del, _ := setupFull(t)

	created, err := create.Handle(context.Background(), commands.CreateTaskCommand{
		Title: "To be deleted",
	})
	require.NoError(t, err)

	err = del.Handle(context.Background(), commands.DeleteTaskCommand{ID: created.ID})
	require.NoError(t, err)

	// Verify gone — FindByID must return ErrNotFound.
	_, err = repo.FindByID(context.Background(), created.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestDeleteTaskHandler_NotFound(t *testing.T) {
	_, _, _, del, _ := setupFull(t)

	err := del.Handle(context.Background(), commands.DeleteTaskCommand{ID: "nonexistent-id"})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ---- ChangeTaskStatus ----

func TestChangeTaskStatusHandler_FullLifecycle(t *testing.T) {
	repo, create, _, _, changeStatus := setupFull(t)

	created, err := create.Handle(context.Background(), commands.CreateTaskCommand{
		Title: "Lifecycle task",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusTodo, created.Status)

	// todo → in_progress
	started, err := changeStatus.Handle(context.Background(), commands.ChangeTaskStatusCommand{
		ID:     created.ID,
		Action: "start",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusInProgress, started.Status)

	// Verify persisted.
	found, err := repo.FindByID(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusInProgress, found.Status)

	// in_progress → done
	completed, err := changeStatus.Handle(context.Background(), commands.ChangeTaskStatusCommand{
		ID:     created.ID,
		Action: "complete",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusDone, completed.Status)

	// Verify persisted.
	found, err = repo.FindByID(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusDone, found.Status)
}

func TestChangeTaskStatusHandler_CancelReopen(t *testing.T) {
	_, create, _, _, changeStatus := setupFull(t)

	created, err := create.Handle(context.Background(), commands.CreateTaskCommand{
		Title: "Cancel then reopen",
	})
	require.NoError(t, err)

	// todo → cancelled
	cancelled, err := changeStatus.Handle(context.Background(), commands.ChangeTaskStatusCommand{
		ID:     created.ID,
		Action: "cancel",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCancelled, cancelled.Status)

	// cancelled → todo
	reopened, err := changeStatus.Handle(context.Background(), commands.ChangeTaskStatusCommand{
		ID:     created.ID,
		Action: "reopen",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusTodo, reopened.Status)
}

func TestChangeTaskStatusHandler_InvalidAction(t *testing.T) {
	_, create, _, _, changeStatus := setupFull(t)

	created, err := create.Handle(context.Background(), commands.CreateTaskCommand{
		Title: "Action test",
	})
	require.NoError(t, err)

	_, err = changeStatus.Handle(context.Background(), commands.ChangeTaskStatusCommand{
		ID:     created.ID,
		Action: "fly",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown action")
}

func TestChangeTaskStatusHandler_InvalidTransition(t *testing.T) {
	_, create, _, _, changeStatus := setupFull(t)

	created, err := create.Handle(context.Background(), commands.CreateTaskCommand{
		Title: "Transition test",
	})
	require.NoError(t, err)

	// Cannot reopen a todo task — it's not done or cancelled.
	_, err = changeStatus.Handle(context.Background(), commands.ChangeTaskStatusCommand{
		ID:     created.ID,
		Action: "reopen",
	})
	assert.ErrorIs(t, err, domain.ErrInvalidTransition)
}
