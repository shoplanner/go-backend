package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/samber/mo"

	"go-backend/internal/backend/list"
	"go-backend/internal/backend/list/repo/sqlgen"
	"go-backend/internal/backend/product"
	productRepo "go-backend/internal/backend/product/repo"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/god"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
	"go-backend/pkg/mymysql"
)

//go:generate python $SQLC_HELPER

type Repo struct {
	db      *sql.DB
	queries *sqlgen.Queries
}

func NewRepo(ctx context.Context, db *sql.DB) (*Repo, error) {
	q := sqlgen.New(db)

	if err := q.InitProductLists(ctx); err != nil {
		return nil, fmt.Errorf("can't create product list tables: %w", err)
	}
	if err := q.InitProductListMembers(ctx); err != nil {
		return nil, fmt.Errorf("can't create product list tables: %w", err)
	}
	if err := q.InitProductListStates(ctx); err != nil {
		return nil, fmt.Errorf("can't create product list tables: %w", err)
	}

	return &Repo{db: db, queries: q}, nil
}

func (r *Repo) GetListMetaByUserID(ctx context.Context, userID id.ID[user.User]) ([]list.ProductList, error) {
	ids, err := r.queries.GetProductListIDsByUserID(ctx, userID.String())
	if err != nil {
		return nil, fmt.Errorf("can't find ids of lists related to user %s: %w", userID, err)
	}

	result := make([]list.ProductList, 0, len(ids))
	for _, listIDRaw := range ids {
		item, getErr := r.GetByListID(ctx, id.ID[list.ProductList]{UUID: god.Believe(uuid.Parse(listIDRaw))})
		if getErr != nil {
			return nil, getErr
		}
		result = append(result, item)
	}

	return result, nil
}

func (r *Repo) GetByListID(ctx context.Context, listID id.ID[list.ProductList]) (list.ProductList, error) {
	return r.getProductList(ctx, r.queries, listID)
}

func (r *Repo) CreateList(ctx context.Context, model list.ProductList) error {
	err := withTx(ctx, r.db, func(qtx *sqlgen.Queries) error {
		return insertOrUpdateList(ctx, qtx, model)
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
	err := withTx(ctx, r.db, func(qtx *sqlgen.Queries) error {
		var err error

		model, err = r.getProductList(ctx, qtx, listID)
		if err != nil {
			return err
		}

		model, err = updateFunc(model)
		if err != nil {
			return err
		}

		if err = insertOrUpdateList(ctx, qtx, model); err != nil {
			return err
		}

		model, err = r.getProductList(ctx, qtx, listID)
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
	err := withTx(ctx, r.db, func(qtx *sqlgen.Queries) error {
		model, err := r.getProductList(ctx, qtx, listID)
		if err != nil {
			return err
		}

		if err = validateFunc(model); err != nil {
			return err
		}

		if err = qtx.DeleteProductListMembersByListID(ctx, listID.String()); err != nil {
			return err
		}
		if err = qtx.DeleteProductListStatesByListID(ctx, listID.String()); err != nil {
			return err
		}
		if err = qtx.DeleteProductListByID(ctx, listID.String()); err != nil {
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
	err := withTx(ctx, r.db, func(qtx *sqlgen.Queries) error {
		members, err := r.getMembers(ctx, qtx, listID)
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
			if err = qtx.UpdateStateIndexByProductID(ctx, sqlgen.UpdateStateIndexByProductIDParams{
				Index:     int32(idx),
				ListID:    listID.String(),
				ProductID: productID.String(),
			}); err != nil {
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
	q interface {
		GetMembersByListID(context.Context, string) ([]sqlgen.GetMembersByListIDRow, error)
	},
	listID id.ID[list.ProductList],
) ([]list.Member, error) {
	rows, err := q.GetMembersByListID(ctx, listID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to query members of list %s: %w", listID, err)
	}

	return lo.Map(rows, func(item sqlgen.GetMembersByListIDRow, _ int) list.Member {
		return list.Member{
			MemberOptions: list.MemberOptions{UserID: id.ID[user.User]{UUID: god.Believe(uuid.Parse(item.UserID))}, Role: list.MemberType(item.MemberType)},
			UserName:      user.Login(item.Login),
			CreatedAt:     date.CreateDate[list.Member]{Time: item.CreatedAt},
			UpdatedAt:     date.UpdateDate[list.Member]{Time: item.UpdatedAt},
		}
	}), nil
}

func (r *Repo) getProductList(
	ctx context.Context,
	q interface {
		GetProductListByID(context.Context, string) (sqlgen.ProductList, error)
		GetMembersByListID(context.Context, string) ([]sqlgen.GetMembersByListIDRow, error)
		GetStatesByListID(context.Context, string) ([]sqlgen.GetStatesByListIDRow, error)
		LoadProductsByIDs(context.Context, []string) ([]sqlgen.LoadProductsByIDsRow, error)
	},
	listID id.ID[list.ProductList],
) (list.ProductList, error) {
	entity, err := q.GetProductListByID(ctx, listID.String())
	if err != nil {
		return list.ProductList{}, fmt.Errorf("can't select product list %s: %w", listID, err)
	}

	members, err := r.getMembers(ctx, q, listID)
	if err != nil {
		return list.ProductList{}, err
	}

	statesRaw, err := q.GetStatesByListID(ctx, listID.String())
	if err != nil {
		return list.ProductList{}, err
	}

	productIDs := make([]string, 0, len(statesRaw))
	replacementIDs := make([]string, 0, len(statesRaw))
	for _, s := range statesRaw {
		productIDs = append(productIDs, s.ProductID)
		if s.ReplacementProductID.Valid {
			replacementIDs = append(replacementIDs, s.ReplacementProductID.String)
		}
	}

	allIDs := lo.Uniq(append(productIDs, replacementIDs...))
	productMap, err := loadProducts(ctx, q, allIDs)
	if err != nil {
		return list.ProductList{}, err
	}

	states := make([]list.ProductState, len(statesRaw))
	for _, s := range statesRaw {
		state := list.ProductState{
			ProductStateOptions: list.ProductStateOptions{
				Count:     mo.PointerToOption(lo.Ternary(s.Count.Valid, lo.ToPtr(s.Count.Int32), (*int32)(nil))),
				FormIndex: mo.PointerToOption(lo.Ternary(s.FormIdx.Valid, lo.ToPtr(s.FormIdx.Int32), (*int32)(nil))),
				Status:    list.StateStatus(s.Status),
			},
			Product:   productMap[s.ProductID],
			CreatedAt: date.CreateDate[list.ProductState]{Time: s.CreatedAt},
			UpdatedAt: date.UpdateDate[list.ProductState]{Time: s.UpdatedAt},
		}
		if s.ReplacementProductID.Valid {
			repl := &list.ProductStateReplacement{
				Count:     mo.PointerToOption(lo.Ternary(s.ReplacementCount.Valid, lo.ToPtr(s.ReplacementCount.Int32), (*int32)(nil))),
				FormIndex: mo.PointerToOption(lo.Ternary(s.ReplacementFormIdx.Valid, lo.ToPtr(s.ReplacementFormIdx.Int32), (*int32)(nil))),
				Product:   productMap[s.ReplacementProductID.String],
			}
			state.Replacement = mo.PointerToOption(repl)
		}
		if int(s.Index) >= len(states) {
			return list.ProductList{}, fmt.Errorf("invalid state index %d for list %s", s.Index, listID)
		}
		states[s.Index] = state
	}

	return list.ProductList{
		ID:          id.ID[list.ProductList]{UUID: god.Believe(uuid.Parse(entity.ID))},
		ListOptions: list.ListOptions{Status: list.ExecStatus(entity.Status), Title: entity.Title},
		CreatedAt:   date.CreateDate[list.ProductList]{Time: entity.CreatedAt},
		UpdatedAt:   date.UpdateDate[list.ProductList]{Time: entity.UpdatedAt},
		Members:     members,
		States:      states,
	}, nil
}

func loadProducts(
	ctx context.Context,
	q interface {
		LoadProductsByIDs(context.Context, []string) ([]sqlgen.LoadProductsByIDsRow, error)
	},
	ids []string,
) (map[string]product.Product, error) {
	if len(ids) == 0 {
		return map[string]product.Product{}, nil
	}

	rows, err := q.LoadProductsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	entities := map[string]productRepo.Product{}
	formSeen := map[string]map[string]struct{}{}
	for _, row := range rows {
		if _, ok := entities[row.ID]; !ok {
			var cat *productRepo.ProductCategory
			if row.CategoryName.Valid {
				cat = &productRepo.ProductCategory{ID: row.CategoryID.String, Name: row.CategoryName.String}
			}
			entities[row.ID] = productRepo.Product{
				ID:         row.ID,
				CreatedAt:  date.CreateDate[product.Product]{Time: row.CreatedAt},
				UpdatedAt:  date.UpdateDate[product.Product]{Time: row.UpdatedAt},
				Name:       product.Name(row.Name),
				CategoryID: row.CategoryID,
				Category:   cat,
				Forms:      []productRepo.ProductForm{},
			}
			formSeen[row.ID] = map[string]struct{}{}
		}
		if row.FormName.Valid {
			if _, ok := formSeen[row.ID][row.FormName.String]; !ok {
				item := entities[row.ID]
				item.Forms = append(item.Forms, productRepo.ProductForm{ID: row.FormID.String, ProductID: row.ID, Name: row.FormName.String})
				entities[row.ID] = item
				formSeen[row.ID][row.FormName.String] = struct{}{}
			}
		}
	}

	result := map[string]product.Product{}
	for pid, entity := range entities {
		result[pid] = productRepo.EntityToModel(entity)
	}
	return result, nil
}

func insertOrUpdateList(ctx context.Context, q *sqlgen.Queries, model list.ProductList) error {
	if err := q.UpsertProductList(ctx, sqlgen.UpsertProductListParams{
		ID:        model.ID.String(),
		Status:    int32(model.Status),
		UpdatedAt: model.UpdatedAt.Time,
		CreatedAt: model.CreatedAt.Time,
		Title:     model.Title,
	}); err != nil {
		return fmt.Errorf("can't update product list %s: %w", model.ID, err)
	}

	if err := q.DeleteProductListMembersByListID(ctx, model.ID.String()); err != nil {
		return fmt.Errorf("can't update members of list %s: %w", model.ID, err)
	}
	for _, member := range model.Members {
		if err := q.InsertProductListMember(ctx, sqlgen.InsertProductListMemberParams{
			ID:         uuid.NewString(),
			UserID:     member.UserID.String(),
			ListID:     model.ID.String(),
			CreatedAt:  member.CreatedAt.Time,
			UpdatedAt:  member.UpdatedAt.Time,
			MemberType: int32(member.Role),
		}); err != nil {
			return err
		}
	}

	if err := q.DeleteProductListStatesByListID(ctx, model.ID.String()); err != nil {
		return fmt.Errorf("can't update states of list %s: %w", model.ID, err)
	}
	for index, state := range model.States {
		replacement := state.Replacement.ToPointer()
		var replacementProductID sql.NullString
		if replacement != nil {
			replacementProductID = sql.NullString{String: replacement.Product.ID.String(), Valid: true}
		}
		if err := q.InsertProductListState(ctx, sqlgen.InsertProductListStateParams{
			ID:                   uuid.NewString(),
			ProductID:            state.Product.ID.String(),
			ListID:               model.ID.String(),
			CreatedAt:            state.CreatedAt.Time,
			UpdatedAt:            state.UpdatedAt.Time,
			Index:                int32(index),
			Count:                toNullInt32(state.Count.ToPointer()),
			FormIdx:              toNullInt32(state.FormIndex.ToPointer()),
			Status:               int32(state.Status),
			ReplacementCount:     toNullInt32(lo.If(replacement == nil, (*int32)(nil)).Else(replacement.Count.ToPointer())),
			ReplacementFormIdx:   toNullInt32(lo.If(replacement == nil, (*int32)(nil)).Else(replacement.FormIndex.ToPointer())),
			ReplacementProductID: replacementProductID,
		}); err != nil {
			return err
		}
	}

	return nil
}

func toNullInt32(ptr *int32) sql.NullInt32 {
	if ptr == nil {
		return sql.NullInt32{}
	}
	return sql.NullInt32{Int32: *ptr, Valid: true}
}

func withTx(ctx context.Context, db *sql.DB, f func(*sqlgen.Queries) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := sqlgen.New(tx)
	if err = f(qtx); err != nil {
		return err
	}

	return tx.Commit()
}
