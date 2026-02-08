package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/samber/mo"

	"go-backend/internal/backend/list"
	"go-backend/internal/backend/product"
	productRepo "go-backend/internal/backend/product/repo"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/god"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
	"go-backend/pkg/mymysql"
)

type Repo struct {
	db *sql.DB
}

func NewRepo(ctx context.Context, db *sql.DB) (*Repo, error) {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS product_lists (
			id TEXT PRIMARY KEY,
			status INTEGER NOT NULL,
			updated_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL,
			title TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS product_list_members (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			list_id TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			member_type INTEGER NOT NULL,
			UNIQUE(user_id, list_id)
		)`,
		`CREATE TABLE IF NOT EXISTS product_list_states (
			id TEXT PRIMARY KEY,
			product_id TEXT NOT NULL,
			list_id TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			"index" INTEGER NOT NULL,
			count INTEGER NULL,
			form_idx INTEGER NULL,
			status INTEGER NOT NULL,
			replacement_count INTEGER NULL,
			replacement_form_idx INTEGER NULL,
			replacement_product_id TEXT NULL
		)`,
	}

	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return nil, fmt.Errorf("can't create product list tables: %w", err)
		}
	}

	return &Repo{db: db}, nil
}

func (r *Repo) GetListMetaByUserID(ctx context.Context, userID id.ID[user.User]) ([]list.ProductList, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT list_id FROM product_list_members WHERE user_id = ?`, userID.String())
	if err != nil {
		return nil, fmt.Errorf("can't find ids of lists related to user %s: %w", userID, err)
	}
	defer rows.Close()

	ids := []id.ID[list.ProductList]{}
	for rows.Next() {
		var listIDRaw string
		if err = rows.Scan(&listIDRaw); err != nil {
			return nil, err
		}
		ids = append(ids, id.ID[list.ProductList]{UUID: god.Believe(uuid.Parse(listIDRaw))})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	result := make([]list.ProductList, 0, len(ids))
	for _, listIDItem := range ids {
		item, getErr := r.GetByListID(ctx, listIDItem)
		if getErr != nil {
			return nil, getErr
		}
		result = append(result, item)
	}

	return result, nil
}

func (r *Repo) GetByListID(ctx context.Context, listID id.ID[list.ProductList]) (list.ProductList, error) {
	return r.getProductList(ctx, r.db, listID)
}

func (r *Repo) CreateList(ctx context.Context, model list.ProductList) error {
	err := withTx(ctx, r.db, func(tx *sql.Tx) error {
		if err := insertOrUpdateList(ctx, tx, model); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("can't insert new product list %s: %w", model.ID, err)
	}

	return nil
}

func (r *Repo) GetAndUpdate(
	ctx context.Context,
	listID id.ID[list.ProductList],
	updateFunc func(list.ProductList) (list.ProductList, error),
) (
	list.ProductList,
	error,
) {
	var model list.ProductList
	err := withTx(ctx, r.db, func(tx *sql.Tx) error {
		var err error

		model, err = r.getProductList(ctx, tx, listID)
		if err != nil {
			return err
		}

		model, err = updateFunc(model)
		if err != nil {
			return err
		}

		if err = insertOrUpdateList(ctx, tx, model); err != nil {
			return err
		}

		model, err = r.getProductList(ctx, tx, listID)
		return err
	})
	if err != nil {
		return list.ProductList{}, fmt.Errorf("%w: transaction failed: %w", mymysql.GetType(err), err)
	}

	return model, nil
}

func (r *Repo) GetAndDeleteList(
	ctx context.Context,
	listID id.ID[list.ProductList],
	validateFunc func(list.ProductList) error,
) error {
	err := withTx(ctx, r.db, func(tx *sql.Tx) error {
		model, err := r.getProductList(ctx, tx, listID)
		if err != nil {
			return err
		}

		if err = validateFunc(model); err != nil {
			return err
		}

		if _, err = tx.ExecContext(ctx, `DELETE FROM product_list_members WHERE list_id = ?`, listID.String()); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM product_list_states WHERE list_id = ?`, listID.String()); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM product_lists WHERE id = ?`, listID.String()); err != nil {
			return fmt.Errorf("can't delete product list %s: %w", listID, err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	return nil
}

func (r *Repo) ApplyOrder(
	ctx context.Context,
	validateFunc list.RoleCheckFunc,
	listID id.ID[list.ProductList],
	ids []id.ID[product.Product],
) error {
	err := withTx(ctx, r.db, func(tx *sql.Tx) error {
		members, err := r.getMembers(ctx, tx, listID)
		if err != nil {
			return fmt.Errorf("failed to get product list members: %w", err)
		}

		if err = validateFunc(members); err != nil {
			return err
		}

		if len(ids) == 0 {
			return fmt.Errorf("%w: order list is empty", myerr.ErrInvalidArgument)
		}

		for idx, productID := range ids {
			if _, err = tx.ExecContext(
				ctx,
				`UPDATE product_list_states SET "index" = ? WHERE list_id = ? AND product_id = ?`,
				idx,
				listID.String(),
				productID.String(),
			); err != nil {
				return fmt.Errorf("can't apply order to DoltDB: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	return nil
}

func (r *Repo) getMembers(
	ctx context.Context,
	db interface {
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	},
	listID id.ID[list.ProductList],
) ([]list.Member, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT m.user_id, m.member_type, m.created_at, m.updated_at, COALESCE(u.login, '')
		 FROM product_list_members m
		 LEFT JOIN users u ON u.id = m.user_id
		 WHERE m.list_id = ?`,
		listID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query members of list %s: %w", listID, err)
	}
	defer rows.Close()

	members := []list.Member{}
	for rows.Next() {
		var (
			userIDRaw string
			role      int32
			createdAt time.Time
			updatedAt time.Time
			login     string
		)
		if err = rows.Scan(&userIDRaw, &role, &createdAt, &updatedAt, &login); err != nil {
			return nil, err
		}
		members = append(members, list.Member{
			MemberOptions: list.MemberOptions{UserID: id.ID[user.User]{UUID: god.Believe(uuid.Parse(userIDRaw))}, Role: list.MemberType(role)},
			UserName:      user.Login(login),
			CreatedAt:     date.CreateDate[list.Member]{Time: createdAt},
			UpdatedAt:     date.UpdateDate[list.Member]{Time: updatedAt},
		})
	}

	return members, rows.Err()
}

func (r *Repo) getProductList(
	ctx context.Context,
	db interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	},
	listID id.ID[list.ProductList],
) (list.ProductList, error) {
	model := list.ProductList{ID: listID}
	if err := db.QueryRowContext(ctx,
		`SELECT status, updated_at, created_at, title FROM product_lists WHERE id = ?`,
		listID.String(),
	).Scan(&model.Status, &model.UpdatedAt.Time, &model.CreatedAt.Time, &model.Title); err != nil {
		return list.ProductList{}, fmt.Errorf("can't select product list %s: %w", listID, err)
	}

	members, err := r.getMembers(ctx, db, listID)
	if err != nil {
		return list.ProductList{}, err
	}
	model.Members = members

	rows, err := db.QueryContext(ctx,
		`SELECT id, product_id, created_at, updated_at, "index", count, form_idx, status,
			replacement_count, replacement_form_idx, replacement_product_id
		 FROM product_list_states WHERE list_id = ?`,
		listID.String(),
	)
	if err != nil {
		return list.ProductList{}, err
	}
	defer rows.Close()

	type stateRow struct {
		productID            string
		createdAt            time.Time
		updatedAt            time.Time
		index                int64
		count                sql.NullInt32
		formIdx              sql.NullInt32
		status               int
		replacementCount     sql.NullInt32
		replacementFormIdx   sql.NullInt32
		replacementProductID sql.NullString
	}

	statesRaw := []stateRow{}
	productIDs := []string{}
	replacementIDs := []string{}

	for rows.Next() {
		var (
			stateID string
			r       stateRow
		)
		_ = stateID
		if err = rows.Scan(
			&stateID,
			&r.productID,
			&r.createdAt,
			&r.updatedAt,
			&r.index,
			&r.count,
			&r.formIdx,
			&r.status,
			&r.replacementCount,
			&r.replacementFormIdx,
			&r.replacementProductID,
		); err != nil {
			return list.ProductList{}, err
		}
		statesRaw = append(statesRaw, r)
		productIDs = append(productIDs, r.productID)
		if r.replacementProductID.Valid {
			replacementIDs = append(replacementIDs, r.replacementProductID.String)
		}
	}

	allProductIDs := lo.Uniq(append(productIDs, replacementIDs...))
	productMap, err := loadProducts(ctx, db, allProductIDs)
	if err != nil {
		return list.ProductList{}, err
	}

	model.States = make([]list.ProductState, len(statesRaw))
	for _, s := range statesRaw {
		state := list.ProductState{
			ProductStateOptions: list.ProductStateOptions{
				Count:     mo.PointerToOption(lo.Ternary(s.count.Valid, lo.ToPtr(s.count.Int32), (*int32)(nil))),
				FormIndex: mo.PointerToOption(lo.Ternary(s.formIdx.Valid, lo.ToPtr(s.formIdx.Int32), (*int32)(nil))),
				Status:    list.StateStatus(s.status),
			},
			Product:   productMap[s.productID],
			CreatedAt: date.CreateDate[list.ProductState]{Time: s.createdAt},
			UpdatedAt: date.UpdateDate[list.ProductState]{Time: s.updatedAt},
		}
		if s.replacementProductID.Valid {
			repl := &list.ProductStateReplacement{
				Count:     mo.PointerToOption(lo.Ternary(s.replacementCount.Valid, lo.ToPtr(s.replacementCount.Int32), (*int32)(nil))),
				FormIndex: mo.PointerToOption(lo.Ternary(s.replacementFormIdx.Valid, lo.ToPtr(s.replacementFormIdx.Int32), (*int32)(nil))),
				Product:   productMap[s.replacementProductID.String],
			}
			state.Replacement = mo.PointerToOption(repl)
		}
		if int(s.index) >= len(model.States) {
			return list.ProductList{}, fmt.Errorf("invalid state index %d for list %s", s.index, listID)
		}
		model.States[s.index] = state
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
		`SELECT p.id, p.created_at, p.updated_at, p.name, p.category_id, pc.name, pf.id, pf.name
		 FROM products p
		 LEFT JOIN product_categories pc ON p.category_id = pc.id OR p.category_id = pc.name
		 LEFT JOIN product_forms pf ON pf.product_id = p.id
		 WHERE p.id IN (`+strings.Join(placeholders, ",")+`)`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entities := map[string]productRepo.Product{}
	formSeen := map[string]map[string]struct{}{}
	for rows.Next() {
		var (
			pid, name, formID string
			createdAt         time.Time
			updatedAt         time.Time
			categoryID        sql.NullString
			categoryName      sql.NullString
			formName          sql.NullString
		)
		if err = rows.Scan(&pid, &createdAt, &updatedAt, &name, &categoryID, &categoryName, &formID, &formName); err != nil {
			return nil, err
		}
		if _, ok := entities[pid]; !ok {
			var cat *productRepo.ProductCategory
			if categoryName.Valid {
				cat = &productRepo.ProductCategory{ID: categoryID.String, Name: categoryName.String}
			}
			entities[pid] = productRepo.Product{
				ID:         pid,
				CreatedAt:  createdAt,
				UpdatedAt:  updatedAt,
				Name:       name,
				CategoryID: categoryID,
				Category:   cat,
				Forms:      []productRepo.ProductForm{},
			}
			formSeen[pid] = map[string]struct{}{}
		}
		if formName.Valid {
			if _, ok := formSeen[pid][formName.String]; !ok {
				item := entities[pid]
				item.Forms = append(item.Forms, productRepo.ProductForm{ID: formID, ProductID: pid, Name: formName.String})
				entities[pid] = item
				formSeen[pid][formName.String] = struct{}{}
			}
		}
	}

	result := map[string]product.Product{}
	for pid, entity := range entities {
		result[pid] = productRepo.EntityToModel(entity)
	}
	return result, rows.Err()
}

func insertOrUpdateList(ctx context.Context, tx *sql.Tx, model list.ProductList) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO product_lists(id, status, updated_at, created_at, title)
		 VALUES(?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET status=excluded.status, updated_at=excluded.updated_at, created_at=excluded.created_at, title=excluded.title`,
		model.ID.String(), int32(model.Status), model.UpdatedAt.Time, model.CreatedAt.Time, model.Title,
	)
	if err != nil {
		return fmt.Errorf("can't update product list %s: %w", model.ID, err)
	}

	if _, err = tx.ExecContext(ctx, `DELETE FROM product_list_members WHERE list_id = ?`, model.ID.String()); err != nil {
		return fmt.Errorf("can't update members of list %s: %w", model.ID, err)
	}
	for _, member := range model.Members {
		if _, err = tx.ExecContext(ctx,
			`INSERT INTO product_list_members(id, user_id, list_id, created_at, updated_at, member_type)
			 VALUES(?, ?, ?, ?, ?, ?)`,
			uuid.NewString(),
			member.UserID.String(),
			model.ID.String(),
			member.CreatedAt.Time,
			member.UpdatedAt.Time,
			int32(member.Role),
		); err != nil {
			return err
		}
	}

	if _, err = tx.ExecContext(ctx, `DELETE FROM product_list_states WHERE list_id = ?`, model.ID.String()); err != nil {
		return fmt.Errorf("can't update states of list %s: %w", model.ID, err)
	}
	for index, state := range model.States {
		replacement := state.Replacement.ToPointer()
		var replacementProductID *string
		if replacement != nil {
			tmp := replacement.Product.ID.String()
			replacementProductID = &tmp
		}
		if _, err = tx.ExecContext(ctx,
			`INSERT INTO product_list_states(
				id, product_id, list_id, created_at, updated_at, "index", count, form_idx, status,
				replacement_count, replacement_form_idx, replacement_product_id
			) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			uuid.NewString(),
			state.Product.ID.String(),
			model.ID.String(),
			state.CreatedAt.Time,
			state.UpdatedAt.Time,
			index,
			state.Count.ToPointer(),
			state.FormIndex.ToPointer(),
			int(state.Status),
			lo.If(replacement == nil, (*int32)(nil)).Else(replacement.Count.ToPointer()),
			lo.If(replacement == nil, (*int32)(nil)).Else(replacement.FormIndex.ToPointer()),
			replacementProductID,
		); err != nil {
			return err
		}
	}

	return nil
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
