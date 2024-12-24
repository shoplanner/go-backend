package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/redis/rueidis"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"

	"go-backend/internal/backend/auth"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

const (
	keyTokenID    = "id"
	keyUserID     = "user_id"
	keyDeviceID   = "device_id"
	keyTokenState = "status"

	jsonRootPath = "$"
)

type record struct {
	ID       string `json:"id"`
	UserID   string `json:"user_id"`
	DeviceID string `json:"device_id"`
	Status   string `json:"status"`
}

type RedisRepo[T any] struct {
	baseName  string
	indexName string
	client    rueidis.Client
}

func NewRedisRepo[T any](client rueidis.Client) *RedisRepo[T] {
	var t T
	baseName := fmt.Sprintf("%T", t)

	return &RedisRepo[T]{client: client, baseName: baseName, indexName: "idx:" + baseName}
}

func (r *RedisRepo[T]) Init(ctx context.Context) error {
	cmd := r.client.B().FtCreate().
		Index(r.indexName).OnJson().Prefix(1).Prefix(fmt.Sprintf("%s:", r.baseName)).
		Schema().
		FieldName(pathJSON(keyTokenID)).As(keyTokenID).Tag().
		FieldName(pathJSON(keyUserID)).As(keyUserID).Tag().
		FieldName(pathJSON(keyDeviceID)).As(keyDeviceID).Tag().
		FieldName(pathJSON(keyTokenState)).As(keyTokenState).Tag().
		Build()

	res := r.client.Do(ctx, cmd)

	if res.Error() != nil {
		return fmt.Errorf("can't create redis index: %w", res.Error())
	}

	return nil
}

func (r *RedisRepo[T]) Set(ctx context.Context, tokenID auth.TokenID[T], state auth.TokenState) error {
	// TODO: think expiration

	bytes, err := json.Marshal(tokenToRecord(tokenID, state))
	if err != nil {
		return fmt.Errorf("can't encode token to JSON: %w", err)
	}

	cmd := r.client.B().JsonSet().Key(r.keyName(tokenID.ID.String())).Path(jsonRootPath).Value(string(bytes)).Build()

	if err = r.client.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("can't save token to redis: %w", err)
	}

	return nil
}

func (r *RedisRepo[T]) GetByID(ctx context.Context, targetID id.ID[T]) (auth.TokenID[T], auth.TokenState, error) {
	var rec []record

	cmd := r.client.B().JsonGet().Key(r.keyName(targetID.String())).Path(jsonRootPath).Build()

	if err := r.client.Do(ctx, cmd).DecodeJSON(&rec); err != nil {
		return auth.TokenID[T]{}, auth.TokenState{}, fmt.Errorf("can't decode token got from redis: %w", err)
	}

	if len(rec) == 0 {
		return auth.TokenID[T]{}, auth.TokenState{}, fmt.Errorf("%w: token with id %s", myerr.ErrNotFound, targetID)
	}

	tokenID, state := recordToToken[T](rec[0])

	log.Debug().Any("tokenID", tokenID).Any("state", state).Str("component", r.baseName+" redis repo").Msg("got token by id")

	return tokenID, state, nil
}

func (r *RedisRepo[T]) DeleteByID(ctx context.Context, targetID id.ID[T]) error {
	cmd := r.client.B().JsonDel().Key(r.keyName(targetID.String())).Path(jsonRootPath).Build()

	if err := r.client.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("can't delete token %s: %w", targetID, err)
	}

	return nil
}

func (r *RedisRepo[T]) RevokeByUserID(ctx context.Context, userID id.ID[user.User]) error {
	return r.revokeTokens(ctx, fmt.Sprintf("@%s:{%s}", keyUserID, escapeUUID(userID.UUID)))
}

func (r *RedisRepo[T]) RevokeByDeviceID(ctx context.Context, userID id.ID[user.User], deviceID auth.DeviceID) error {
	return r.revokeTokens(ctx, fmt.Sprintf(
		"@%s:{%s} @%s:{%s}",
		keyUserID,
		escapeUUID(userID.UUID),
		keyDeviceID,
		deviceID,
	))
}

func (r *RedisRepo[T]) revokeTokens(ctx context.Context, query string) error {
	err := r.client.Dedicated(func(client rueidis.DedicatedClient) error {
		searchCmd := client.B().FtSearch().Index(r.indexName).Query(query).Nocontent().Build()

		total, res, err := client.Do(ctx, searchCmd).AsFtSearch()
		if total == 0 {
			return fmt.Errorf("%w: can't found tokens", myerr.ErrNotFound)
		}
		if err != nil {
			return fmt.Errorf("searching for tokens failed: %w", err)
		}

		keys := lo.Map(res, func(item rueidis.FtSearchDoc, _ int) string { return item.Key })

		client.B().Watch().Key(keys...).Build()

		kek, _ := json.Marshal(auth.TokenStatusRevoked.String())

		mset := client.B().JsonMset().Key(keys[0]).Path(pathJSON(keyTokenState)).Value(string(kek))

		for _, key := range keys[1:] {
			mset = mset.Key(key).Path(pathJSON(keyTokenState)).Value(string(kek))
		}

		log.Debug().Any("token ids", keys).Msg("revoking tokens")

		multi := client.DoMulti(ctx,
			client.B().Multi().Build(),
			mset.Build(),
			client.B().Exec().Build(),
		)

		var errs []error
		for _, r := range multi {
			if err = r.Error(); err != nil {
				errs = append(errs, err)
			}
		}

		return errors.Join(errs...)
	})
	if err != nil {
		return fmt.Errorf("connection to redis failed: %w", err)
	}

	return nil
}

func (r *RedisRepo[T]) keyName(tokenID string) string {
	return fmt.Sprintf("%s:%s", r.baseName, tokenID)
}

func pathJSON(fieldName string) string {
	return fmt.Sprintf("%s.%s", jsonRootPath, fieldName)
}

func escapeUUID(toEscape uuid.UUID) string {
	return strings.ReplaceAll(toEscape.String(), "-", "\\-")
}

func unescapeUUID(s string) uuid.UUID {
	newUUID, _ := uuid.Parse(strings.ReplaceAll(s, "-", "-"))
	return newUUID
}

func tokenToRecord[T any](tokenID auth.TokenID[T], state auth.TokenState) record {
	return record{
		ID:       tokenID.ID.String(),
		UserID:   tokenID.UserID.String(),
		DeviceID: string(tokenID.DeviceID),
		Status:   state.Status.String(),
	}
}

func recordToToken[T any](r record) (auth.TokenID[T], auth.TokenState) {
	parsedStatus, err := auth.ParseTokenStatus(r.Status)
	if err != nil {
		log.Warn().Err(err).Msg("casting record to token")
	}
	return auth.TokenID[T]{
			ID:       id.ID[T]{UUID: unescapeUUID(r.ID)},
			UserID:   id.ID[user.User]{UUID: unescapeUUID(r.UserID)},
			DeviceID: auth.DeviceID(r.DeviceID),
		}, auth.TokenState{
			Status: parsedStatus,
		}
}
