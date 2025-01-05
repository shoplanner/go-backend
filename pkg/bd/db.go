package bd

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
)

type DB struct {
	provider *sql.DB
	log      zerolog.Logger
}

func NewDB(db *sql.DB, log zerolog.Logger) *DB {
	log.UpdateContext(func(c zerolog.Context) zerolog.Context {
		c.Str("component", "general sql adapter")
		return c
	})
	return &DB{
		provider: db,
		log:      log,
	}
}

func (d *DB) Tx(ctx context.Context, f func(context.Context, *DB) error) error {
	tx, err := d.provider.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("can't start transaction: %w", err)
	}
	defer func() {
		if err = tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			d.log.Err(err).Msg("transaction rollback")
		}
	}()

	if err = f(ctx, d); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("can't commit transaction: %w", err)
	}

	return nil
}

func (d *DB) ExecContext(ctx context.Context, q string, args ...interface{}) (sql.Result, error) {
	d.log.Debug().Str("query", q).Any("args", args).Msg("executing request")
	return d.provider.ExecContext(ctx, q, args...) //nolint:wrapcheck
}

func (d *DB) QueryContext(ctx context.Context, q string, args ...interface{}) (*sql.Rows, error) {
	d.log.Debug().Str("query", q).Any("args", args).Msg("running query")
	return d.provider.QueryContext(ctx, q, args...) //nolint:wrapcheck
}

func (d *DB) QueryRowContext(ctx context.Context, q string, args ...interface{}) *sql.Row {
	d.log.Debug().Str("query", q).Any("args", args).Msg("running single row query")
	return d.provider.QueryRowContext(ctx, q, args...) //nolint:wrapcheck
}

func (d *DB) PrepareContext(ctx context.Context, q string) (*sql.Stmt, error) {
	d.log.Debug().Str("query", q).Msg("preparing statement")
	return d.provider.PrepareContext(ctx, q) //nolint:wrapcheck
}
