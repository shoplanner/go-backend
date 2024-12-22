package repo

import (
	"context"
	"fmt"

	"github.com/kr/pretty"
	"github.com/redis/go-redis/v9"

	"go-backend/internal/backend/auth"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
)

const (
	keyTokenID    = "tokenId"
	keyUserID     = "userId"
	keyDeviceID   = "deviceId"
	keyTokenState = "tokenState"
)

type RedisRepo[T any] struct {
	client    *redis.Client
	indexName string
}

func NewRedisRepo[T any](c *redis.Client) *RedisRepo[T] {
	var t T
	return &RedisRepo[T]{client: c, indexName: fmt.Sprintf("%T", t)}
}

//nolint:exhaustruct // to much
func (r *RedisRepo[T]) Init(ctx context.Context) error {
	st := r.client.FTCreate(ctx, r.indexName, &redis.FTCreateOptions{
		OnHash: true,
		// Prefix: []interface{}{keyTokenID, keyUserID, keyDeviceID}, // index only these fields
	}, &redis.FieldSchema{
		FieldName: keyTokenID,
		FieldType: redis.SearchFieldTypeTag,
	}, &redis.FieldSchema{
		FieldName: keyUserID,
		FieldType: redis.SearchFieldTypeTag,
	}, &redis.FieldSchema{
		FieldName: keyDeviceID,
		FieldType: redis.SearchFieldTypeTag,
	}, &redis.FieldSchema{
		FieldName: keyTokenState,
		FieldType: redis.SearchFieldTypeNumeric,
	})

	return st.Err()
}

func (r *RedisRepo[T]) Set(ctx context.Context, tokenID auth.TokenID[T], state auth.TokenState) error {
	// TODO: think expiration

	_, err := r.client.FTDictAdd(
		ctx,
		r.indexName,
		tokenID.ID.String(),
		tokenID.UserID.String(),
		string(tokenID.DeviceID),
		int(state.Status),
	).Result()
	if err != nil {
		return fmt.Errorf("adding token to Redis failed: %w", err)
	}

	return nil
}

func (r *RedisRepo[T]) GetByID(ctx context.Context, targetID id.ID[T]) (auth.TokenID[T], auth.TokenState, error) {
	cmd := r.client.FTSearch(ctx, r.indexName, fmt.Sprintf("tokenId:%s", targetID.String()))

	pretty.Println(cmd.Val().Docs)

	return auth.TokenID[T]{}, auth.TokenState{}, nil
}

func (r *RedisRepo[T]) RevokeByUserID(ctx context.Context, userID id.ID[user.User]) error {
	return nil
}

func (r *RedisRepo[T]) DeleteByID(ctx context.Context, targetID id.ID[T]) error {
	// r.client.FTDictDel(ctx, r.indexName, )
	return nil
}

func (r *RedisRepo[T]) RevokeByDeviceID(ctx context.Context, userID id.ID[user.User], deviceID auth.DeviceID) error {
	return nil
}
