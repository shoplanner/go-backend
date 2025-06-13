package rerr_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"go-backend/pkg/api/rest/rerr"
)

func TestQueryID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("valid query id", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		idStr := uuid.NewString()
		ctx.Request = httptest.NewRequest(http.MethodGet, "/?id="+idStr, nil)

		id, ok := rerr.QueryID[struct{}](ctx, "id")
		require.True(t, ok)
		require.Equal(t, idStr, id.String())
	})

	t.Run("invalid query id", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/?id=invalid", nil)

		id, ok := rerr.QueryID[struct{}](ctx, "id")
		require.False(t, ok)
		require.Equal(t, uuid.Nil, id.UUID)
	})
}

func TestPathID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("valid path id", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		idStr := uuid.NewString()
		ctx.Params = gin.Params{gin.Param{Key: "id", Value: idStr}}

		id, ok := rerr.PathID[struct{}](ctx)
		require.True(t, ok)
		require.Equal(t, idStr, id.String())
	})

	t.Run("invalid path id", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Params = gin.Params{gin.Param{Key: "id", Value: "invalid"}}

		id, ok := rerr.PathID[struct{}](ctx)
		require.False(t, ok)
		require.Equal(t, uuid.Nil, id.UUID)
	})
}
