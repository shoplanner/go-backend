package repo

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/samber/mo"

	"go-backend/internal/backend/favorite"
	"go-backend/internal/backend/favorite/repo/sqlgen"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/god"
	"go-backend/pkg/id"
)

//go:generate python $SQLC_HELPER

type Repo struct {
	db      *sql.DB
	queries *sqlgen.Queries
}

func NewRepo(ctx context.Context, db *sql.DB) (*Repo, error) {
	q := sqlgen.New(db)

	if err := q.InitFavoriteLists(ctx); err != nil {
		return nil, fmt.Errorf("can't create favorites tables: %w", err)
	}
	if err := q.InitFavoriteMembers(ctx); err != nil {
		return nil, fmt.Errorf("can't create favorites tables: %w", err)
	}
	if err := q.InitFavoriteProducts(ctx); err != nil {
		return nil, fmt.Errorf("can't create favorites tables: %w", err)
	}

	return &Repo{db: db, queries: q}, nil
}

func (r *Repo) CreateList(ctx context.Context, model favorite.List) error {
	return withTx(ctx, r.db, func(qtx *sqlgen.Queries) error {
		if err := qtx.CreateFavoriteList(ctx, sqlgen.CreateFavoriteListParams{
			ID:        model.ID.String(),
			ListType:  int32(model.Type),
			CreatedAt: model.CreatedAt.Time,
			UpdatedAt: model.UpdatedAt.Time,
		}); err != nil {
			return fmt.Errorf("can't create new list %s: %w", model.ID, err)
		}

		for _, member := range model.Members {
			if err := qtx.CreateFavoriteMember(ctx, sqlgen.CreateFavoriteMemberParams{
				ID:             uuid.NewString(),
				UserID:         member.UserID.String(),
				FavoriteListID: model.ID.String(),
				CreatedAt:      member.CreatedAt.Time,
				UpdatedAt:      member.UpdatedAt.Time,
				MemberType:     int32(member.Type),
			}); err != nil {
				return err
			}
		}

		for _, p := range model.Products {
			if err := qtx.CreateFavoriteProduct(ctx, sqlgen.CreateFavoriteProductParams{
				ID:             uuid.NewString(),
				ProductID:      p.Product.ID.String(),
				FavoriteListID: model.ID.String(),
				CreatedAt:      p.CreatedAt.Time,
				UpdatedAt:      p.UpdatedAt.Time,
			}); err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *Repo) DeleteList(ctx context.Context, listID id.ID[favorite.List]) error {
	return withTx(ctx, r.db, func(qtx *sqlgen.Queries) error {
		if err := qtx.DeleteFavoriteMembersByListID(ctx, listID.String()); err != nil {
			return err
		}
		if err := qtx.DeleteFavoriteProductsByListID(ctx, listID.String()); err != nil {
			return err
		}
		if err := qtx.DeleteFavoriteList(ctx, listID.String()); err != nil {
			return fmt.Errorf("can't delete favorites list %s: %w", listID, err)
		}
		return nil
	})
}

func (r *Repo) GetByID(ctx context.Context, listID id.ID[favorite.List]) (favorite.List, error) {
	return r.getByID(ctx, r.queries, listID)
}

func (r *Repo) GetByUserID(ctx context.Context, userID id.ID[user.User]) ([]favorite.List, error) {
	listIDs, err := r.queries.GetFavoriteListIDsByUserID(ctx, userID.String())
	if err != nil {
		return nil, fmt.Errorf("can't get lists of user %s: %w", userID, err)
	}

	result := make([]favorite.List, 0, len(listIDs))
	for _, listIDRaw := range listIDs {
		model, getErr := r.GetByID(ctx, id.ID[favorite.List]{UUID: god.Believe(uuid.Parse(listIDRaw))})
		if getErr != nil {
			return nil, getErr
		}
		result = append(result, model)
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
	err := withTx(ctx, r.db, func(qtx *sqlgen.Queries) error {
		var err error
		model, err = r.getByID(ctx, qtx, listID)
		if err != nil {
			return err
		}

		model, err = f(model)
		if err != nil {
			return err
		}

		if err = qtx.UpdateFavoriteList(ctx, sqlgen.UpdateFavoriteListParams{
			ListType:  int32(model.Type),
			CreatedAt: model.CreatedAt.Time,
			UpdatedAt: model.UpdatedAt.Time,
			ID:        listID.String(),
		}); err != nil {
			return err
		}

		if err = qtx.DeleteFavoriteMembersByListID(ctx, listID.String()); err != nil {
			return err
		}
		for _, member := range model.Members {
			if err = qtx.CreateFavoriteMember(ctx, sqlgen.CreateFavoriteMemberParams{
				ID:             uuid.NewString(),
				UserID:         member.UserID.String(),
				FavoriteListID: listID.String(),
				CreatedAt:      member.CreatedAt.Time,
				UpdatedAt:      member.UpdatedAt.Time,
				MemberType:     int32(member.Type),
			}); err != nil {
				return err
			}
		}

		if err = qtx.DeleteFavoriteProductsByListID(ctx, listID.String()); err != nil {
			return err
		}
		for _, p := range model.Products {
			if err = qtx.CreateFavoriteProduct(ctx, sqlgen.CreateFavoriteProductParams{
				ID:             uuid.NewString(),
				ProductID:      p.Product.ID.String(),
				FavoriteListID: listID.String(),
				CreatedAt:      p.CreatedAt.Time,
				UpdatedAt:      p.UpdatedAt.Time,
			}); err != nil {
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
	q interface {
		GetFavoriteListByID(context.Context, string) (sqlgen.FavoriteList, error)
		GetFavoriteMembersByListID(context.Context, string) ([]sqlgen.GetFavoriteMembersByListIDRow, error)
		GetFavoriteProductsByListID(context.Context, string) ([]sqlgen.GetFavoriteProductsByListIDRow, error)
		LoadProductsByIDs(context.Context, []string) ([]sqlgen.LoadProductsByIDsRow, error)
	},
	listID id.ID[favorite.List],
) (favorite.List, error) {
	listDAO, err := q.GetFavoriteListByID(ctx, listID.String())
	if err != nil {
		return favorite.List{}, fmt.Errorf("can't select favorites list %s: %w", listID, err)
	}

	model := favorite.List{
		ID:        id.ID[favorite.List]{UUID: god.Believe(uuid.Parse(listDAO.ID))},
		Type:      favorite.ListType(listDAO.ListType),
		CreatedAt: date.CreateDate[favorite.List]{Time: listDAO.CreatedAt},
		UpdatedAt: date.UpdateDate[favorite.List]{Time: listDAO.UpdatedAt},
	}

	members, err := q.GetFavoriteMembersByListID(ctx, listID.String())
	if err != nil {
		return favorite.List{}, err
	}
	model.Members = lo.Map(members, func(item sqlgen.GetFavoriteMembersByListIDRow, _ int) favorite.Member {
		return favorite.Member{
			UserID:    id.ID[user.User]{UUID: god.Believe(uuid.Parse(item.UserID))},
			Type:      favorite.MemberType(item.MemberType),
			CreatedAt: date.CreateDate[favorite.Member]{Time: item.CreatedAt},
			UpdatedAt: date.UpdateDate[favorite.Member]{Time: item.UpdatedAt},
		}
	})

	productsDAO, err := q.GetFavoriteProductsByListID(ctx, listID.String())
	if err != nil {
		return favorite.List{}, err
	}

	productMeta := map[string]favorite.Favorite{}
	ids := []string{}
	for _, item := range productsDAO {
		productMeta[item.ProductID] = favorite.Favorite{
			CreatedAt: date.CreateDate[favorite.Favorite]{Time: item.CreatedAt},
			UpdatedAt: date.UpdateDate[favorite.Favorite]{Time: item.UpdatedAt},
		}
		ids = append(ids, item.ProductID)
	}

	productsMap, err := loadProducts(ctx, q, ids)
	if err != nil {
		return favorite.List{}, err
	}

	sort.Strings(ids)
	model.Products = make([]favorite.Favorite, 0, len(ids))
	for _, pid := range ids {
		item := productMeta[pid]
		item.Product = productsMap[pid]
		model.Products = append(model.Products, item)
	}

	return model, nil
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

	type aggregate struct {
		product product.Product
		forms   map[string]struct{}
	}
	agg := map[string]*aggregate{}

	for _, row := range rows {
		if _, ok := agg[row.ID]; !ok {
			cat := mo.None[product.Category]()
			if row.CategoryName.Valid {
				cat = mo.Some(product.Category(row.CategoryName.String))
			}
			agg[row.ID] = &aggregate{product: product.Product{
				Options: product.Options{Name: product.Name(row.Name), Category: cat, Forms: []product.Form{}},
				ID:      id.ID[product.Product]{UUID: god.Believe(uuid.Parse(row.ID))},
				CreatedAt: date.CreateDate[product.Product]{
					Time: row.CreatedAt,
				},
				UpdatedAt: date.UpdateDate[product.Product]{
					Time: row.UpdatedAt,
				},
			}, forms: map[string]struct{}{}}
		}

		if row.FormName.Valid {
			if _, exists := agg[row.ID].forms[row.FormName.String]; !exists {
				agg[row.ID].forms[row.FormName.String] = struct{}{}
				agg[row.ID].product.Forms = append(agg[row.ID].product.Forms, product.Form(row.FormName.String))
			}
		}
	}

	result := make(map[string]product.Product, len(agg))
	for key, value := range agg {
		result[key] = value.product
	}
	return result, nil
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
