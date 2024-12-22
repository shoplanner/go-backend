package repo

import (
	"context"
	"fmt"
	"sync"

	"go-backend/internal/backend/auth"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

type TokenRepo[T any] struct {
	lock sync.RWMutex
	m    map[auth.TokenID[T]]auth.TokenState
}

func NewTokenRepo[T any]() *TokenRepo[T] {
	return &TokenRepo[T]{
		lock: sync.RWMutex{},
		m:    make(map[auth.TokenID[T]]auth.TokenState),
	}
}

func (r *TokenRepo[T]) Set(_ context.Context, tokenID auth.TokenID[T], state auth.TokenState) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.m[tokenID] = state
	return nil
}

func (r *TokenRepo[T]) GetByID(_ context.Context, targetID id.ID[T]) (auth.TokenID[T], auth.TokenState, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	for tokenID, state := range r.m {
		if tokenID.ID == targetID {
			return tokenID, state, nil
		}
	}

	return auth.TokenID[T]{}, auth.TokenState{}, fmt.Errorf("%w: token with ID %s", myerr.ErrNotFound, targetID)
}

func (r *TokenRepo[T]) DeleteByID(_ context.Context, targetID id.ID[T]) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	for tokenID := range r.m {
		if tokenID.ID == targetID {
			delete(r.m, tokenID)
			break
		}
	}

	return nil
}

func (r *TokenRepo[T]) RevokeByDeviceID(_ context.Context, userID id.ID[user.User], deviceID auth.DeviceID) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	for tokenID, state := range r.m {
		if tokenID.DeviceID != deviceID || tokenID.UserID != userID {
			continue
		}

		state.Status = auth.TokenStatusRevoked
		r.m[tokenID] = state
	}

	return nil
}

func (r *TokenRepo[T]) RevokeByUserID(_ context.Context, userID id.ID[user.User]) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	for tokenID, state := range r.m {
		if tokenID.UserID != userID {
			continue
		}

		state.Status = auth.TokenStatusRevoked
		r.m[tokenID] = state
	}

	return nil
}


