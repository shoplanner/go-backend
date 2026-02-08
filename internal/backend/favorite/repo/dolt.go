package repo

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/samber/mo"

	"go-backend/internal/backend/favorite"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/god"
	"go-backend/pkg/id"
)

type Repo struct {
	db *sql.DB
}

func NewRepo(ctx context.Context, db *sql.DB) (*Repo, error) {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS favorite_lists (
			id TEXT PRIMARY KEY,
			list_type INTEGER NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS favorite_members (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			favorite_list_id TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			member_type INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS favorite_products (
			id TEXT PRIMARY KEY,
			product_id TEXT NOT NULL,
			favorite_list_id TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			UNIQUE(product_id, favorite_list_id)
		)`,
	}

	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return nil, fmt.Errorf("can't create favorites tables: %w", err)
		}
	}

	return &Repo{db: db}, nil
}

func (r *Repo) CreateList(ctx context.Context, model favorite.List) error {
	return withTx(ctx, r.db, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(
			ctx,
			`INSERT INTO favorite_lists(id, list_type, created_at, updated_at) VALUES(?, ?, ?, ?)`,
			model.ID.String(),
			int32(model.Type),
			model.CreatedAt.Time,
			model.UpdatedAt.Time,
		)
		if err != nil {
			return fmt.Errorf("can't create new list %s: %w", model.ID, err)
		}

		for _, member := range model.Members {
			_, err = tx.ExecContext(
				ctx,
				`INSERT INTO favorite_members(id, user_id, favorite_list_id, created_at, updated_at, member_type)
				 VALUES(?, ?, ?, ?, ?, ?)`,
				uuid.NewString(),
				member.UserID.String(),
				model.ID.String(),
				member.CreatedAt.Time,
				member.UpdatedAt.Time,
				int32(member.Type),
			)
			if err != nil {
				return err
			}
		}

		for _, p := range model.Products {
			_, err = tx.ExecContext(
				ctx,
				`INSERT INTO favorite_products(id, product_id, favorite_list_id, created_at, updated_at)
				 VALUES(?, ?, ?, ?, ?)
				 ON CONFLICT(product_id, favorite_list_id) DO UPDATE SET updated_at=excluded.updated_at`,
				uuid.NewString(),
				p.Product.ID.String(),
				model.ID.String(),
				p.CreatedAt.Time,
				p.UpdatedAt.Time,
			)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *Repo) DeleteList(ctx context.Context, listID id.ID[favorite.List]) error {
	return withTx(ctx, r.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM favorite_members WHERE favorite_list_id = ?`, listID.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM favorite_products WHERE favorite_list_id = ?`, listID.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM favorite_lists WHERE id = ?`, listID.String()); err != nil {
			return fmt.Errorf("can't delete favorites list %s: %w", listID, err)
		}
		return nil
	})
}

func (r *Repo) GetByID(ctx context.Context, listID id.ID[favorite.List]) (favorite.List, error) {
	return r.getByID(ctx, r.db, listID)
}

func (r *Repo) GetByUserID(ctx context.Context, userID id.ID[user.User]) ([]favorite.List, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT DISTINCT fl.id
		 FROM favorite_lists fl
		 JOIN favorite_members fm ON fm.favorite_list_id = fl.id
		 WHERE fm.user_id = ?`,
		userID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("can't get lists of user %s: %w", userID, err)
	}
	defer rows.Close()

	ids := []id.ID[favorite.List]{}
	for rows.Next() {
		var listIDRaw string
		if err = rows.Scan(&listIDRaw); err != nil {
			return nil, err
		}
		ids = append(ids, id.ID[favorite.List]{UUID: god.Believe(uuid.Parse(listIDRaw))})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	result := make([]favorite.List, 0, len(ids))
	for _, listIDItem := range ids {
		item, getErr := r.GetByID(ctx, listIDItem)
		if getErr != nil {
			return nil, getErr
		}
		result = append(result, item)
	}

	return result, nil
}

func (r *Repo) GetListsByMembership(
	ctx context.Context,
	userID id.ID[user.User],
	memberType favorite.MemberType,
) (
	[]favorite.List,
	error,
) {
	models, err := r.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return lo.Filter(models, func(item favorite.List, _ int) bool {
		member, found := lo.Find(item.Members, func(m favorite.Member) bool { return m.UserID == userID })
		return found && member.Type == memberType
	}), nil
}

func (r *Repo) GetAndUpdate(
	ctx context.Context,
	listID id.ID[favorite.List],
	f func(favorite.List) (favorite.List, error),
) (
	favorite.List,
	error,
) {
	var model favorite.List
	err := withTx(ctx, r.db, func(tx *sql.Tx) error {
		var err error
		model, err = r.getByID(ctx, tx, listID)
		if err != nil {
			return err
		}

		model, err = f(model)
		if err != nil {
			return err
		}

		if _, err = tx.ExecContext(ctx, `UPDATE favorite_lists SET list_type = ?, created_at = ?, updated_at = ? WHERE id = ?`,
			int32(model.Type), model.CreatedAt.Time, model.UpdatedAt.Time, listID.String()); err != nil {
			return err
		}

		if _, err = tx.ExecContext(ctx, `DELETE FROM favorite_members WHERE favorite_list_id = ?`, listID.String()); err != nil {
			return err
		}
		for _, member := range model.Members {
			if _, err = tx.ExecContext(ctx,
				`INSERT INTO favorite_members(id, user_id, favorite_list_id, created_at, updated_at, member_type)
				 VALUES(?, ?, ?, ?, ?, ?)`,
				uuid.NewString(),
				member.UserID.String(),
				listID.String(),
				member.CreatedAt.Time,
				member.UpdatedAt.Time,
				int32(member.Type),
			); err != nil {
				return err
			}
		}

		if _, err = tx.ExecContext(ctx, `DELETE FROM favorite_products WHERE favorite_list_id = ?`, listID.String()); err != nil {
			return err
		}
		for _, p := range model.Products {
			if _, err = tx.ExecContext(ctx,
				`INSERT INTO favorite_products(id, product_id, favorite_list_id, created_at, updated_at)
				 VALUES(?, ?, ?, ?, ?)
				 ON CONFLICT(product_id, favorite_list_id) DO UPDATE SET updated_at=excluded.updated_at`,
				uuid.NewString(),
				p.Product.ID.String(),
				listID.String(),
				p.CreatedAt.Time,
				p.UpdatedAt.Time,
			); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return model, fmt.Errorf("transaction failed: %w", err)
	}

	return model, nil
}

func (r *Repo) getByID(
	ctx context.Context,
	db interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	},
	listID id.ID[favorite.List],
) (favorite.List, error) {
	model := favorite.List{ID: listID}

	if err := db.QueryRowContext(
		ctx,
		`SELECT list_type, created_at, updated_at FROM favorite_lists WHERE id = ?`,
		listID.String(),
	).Scan(&model.Type, &model.CreatedAt.Time, &model.UpdatedAt.Time); err != nil {
		return favorite.List{}, fmt.Errorf("can't select favorites list %s: %w", listID, err)
	}

	membersRows, err := db.QueryContext(
		ctx,
		`SELECT user_id, member_type, created_at, updated_at
		 FROM favorite_members
		 WHERE favorite_list_id = ?`,
		listID.String(),
	)
	if err != nil {
		return favorite.List{}, err
	}
	defer membersRows.Close()

	model.Members = []favorite.Member{}
	for membersRows.Next() {
		var userIDRaw string
		var m favorite.Member
		if err = membersRows.Scan(&userIDRaw, &m.Type, &m.CreatedAt.Time, &m.UpdatedAt.Time); err != nil {
			return favorite.List{}, err
		}
		m.UserID = id.ID[user.User]{UUID: god.Believe(uuid.Parse(userIDRaw))}
		model.Members = append(model.Members, m)
	}

	productsRows, err := db.QueryContext(
		ctx,
		`SELECT product_id, created_at, updated_at
		 FROM favorite_products
		 WHERE favorite_list_id = ?`,
		listID.String(),
	)
	if err != nil {
		return favorite.List{}, err
	}
	defer productsRows.Close()

	productMeta := map[string]favorite.Favorite{}
	productIDs := []string{}
	for productsRows.Next() {
		var productID string
		var item favorite.Favorite
		if err = productsRows.Scan(&productID, &item.CreatedAt.Time, &item.UpdatedAt.Time); err != nil {
			return favorite.List{}, err
		}
		item.Product.ID = id.ID[product.Product]{UUID: god.Believe(uuid.Parse(productID))}
		productMeta[productID] = item
		productIDs = append(productIDs, productID)
	}

	productMap, err := loadProducts(ctx, db, productIDs)
	if err != nil {
		return favorite.List{}, err
	}

	sort.Strings(productIDs)
	model.Products = make([]favorite.Favorite, 0, len(productIDs))
	for _, pid := range productIDs {
		item := productMeta[pid]
		item.Product = productMap[pid]
		model.Products = append(model.Products, item)
	}

	return model, nil
}

func loadProducts(
	ctx context.Context,
	db interface {
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	},
	ids []string,
) (map[string]product.Product, error) {
	if len(ids) == 0 {
		return map[string]product.Product{}, nil
	}

	args := make([]any, 0, len(ids))
	placeholders := make([]string, 0, len(ids))
	for _, v := range ids {
		args = append(args, v)
		placeholders = append(placeholders, "?")
	}

	rows, err := db.QueryContext(ctx,
		`SELECT p.id, p.created_at, p.updated_at, p.name, pc.name, pf.name
		 FROM products p
		 LEFT JOIN product_categories pc ON p.category_id = pc.id OR p.category_id = pc.name
		 LEFT JOIN product_forms pf ON pf.product_id = p.id
		 WHERE p.id IN (`+strings.Join(placeholders, ",")+`)
		 ORDER BY p.id`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type aggregate struct {
		product product.Product
		forms   map[string]struct{}
	}

	agg := map[string]*aggregate{}
	for rows.Next() {
		var (
			idRaw, name string
			category    sql.NullString
			form        sql.NullString
			createdAt   time.Time
			updatedAt   time.Time
		)
		if err = rows.Scan(&idRaw, &createdAt, &updatedAt, &name, &category, &form); err != nil {
			return nil, err
		}

		if _, ok := agg[idRaw]; !ok {
			cat := mo.None[product.Category]()
			if category.Valid {
				cat = mo.Some(product.Category(category.String))
			}
			agg[idRaw] = &aggregate{product: product.Product{
				Options: product.Options{Name: product.Name(name), Category: cat, Forms: []product.Form{}},
				ID:      id.ID[product.Product]{UUID: god.Believe(uuid.Parse(idRaw))},
				CreatedAt: date.CreateDate[product.Product]{
					Time: createdAt,
				},
				UpdatedAt: date.UpdateDate[product.Product]{
					Time: updatedAt,
				},
			}, forms: map[string]struct{}{}}
		}

		if form.Valid {
			if _, exists := agg[idRaw].forms[form.String]; !exists {
				agg[idRaw].forms[form.String] = struct{}{}
				agg[idRaw].product.Forms = append(agg[idRaw].product.Forms, product.Form(form.String))
			}
		}
	}

	result := make(map[string]product.Product, len(agg))
	for key, value := range agg {
		result[key] = value.product
	}
	return result, rows.Err()
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
