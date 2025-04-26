package service

import (
	"context"
	"fmt"
	"sync"

	"go-backend/internal/backend/list"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

type providerID struct {
	userID id.ID[user.User]
	listID id.ID[list.ProductList]
}

type eventProvider struct {
	ch    chan list.Event
	id    providerID
	close func()
}

func newEventProvider(id providerID) *eventProvider {
	ch := make(chan list.Event, 1)
	return &eventProvider{
		ch: ch,
		id: id,
		close: sync.OnceFunc(func() {
			close(ch)
		}),
	}
}

func (s *Service) ListenEvents(
	ctx context.Context,
	userID id.ID[user.User],
	listID id.ID[list.ProductList],
) (
	<-chan list.Event,
	error,
) {
	currentList, err := s.GetByID(ctx, listID, userID)
	if err != nil {
		return nil, err
	}

	s.channelsLock.Lock()

	id := providerID{userID: userID, listID: listID}

	provider, found := s.channels[id]
	if !found {
		provider = newEventProvider(id)
		s.channels[id] = provider
	}

	provider.ch <- list.Event{
		ListID: listID,
		Type:   list.EventTypeFull,
		Member: nil, // no real change here
		Change: currentList,
	}

	s.channelsLock.Unlock()

	return provider.ch, nil
}

// StopListenEvents close sending events to provided user from provided listID
func (s *Service) StopListenEvents(
	userID id.ID[user.User],
	listID id.ID[list.ProductList],
) error {
	s.channelsLock.Lock()
	defer s.channelsLock.Unlock()

	id := providerID{
		userID: userID,
		listID: listID,
	}

	_, found := s.channels[id]
	if found {
		s.channels[id].close()
		delete(s.channels, id)
		return nil
	}

	return fmt.Errorf("%w: listener %d", myerr.ErrNotFound, id)
}

func (s *Service) sendUpdateEvent(
	listID id.ID[list.ProductList],
	member list.Member,
	eventType list.EventType,
	change any,
) {
	s.channelsLock.RLock()
	defer s.channelsLock.RUnlock()

	event := list.Event{
		Type:   eventType,
		ListID: listID,
		Member: &member,
		Change: change,
	}

	s.log.Info().Any("event", event).Msg("sending event")

	for id, provider := range s.channels {
		if id.listID != listID || member.UserID == id.userID {
			continue
		}

		provider.ch <- event
	}
}
