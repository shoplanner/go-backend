package functest_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go-backend/internal/backend/list"
	"go-backend/pkg/myerr"
)

const eventTimeout = 2 * time.Second

func recvEvent(t *testing.T, ch <-chan list.Event) list.Event {
	t.Helper()

	select {
	case event, open := <-ch:
		require.True(t, open, "event channel closed unexpectedly")

		return event
	case <-time.After(eventTimeout):
		t.Fatal("timed out waiting for an event")

		return list.Event{}
	}
}

func requireNoEvent(t *testing.T, ch <-chan list.Event) {
	t.Helper()

	select {
	case event := <-ch:
		t.Fatalf("unexpected event: %+v", event)
	case <-time.After(100 * time.Millisecond):
	}
}

// Subscribing replays the current state of the list as a full-update event, so a fresh
// websocket client does not have to fetch the list separately.
func TestListenEventsRepliesWithFullState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	listModel := a.newList(t, owner.ID, "groceries")
	milk := a.newProduct(t, "milk", "dairy")
	a.addProducts(t, listModel.ID, owner.ID, milk)

	events, err := a.lists.ListenEvents(ctx, owner.ID, listModel.ID)
	require.NoError(t, err)

	event := recvEvent(t, events)
	require.Equal(t, listModel.ID, event.ListID)
	require.Equal(t, list.EventTypeFull, event.Change.Type)
	require.Nil(t, event.Member, "the initial replay is not attributed to anyone")

	full, casted := event.Change.Data.(list.FullUpdateChange)
	require.True(t, casted)
	require.Len(t, full.States, 1)
	require.Equal(t, milk.ID, full.States[0].Product.ID)

	require.NoError(t, a.lists.StopListenEvents(owner.ID, listModel.ID))
}

func TestListenEventsRejectsNonMember(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	stranger := a.newUser(t, "stranger")
	listModel := a.newList(t, owner.ID, "groceries")

	_, err := a.lists.ListenEvents(ctx, stranger.ID, listModel.ID)
	require.ErrorIs(t, err, myerr.ErrForbidden)
}

// A mutation fans out to every subscriber of the list except the member who caused it.
func TestListEventsFanOutSkipsTheAuthor(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	editor := a.newUser(t, "editor")
	listModel := a.newList(t, owner.ID, "groceries")
	a.addMember(t, listModel.ID, owner.ID, editor.ID, list.MemberTypeEditor)

	ownerEvents, err := a.lists.ListenEvents(ctx, owner.ID, listModel.ID)
	require.NoError(t, err)
	require.Equal(t, list.EventTypeFull, recvEvent(t, ownerEvents).Change.Type)

	editorEvents, err := a.lists.ListenEvents(ctx, editor.ID, listModel.ID)
	require.NoError(t, err)
	require.Equal(t, list.EventTypeFull, recvEvent(t, editorEvents).Change.Type)

	_, err = a.lists.Update(ctx, listModel.ID, owner.ID, list.ListOptions{
		Status: list.ExecStatusProcessing,
		Title:  "renamed",
	})
	require.NoError(t, err)

	event := recvEvent(t, editorEvents)
	require.Equal(t, list.EventTypeOptsUpdated, event.Change.Type)
	require.NotNil(t, event.Member)
	require.Equal(t, owner.ID, event.Member.UserID)

	change, casted := event.Change.Data.(list.ListOptionsChange)
	require.True(t, casted)
	require.Equal(t, "renamed", change.NewOptions.Title)

	requireNoEvent(t, ownerEvents)

	require.NoError(t, a.lists.StopListenEvents(owner.ID, listModel.ID))
	require.NoError(t, a.lists.StopListenEvents(editor.ID, listModel.ID))
}

// Subscribers of one list must not see another list's traffic.
func TestListEventsAreScopedToTheList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	watcher := a.newUser(t, "watcher")

	watched := a.newList(t, owner.ID, "watched")
	other := a.newList(t, owner.ID, "other")
	a.addMember(t, watched.ID, owner.ID, watcher.ID, list.MemberTypeViewer)

	events, err := a.lists.ListenEvents(ctx, watcher.ID, watched.ID)
	require.NoError(t, err)
	require.Equal(t, list.EventTypeFull, recvEvent(t, events).Change.Type)

	_, err = a.lists.Update(ctx, other.ID, owner.ID, list.ListOptions{
		Status: list.ExecStatusArchived,
		Title:  "unrelated",
	})
	require.NoError(t, err)

	requireNoEvent(t, events)

	require.NoError(t, a.lists.StopListenEvents(watcher.ID, watched.ID))
}

func TestStopListenEvents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	listModel := a.newList(t, owner.ID, "groceries")

	events, err := a.lists.ListenEvents(ctx, owner.ID, listModel.ID)
	require.NoError(t, err)
	require.Equal(t, list.EventTypeFull, recvEvent(t, events).Change.Type)

	require.NoError(t, a.lists.StopListenEvents(owner.ID, listModel.ID))

	_, open := <-events
	require.False(t, open, "the channel must be closed")

	require.ErrorIs(t, a.lists.StopListenEvents(owner.ID, listModel.ID), myerr.ErrNotFound)
}

// Every mutation carries its own change type. This is the contract the websocket clients
// decode against, so it is pinned per operation.
func TestListEventTypesPerMutation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	watcher := a.newUser(t, "watcher")
	listModel := a.newList(t, owner.ID, "groceries")
	a.addMember(t, listModel.ID, owner.ID, watcher.ID, list.MemberTypeViewer)

	milk := a.newProduct(t, "milk", "dairy")
	bread := a.newProduct(t, "bread", "bakery")

	events, err := a.lists.ListenEvents(ctx, watcher.ID, listModel.ID)
	require.NoError(t, err)
	require.Equal(t, list.EventTypeFull, recvEvent(t, events).Change.Type)

	a.addProducts(t, listModel.ID, owner.ID, milk)
	require.Equal(t, list.EventTypeProductsAdded, recvEvent(t, events).Change.Type)

	_, err = a.lists.UpdateProductState(ctx, listModel.ID, owner.ID, milk.ID, zeroStateOptions())
	require.NoError(t, err)
	require.Equal(t, list.EventTypeStateUpdated, recvEvent(t, events).Change.Type)

	a.addProducts(t, listModel.ID, owner.ID, bread)
	require.Equal(t, list.EventTypeProductsAdded, recvEvent(t, events).Change.Type)

	require.NoError(t, a.lists.ReoderStates(ginCtx(), owner.ID, listModel.ID, productIDsOf(bread, milk)))
	require.Equal(t, list.EventTypeStatesReordered, recvEvent(t, events).Change.Type)

	_, err = a.lists.DeleteProducts(ctx, listModel.ID, owner.ID, productIDsOf(bread))
	require.NoError(t, err)
	require.Equal(t, list.EventTypeProductsRemoved, recvEvent(t, events).Change.Type)

	extra := a.newUser(t, "extra")
	a.addMember(t, listModel.ID, owner.ID, extra.ID, list.MemberTypeViewer)
	require.Equal(t, list.EventTypeMembersAdded, recvEvent(t, events).Change.Type)

	_, err = a.lists.DeleteMembers(ctx, listModel.ID, owner.ID, memberIDsOf(extra.ID))
	require.NoError(t, err)
	require.Equal(t, list.EventTypeMembersRemoved, recvEvent(t, events).Change.Type)

	require.NoError(t, a.lists.DeleteList(ctx, owner.ID, listModel.ID))
	require.Equal(t, list.EventTypeDeleted, recvEvent(t, events).Change.Type)

	require.NoError(t, a.lists.StopListenEvents(watcher.ID, listModel.ID))
}
