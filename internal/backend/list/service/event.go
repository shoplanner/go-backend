package service

import (
	"context"
	"slices"

	"go-backend/internal/backend/list"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
)

type EventProvider struct {
	ch     chan list.Event
	userID id.ID[user.User]
}

func (s *Service) ListenEvents(
	ctx context.Context,
	userID id.ID[user.User],
	listID id.ID[list.ProductList],
) (
	*EventProvider,
	error,
) {
	currentList, err := s.GetByID(ctx, listID, userID)
	if err != nil {
		return nil, err
	}

	provider := &EventProvider{
		ch:     make(chan list.Event),
		userID: userID,
	}

	s.channelsLock.Lock()

	providers := s.channels[listID.String()]
	if idx := slices.IndexFunc(providers, func(p *EventProvider) bool { return p.userID == userID }); idx != -1 {
		provider = providers[idx]
	} else {
		s.channels[listID.String()] = append(s.channels[listID.String()], provider)
	}

	s.channelsLock.Unlock()

	provider.ch <- list.Event{
		ListID: listID,
		Member: nil, // no real change here
		Change: currentList,
	}

	return provider, nil
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
