package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"

	"go-backend/internal/backend/favorite"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
)

type Repo struct{ db *sql.DB }

func NewRepo(ctx context.Context, db *sql.DB) (*Repo, error) {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS favorite_lists (
		id TEXT PRIMARY KEY,
		model_json TEXT NOT NULL
	)`)
	if err != nil {
		return nil, fmt.Errorf("can't create favorites tables: %w", err)
	}
	return &Repo{db: db}, nil
}

func (r *Repo) CreateList(ctx context.Context, model favorite.List) error {
	data, _ := json.Marshal(model)
	_, err := r.db.ExecContext(ctx, `INSERT INTO favorite_lists(id, model_json) VALUES(?,?)`, model.ID.String(), string(data))
	if err != nil {
		return fmt.Errorf("can't create new list %s: %w", model.ID, err)
	}
	return nil
}

func (r *Repo) DeleteList(ctx context.Context, listID id.ID[favorite.List]) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM favorite_lists WHERE id = ?`, listID.String())
	if err != nil {
		return fmt.Errorf("can't delete favorites list %s: %w", listID, err)
	}
	return nil
}

func (r *Repo) GetByID(ctx context.Context, listID id.ID[favorite.List]) (favorite.List, error) {
	var raw string
	if err := r.db.QueryRowContext(ctx, `SELECT model_json FROM favorite_lists WHERE id = ?`, listID.String()).Scan(&raw); err != nil {
		return favorite.List{}, fmt.Errorf("can't select favorites list %s: %w", listID, err)
	}
	return decode(raw)
}

func (r *Repo) GetByUserID(ctx context.Context, userID id.ID[user.User]) ([]favorite.List, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT model_json FROM favorite_lists`)
	if err != nil {
		return nil, fmt.Errorf("can't get lists of user %s: %w", userID, err)
	}
	defer rows.Close()
	out := []favorite.List{}
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		model, err := decode(raw)
		if err != nil {
			return nil, err
		}
		if slices.ContainsFunc(model.Members, func(m favorite.Member) bool { return m.UserID == userID }) {
			out = append(out, model)
		}
	}
	return out, rows.Err()
}

func (r *Repo) GetListsByMembership(ctx context.Context, userID id.ID[user.User], memberType favorite.MemberType) ([]favorite.List, error) {
	models, err := r.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(models, func(l favorite.List) bool {
		idx := slices.IndexFunc(l.Members, func(m favorite.Member) bool { return m.UserID == userID })
		return idx == -1 || l.Members[idx].Type != memberType
	}), nil
}

func (r *Repo) GetAndUpdate(ctx context.Context, listID id.ID[favorite.List], f func(favorite.List) (favorite.List, error)) (favorite.List, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return favorite.List{}, fmt.Errorf("transaction failed: %w", err)
	}
	defer tx.Rollback()
	var raw string
	if err = tx.QueryRowContext(ctx, `SELECT model_json FROM favorite_lists WHERE id = ?`, listID.String()).Scan(&raw); err != nil {
		return favorite.List{}, err
	}
	model, err := decode(raw)
	if err != nil {
		return favorite.List{}, err
	}
	model, err = f(model)
	if err != nil {
		return model, err
	}
	data, _ := json.Marshal(model)
	if _, err = tx.ExecContext(ctx, `UPDATE favorite_lists SET model_json = ? WHERE id = ?`, string(data), listID.String()); err != nil {
		return model, err
	}
	if err = tx.Commit(); err != nil {
		return model, err
	}
	return model, nil
}

func decode(raw string) (favorite.List, error) {
	var model favorite.List
	if err := json.Unmarshal([]byte(raw), &model); err != nil {
		return favorite.List{}, err
	}
	return model, nil
}
