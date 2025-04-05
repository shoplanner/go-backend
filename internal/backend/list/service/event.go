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
	ch := make(chan list.Event)
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

	if found {
		provider = provider
	} else {
		provider = newEventProvider(id)
		s.channels[id] = provider
	}

	s.channelsLock.Unlock()

	provider.ch <- list.Event{
		ListID: listID,
		Member: nil, // no real change here
		Change: currentList,
	}

	return provider.ch, nil
}

func (s *Service) StopListenEvents(
	ctx context.Context,
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
		delete(s.channels, id)
		return nil
	} else {
		return fmt.Errorf("%w: listener %d", myerr.ErrNotFound, id)
	}
}

func (s *Service) sendUpdateEvent(listID id.ID[list.ProductList], member list.Member, change any) {
	s.channelsLock.RLock()
	defer s.channelsLock.RUnlock()

	event := list.Event{
		ListID: listID,
		Member: &member,
		Change: change,
	}

	for _, provider := range s.channels[listID.String()] {
		provider.ch <- event
	}
}
