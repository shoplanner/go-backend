package handler

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type listService interface {
	SubscribeUpdates()
}

type WebSocket struct {
	log    *slog.Logger
	config websocket.Upgrader
	list   listService
}

func NewWebSocket(r *gin.Engine, listService listService) {
	w := WebSocket{
		log:  slog.Default().With("component", "product list websocket"),
		list: listService,
	}

	listService.SubscribeUpdates()

	r.GET("/list/id/:id/ws", w.Listen)
}

func (s *WebSocket) Listen(ctx *gin.Context) {
	conn, err := s.config.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		s.log.Error(fmt.Sprintf("can't open websocket: %s", err.Error()))
		ctx.String(http.StatusInternalServerError, "can't open websocket")
		return
	}

	for {
		if err := conn.WriteJSON("event"); err != nil {
			s.log.Error(err.Error())
			return
		}
	}
}
