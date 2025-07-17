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
	"go-backend/pkg/myerr"
)

type listService interface {
	ListenEvents(context.Context, id.ID[user.User], id.ID[list.ProductList]) (<-chan list.Event, error)
	StopListenEvents(id.ID[user.User], id.ID[list.ProductList]) error
}

type WebSocket struct {
	rerr.BaseHandler

	log    zerolog.Logger
	config websocket.Upgrader
	list   listService
}

func RegisterWebSocket(r *gin.RouterGroup, listService listService, log zerolog.Logger) {
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

	r.GET("/lists/:id/ws", w.Listen)
}

func (s *WebSocket) Listen(ctx *gin.Context) {
	closeChan := make(chan struct{})
	listID, ok := rerr.PathID[list.ProductList](ctx)
	if !ok {
		s.HandleError(ctx, fmt.Errorf("%w: no id provided", myerr.ErrInvalidArgument))
		return
	}
	userID := api.GetUserID(ctx)

	conn, err := s.config.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		s.HandleError(ctx, fmt.Errorf("opening websocket: %w", err))
		return
	}
	defer conn.Close()

	eventChannel, err := s.list.ListenEvents(ctx, userID, listID)
	if err != nil {
		s.HandleError(ctx, fmt.Errorf("getting event channel failed: %w", err))
		return
	}

	conn.SetCloseHandler(func(code int, text string) error {
		s.log.Info().Str("text", text).Int("code", code).Msg("closing updater")
		close(closeChan)
		closeErr := s.list.StopListenEvents(userID, listID)
		if closeErr != nil {
			s.log.Err(closeErr).Msg("closing event channel failed")
		}
		return nil
	})

	s.log.Info().Ctx(ctx).Stringer("user_id", userID).Stringer("list_id", listID).Msg("start listening")

	for {
		select {
		case event, open := <-eventChannel:
			s.log.Debug().Ctx(ctx).Any("event", event).Stringer("user_id", userID).Stringer("list_id", listID).Msg("got event")
			if !open {
				return
			}

			err = conn.WriteJSON(event)
			if err != nil {
				s.HandleError(ctx, fmt.Errorf("writing JSON message: %w", err))
				err = s.list.StopListenEvents(userID, listID)
				if err != nil {
					s.log.Err(err).Msg("closing event channel")
				}
				return
			}
		case <-closeChan:
			return
		}
	}
}
