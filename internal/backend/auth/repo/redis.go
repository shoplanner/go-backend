package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/kr/pretty"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"go-backend/internal/backend/auth"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
)

const (
	keyTokenID    = "id"
	keyUserID     = "user_id"
	keyDeviceID   = "device_id"
	keyTokenState = "status"
)

type RedisRepo[T any] struct {
	client    *redis.Client
	baseName  string
	indexName string
}

func NewRedisRepo[T any](c *redis.Client) *RedisRepo[T] {
	var t T
	baseName := fmt.Sprintf("%T", t)

	return &RedisRepo[T]{client: c, baseName: baseName, indexName: "idx:" + baseName}
}

//nolint:exhaustruct // to much
func (r *RedisRepo[T]) Init(ctx context.Context) error {
	st := r.client.FTCreate(ctx, "idx:"+r.baseName, &redis.FTCreateOptions{
		OnJSON: true,
		Prefix: []interface{}{r.baseName + ":"},
	}, &redis.FieldSchema{
		FieldName: pathJSON(keyTokenID),
		As:        keyTokenID,
		FieldType: redis.SearchFieldTypeTag,
	}, &redis.FieldSchema{
		FieldName: pathJSON(keyUserID),
		As:        keyUserID,
		FieldType: redis.SearchFieldTypeTag,
	}, &redis.FieldSchema{
		FieldName: pathJSON(keyDeviceID),
		As:        keyDeviceID,
		FieldType: redis.SearchFieldTypeTag,
	}, &redis.FieldSchema{
		FieldName: pathJSON(keyTokenState),
		As:        keyTokenState,
		FieldType: redis.SearchFieldTypeTag,
	})

	return st.Err()
}

type record struct {
	ID       string `json:"id"`
	UserID   string `json:"user_id"`
	DeviceID string `json:"device_id"`
	Status   string `json:"status"`
}

func (r *RedisRepo[T]) Set(ctx context.Context, tokenID auth.TokenID[T], state auth.TokenState) error {
	// TODO: think expiration

	err := r.client.JSONSet(
		ctx,
		fmt.Sprintf("%s:%s", r.baseName, tokenID.ID.String()),
		"$",
		tokenToRecord(tokenID, state),
	).Err()
	if err != nil {
		return fmt.Errorf("adding token to redis failed: %w", err)
	}

	return nil
}

func (r *RedisRepo[T]) GetByID(ctx context.Context, targetID id.ID[T]) (auth.TokenID[T], auth.TokenState, error) {
	var rec []record
	val, err := r.client.JSONGet(ctx, r.baseName+fmt.Sprintf(":%s", targetID), "$").Result()
	if err != nil {
		return auth.TokenID[T]{}, auth.TokenState{}, fmt.Errorf("can't get token from redis: %w", err)
	}

	log.Info().Str("kek", val).Send()

	if err = json.Unmarshal([]byte(val), &rec); err != nil {
		return auth.TokenID[T]{}, auth.TokenState{}, fmt.Errorf("can't decode token got from redis: %w", err)
	}

	tokenID, state := recordToToken[T](rec[0])
	return tokenID, state, nil
}

func (r *RedisRepo[T]) RevokeByUserID(ctx context.Context, userID id.ID[user.User]) error {
	res, err := r.client.FTSearchWithArgs(ctx, r.indexName, fmt.Sprintf("@%s:{%s}", keyUserID, userID), &redis.FTSearchOptions{
		NoContent: false,
	}).Result()
	if err != nil {
		return fmt.Errorf("can't select tokens by user id %s: %w", userID, err)
	}

	pretty.Println(res.Docs[0])
	return nil
}

func (r *RedisRepo[T]) DeleteByID(ctx context.Context, targetID id.ID[T]) error {
	// r.client.FTDictDel(ctx, r.indexName, )
	return nil
}

func (r *RedisRepo[T]) RevokeByDeviceID(ctx context.Context, userID id.ID[user.User], deviceID auth.DeviceID) error {
	return nil
}

func pathJSON(fieldName string) string {
	return "$." + fieldName
}

func escapeUUID(toEscape uuid.UUID) string {
	return strings.ReplaceAll(toEscape.String(), "-", "\\-")
}

func unescapeUUID(s string) uuid.UUID {
	newUUID, _ := uuid.Parse(strings.ReplaceAll(s, "\\-", "-"))
	return newUUID
}

func tokenToRecord[T any](tokenID auth.TokenID[T], state auth.TokenState) record {
	return record{
		ID:       escapeUUID(tokenID.ID.UUID),
		UserID:   escapeUUID(tokenID.UserID.UUID),
		DeviceID: string(tokenID.DeviceID),
		Status:   state.Status.String(),
	}
}

func recordToToken[T any](r record) (auth.TokenID[T], auth.TokenState) {
	parsedStatus, _ := auth.ParseTokenStatus(r.Status)
	return auth.TokenID[T]{
			ID:       id.ID[T]{UUID: unescapeUUID(r.ID)},
			UserID:   id.ID[user.User]{UUID: unescapeUUID(r.UserID)},
			DeviceID: auth.DeviceID(r.DeviceID),
		}, auth.TokenState{
			Status: parsedStatus,
		}
}
