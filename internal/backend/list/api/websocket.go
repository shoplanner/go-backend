package api

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"

	"go-backend/internal/backend/auth/api"
	"go-backend/internal/backend/list"
	"go-backend/internal/backend/user"
	"go-backend/pkg/api/rest/rerr"
	"go-backend/pkg/id"
)

type eventProvider interface {
	GetEventChan() <-chan list.Event
	Close() error
}

type listService[T eventProvider] interface {
	ListenEvents(ctx context.Context, userID id.ID[user.User], listID id.ID[list.ProductList]) (T, error)
}

type WebSocket struct {
	rerr.BaseHandler

	log    zerolog.Logger
	config websocket.Upgrader
	list   listService[eventProvider]
}

func RegisterWebSocket(r *gin.RouterGroup, listService listService[eventProvider], log zerolog.Logger) {
	log = log.With().Str("component", "product list websocket").Logger()
	w := WebSocket{BaseHandler: rerr.NewBaseHandler(log), log: log, list: listService, config: websocket.Upgrader{
		HandshakeTimeout:  0,
		ReadBufferSize:    0,
		WriteBufferSize:   0,
		WriteBufferPool:   nil,
		Subprotocols:      nil,
		Error:             nil,
		CheckOrigin:       nil,
		EnableCompression: false,
	}}

	r.GET("/list/id/:id/ws", w.Listen)
}

func (s *WebSocket) Listen(ctx *gin.Context) {
	closeChan := make(chan struct{})
	listID, ok := rerr.PathID[list.ProductList](ctx)
	if !ok {
		return
	}

	conn, err := s.config.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		s.HandleError(ctx, fmt.Errorf("opening websocket: %w", err))
		return
	}
	defer conn.Close()

	provider, err := s.list.ListenEvents(ctx, api.GetUserID(ctx), listID)
	if err != nil {
		s.HandleError(ctx, fmt.Errorf("getting event channel failed: %w", err))
		return
	}
	eventChannel := provider.GetEventChan()

	conn.SetCloseHandler(func(code int, text string) error {
		s.log.Info().Str("text", text).Int("code", code).Msg("closing updater")
		close(closeChan)
		closeErr := provider.Close()
		if closeErr != nil {
			s.log.Err(closeErr).Msg("closing event channel failed")
		}
		return nil
	})

	for {
		select {
		case event, open := <-eventChannel:
			if !open {
				return
			}

			err = conn.WriteJSON(event)
			if err != nil {
				s.HandleError(ctx, fmt.Errorf("writing JSON message: %w", err))
				return
			}
		case <-closeChan:
			return
		}
	}
}
