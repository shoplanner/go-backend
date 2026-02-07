package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"

	"go-backend/internal/backend/list"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
)

type Repo struct{ db *sql.DB }

func NewRepo(ctx context.Context, db *sql.DB) (*Repo, error) {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS product_lists (
		id TEXT PRIMARY KEY,
		model_json TEXT NOT NULL
	)`)
	if err != nil {
		return nil, fmt.Errorf("can't initialize list tables: %w", err)
	}
	return &Repo{db: db}, nil
}

func (r *Repo) GetListMetaByUserID(ctx context.Context, userID id.ID[user.User]) ([]list.ProductList, error) {
	models, err := r.getAll(ctx)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(models, func(m list.ProductList) bool {
		return !slices.ContainsFunc(m.Members, func(member list.Member) bool { return member.UserID == userID })
	}), nil
}

func (r *Repo) GetByListID(ctx context.Context, listID id.ID[list.ProductList]) (list.ProductList, error) {
	var raw string
	if err := r.db.QueryRowContext(ctx, `SELECT model_json FROM product_lists WHERE id = ?`, listID.String()).Scan(&raw); err != nil {
		return list.ProductList{}, fmt.Errorf("can't select product list %s: %w", listID, err)
	}
	return decode(raw)
}

func (r *Repo) CreateList(ctx context.Context, model list.ProductList) error {
	data, _ := json.Marshal(model)
	_, err := r.db.ExecContext(ctx, `INSERT INTO product_lists(id, model_json) VALUES(?, ?)`, model.ID.String(), string(data))
	if err != nil {
		return fmt.Errorf("can't create list %s: %w", model.ID, err)
	}
	return nil
}

func (r *Repo) GetAndUpdate(ctx context.Context, listID id.ID[list.ProductList], f func(list.ProductList) (list.ProductList, error)) (list.ProductList, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return list.ProductList{}, fmt.Errorf("transaction failed: %w", err)
	}
	defer tx.Rollback()
	model, err := getByID(ctx, tx, listID)
	if err != nil {
		return list.ProductList{}, err
	}
	model, err = f(model)
	if err != nil {
		return model, err
	}
	if err = save(ctx, tx, model); err != nil {
		return model, err
	}
	if err = tx.Commit(); err != nil {
		return model, err
	}
	return model, nil
}

func (r *Repo) GetAndDeleteList(ctx context.Context, listID id.ID[list.ProductList], f func(list.ProductList) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}
	defer tx.Rollback()
	model, err := getByID(ctx, tx, listID)
	if err != nil {
		return err
	}
	if err = f(model); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM product_lists WHERE id = ?`, listID.String()); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repo) ApplyOrder(ctx context.Context, roleCheck list.RoleCheckFunc, listID id.ID[list.ProductList], ids []id.ID[product.Product]) error {
	return withTx(ctx, r.db, func(tx *sql.Tx) error {
		model, err := getByID(ctx, tx, listID)
		if err != nil {
			return err
		}
		if err = roleCheck(model.Members); err != nil {
			return err
		}
		stateByID := map[string]list.ProductState{}
		for _, s := range model.States {
			stateByID[s.Product.ID.String()] = s
		}
		ordered := make([]list.ProductState, 0, len(ids))
		for _, pid := range ids {
			state, ok := stateByID[pid.String()]
			if !ok {
				return fmt.Errorf("product %s is not in list %s", pid, listID)
			}
			ordered = append(ordered, state)
		}
		model.States = ordered
		return save(ctx, tx, model)
	})
}

func (r *Repo) getAll(ctx context.Context) ([]list.ProductList, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT model_json FROM product_lists`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []list.ProductList{}
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		m, err := decode(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func withTx(ctx context.Context, db *sql.DB, f func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = f(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func getByID(ctx context.Context, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, listID id.ID[list.ProductList]) (list.ProductList, error) {
	var raw string
	if err := db.QueryRowContext(ctx, `SELECT model_json FROM product_lists WHERE id = ?`, listID.String()).Scan(&raw); err != nil {
		return list.ProductList{}, err
	}
	return decode(raw)
}

func save(ctx context.Context, db interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, model list.ProductList) error {
	data, _ := json.Marshal(model)
	_, err := db.ExecContext(ctx, `UPDATE product_lists SET model_json = ? WHERE id = ?`, string(data), model.ID.String())
	return err
}

func decode(raw string) (list.ProductList, error) {
	var model list.ProductList
	if err := json.Unmarshal([]byte(raw), &model); err != nil {
		return list.ProductList{}, err
	}
	return model, nil
}
