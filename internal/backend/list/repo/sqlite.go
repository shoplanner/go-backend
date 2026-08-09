package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"github.com/samber/mo"

	"go-backend/internal/backend/list"
	"go-backend/internal/backend/list/repo/sqlgen"
	"go-backend/internal/backend/product"
	productRepo "go-backend/internal/backend/product/repo"
	"go-backend/internal/backend/user"
	userRepo "go-backend/internal/backend/user/repo"
	"go-backend/pkg/date"
	"go-backend/pkg/god"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
	"go-backend/pkg/mysqlite"
)

//go:generate python $SQLC_HELPER

// createIndexQuery duplicates the CREATE UNIQUE INDEX in schema.sql, because sqlc generates no
// query for a CREATE INDEX statement. It is what makes a duplicate member a storage error.
const createIndexQuery = `CREATE UNIQUE INDEX IF NOT EXISTS idx_list_user ` +
	`ON product_list_members(user_id, list_id)`

// The three product shapes the read path needs. A state's product is rendered in full; its
// replacement is rendered without forms, because GORM only ever preloaded the category there.
//
//nolint:gochecknoglobals // immutable option sets, spelled out once instead of at every call
var (
	stateProductLoad       = productRepo.LoadOptions{WithCategory: true, WithForms: true}
	replacementProductLoad = productRepo.LoadOptions{WithCategory: true, WithForms: false}
)

type Repo struct {
	queries *sqlgen.Queries
	db      *sql.DB
}

func NewRepo(ctx context.Context, db *sql.DB) (*Repo, error) {
	queries := sqlgen.New(db)

	if err := queries.InitProductLists(ctx); err != nil {
		return nil, wrapErr(fmt.Errorf("can't init product lists table: %w", err))
	} else if err = queries.InitProductListMembers(ctx); err != nil {
		return nil, wrapErr(fmt.Errorf("can't init product list members table: %w", err))
	} else if err = queries.InitProductListStates(ctx); err != nil {
		return nil, wrapErr(fmt.Errorf("can't init product list states table: %w", err))
	}

	if _, err := db.ExecContext(ctx, createIndexQuery); err != nil {
		return nil, wrapErr(fmt.Errorf("can't init product list members index: %w", err))
	}

	return &Repo{queries: queries, db: db}, nil
}

func (r *Repo) CreateList(ctx context.Context, model list.ProductList) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault, ReadOnly: false})
	if err != nil {
		return wrapErr(fmt.Errorf("can't start transaction: %w", err))
	}

	defer func() { checkRollback(tx.Rollback()) }()

	qtx := r.queries.WithTx(tx)

	err = qtx.InsertList(ctx, sqlgen.InsertListParams{
		ID:        model.ID.String(),
		Status:    int64(model.Status),
		UpdatedAt: model.UpdatedAt.Time,
		CreatedAt: model.CreatedAt.Time,
		Title:     model.Title,
	})
	if err != nil {
		return wrapErr(fmt.Errorf("can't insert new product list %s: %w", model.ID, err))
	}

	if err = insertMembers(ctx, qtx, model.ID, model.Members); err != nil {
		return wrapErr(fmt.Errorf("can't insert new product list %s: %w", model.ID, err))
	}

	if err = insertStates(ctx, qtx, model.ID, model.States); err != nil {
		return wrapErr(fmt.Errorf("can't insert new product list %s: %w", model.ID, err))
	}

	return wrapErr(tx.Commit())
}

func (r *Repo) GetByListID(ctx context.Context, listID id.ID[list.ProductList]) (list.ProductList, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault, ReadOnly: true})
	if err != nil {
		return list.ProductList{}, wrapErr(fmt.Errorf("can't start transaction: %w", err))
	}

	defer func() { checkRollback(tx.Rollback()) }()

	model, err := getProductList(ctx, tx, r.queries.WithTx(tx), listID)
	if err != nil {
		return model, err
	}

	return model, wrapErr(tx.Commit())
}

func (r *Repo) GetListMetaByUserID(ctx context.Context, userID id.ID[user.User]) ([]list.ProductList, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault, ReadOnly: true})
	if err != nil {
		return nil, wrapErr(fmt.Errorf("can't start transaction: %w", err))
	}

	defer func() { checkRollback(tx.Rollback()) }()

	qtx := r.queries.WithTx(tx)

	listIDs, err := qtx.GetListIDListByUserID(ctx, userID.String())
	if err != nil {
		return nil, wrapErr(fmt.Errorf("can't find ids of lists related to user %s: %w", userID, err))
	}

	rows, err := qtx.GetListsByIDList(ctx, listIDs)
	if err != nil {
		return nil, wrapErr(fmt.Errorf("can't select lists related to user %s: %w", userID, err))
	}

	models, err := assemble(ctx, tx, qtx, rows)
	if err != nil {
		return nil, wrapErr(fmt.Errorf("can't select lists related to user %s: %w", userID, err))
	}

	ordered := lo.Map(rows, func(row sqlgen.ProductList, _ int) list.ProductList { return models[row.ID] })

	return ordered, wrapErr(tx.Commit())
}

func (r *Repo) GetAndUpdate(
	ctx context.Context,
	listID id.ID[list.ProductList],
	updateFunc func(list.ProductList) (list.ProductList, error),
) (
	list.ProductList,
	error,
) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault, ReadOnly: false})
	if err != nil {
		return list.ProductList{}, wrapErr(fmt.Errorf("can't start transaction: %w", err))
	}

	defer func() { checkRollback(tx.Rollback()) }()

	qtx := r.queries.WithTx(tx)

	model, err := getProductList(ctx, tx, qtx, listID)
	if err != nil {
		return list.ProductList{}, err
	}

	if model, err = updateFunc(model); err != nil {
		return list.ProductList{}, err
	}

	if err = qtx.DeleteMembersByListID(ctx, listID.String()); err != nil {
		return list.ProductList{}, typedErr(fmt.Errorf("can't clear members of list %s: %w", listID, err))
	}

	if err = qtx.DeleteStatesByListID(ctx, listID.String()); err != nil {
		return list.ProductList{}, typedErr(fmt.Errorf("can't clear states of list %s: %w", listID, err))
	}

	if err = insertMembers(ctx, qtx, listID, model.Members); err != nil {
		return list.ProductList{}, typedErr(fmt.Errorf("can't update members of list %s: %w", listID, err))
	}

	if err = insertStates(ctx, qtx, listID, model.States); err != nil {
		return list.ProductList{}, typedErr(fmt.Errorf("can't update states of list %s: %w", listID, err))
	}

	err = qtx.UpdateList(ctx, sqlgen.UpdateListParams{
		Status:    int64(model.Status),
		UpdatedAt: model.UpdatedAt.Time,
		CreatedAt: model.CreatedAt.Time,
		Title:     model.Title,
		ID:        listID.String(),
	})
	if err != nil {
		return list.ProductList{}, typedErr(fmt.Errorf("can't update product list %s: %w", listID, err))
	}

	// The caller gets a re-read rather than the model it just handed in, so the states come
	// back in stored order and the members carry the logins the write path never set.
	if model, err = getProductList(ctx, tx, qtx, listID); err != nil {
		return list.ProductList{}, err
	}

	return model, wrapErr(tx.Commit())
}

func (r *Repo) GetAndDeleteList(
	ctx context.Context,
	listID id.ID[list.ProductList],
	validateFunc func(list.ProductList) error,
) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault, ReadOnly: false})
	if err != nil {
		return wrapErr(fmt.Errorf("can't start transaction: %w", err))
	}

	defer func() { checkRollback(tx.Rollback()) }()

	qtx := r.queries.WithTx(tx)

	model, err := getProductList(ctx, tx, qtx, listID)
	if err != nil {
		return err
	}

	if err = validateFunc(model); err != nil {
		return err
	}

	// Only the list row is removed. Its members and states are left behind, which is what the
	// declared ON DELETE CASCADE would have prevented if foreign keys were enabled;
	// TestListDeleteLeavesOrphanRows_CurrentBehaviour pins the fact that they are not.
	if err = qtx.DeleteList(ctx, listID.String()); err != nil {
		return wrapErr(fmt.Errorf("can't delete product list %s: %w", listID, err))
	}

	return wrapErr(tx.Commit())
}

func (r *Repo) ApplyOrder(
	ctx context.Context,
	validateFunc list.RoleCheckFunc,
	listID id.ID[list.ProductList],
	ids []id.ID[product.Product],
) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault, ReadOnly: false})
	if err != nil {
		return wrapErr(fmt.Errorf("can't start transaction: %w", err))
	}

	defer func() { checkRollback(tx.Rollback()) }()

	qtx := r.queries.WithTx(tx)

	members, err := getMembers(ctx, tx, qtx, listID)
	if err != nil {
		return wrapErr(fmt.Errorf("failed to get product list members: %w", err))
	}

	if err = validateFunc(members); err != nil {
		return err
	}

	if len(ids) == 0 {
		return fmt.Errorf("%w: order list is empty", myerr.ErrInvalidArgument)
	}

	// A single CASE-per-product UPDATE, which has no sqlc equivalent because the statement's
	// shape depends on how many ids there are.
	args := make([]any, 0, 3*len(ids)+1)
	for index, productID := range ids {
		args = append(args, productID.String(), index)
	}

	for _, productID := range ids {
		args = append(args, productID.String())
	}

	args = append(args, listID.String())

	if _, err = tx.ExecContext(ctx, buildApplyOrderQuery(ids), args...); err != nil {
		return wrapErr(fmt.Errorf("can't apply order: %w", err))
	}

	return wrapErr(tx.Commit())
}

// buildApplyOrderQuery writes `index` only for the products it is given and leaves every other
// row of the list alone. A partial reorder therefore produces duplicate index values; that is
// pinned by TestListReorderStatesPartial_CurrentBehaviour.
func buildApplyOrderQuery(ids []id.ID[product.Product]) string {
	var builder strings.Builder

	builder.WriteString(`UPDATE product_list_states SET "index" = CASE product_id `)

	for range ids {
		builder.WriteString("WHEN ? THEN ? ")
	}

	builder.WriteString("END WHERE product_id in (")
	builder.WriteString(strings.TrimSuffix(strings.Repeat("?, ", len(ids)), ", "))
	builder.WriteString(") AND list_id = ?")

	return builder.String()
}

// getProductList reads one list with everything hanging off it.
//
// A list that does not exist is *not* an error: it comes back as a zero-valued model carrying
// the requested id, which is what GORM's Find (rather than First) produced. The service then
// runs its role check against an empty member list and reports ErrForbidden where ErrNotFound
// is meant — see TestListGetMissing_CurrentBehaviour.
func getProductList(
	ctx context.Context,
	tx *sql.Tx,
	qtx *sqlgen.Queries,
	listID id.ID[list.ProductList],
) (
	list.ProductList,
	error,
) {
	row, err := qtx.GetListByID(ctx, listID.String())
	if errors.Is(err, sql.ErrNoRows) {
		return emptyList(listID), nil
	} else if err != nil {
		return list.ProductList{}, wrapErr(fmt.Errorf("can't select product list %s: %w", listID, err))
	}

	models, err := assemble(ctx, tx, qtx, []sqlgen.ProductList{row})
	if err != nil {
		return list.ProductList{}, wrapErr(fmt.Errorf("can't select product list %s: %w", listID, err))
	}

	return models[row.ID], nil
}

func emptyList(listID id.ID[list.ProductList]) list.ProductList {
	return list.ProductList{
		ListOptions: list.ListOptions{Status: 0, Title: ""},
		States:      []list.ProductState{},
		Members:     []list.Member{},
		ID:          listID,
		UpdatedAt:   date.UpdateDate[list.ProductList]{Time: time.Time{}},
		CreatedAt:   date.CreateDate[list.ProductList]{Time: time.Time{}},
	}
}

func getMembers(
	ctx context.Context,
	tx *sql.Tx,
	qtx *sqlgen.Queries,
	listID id.ID[list.ProductList],
) (
	[]list.Member,
	error,
) {
	rows, err := qtx.GetMembersByListIDList(ctx, []string{listID.String()})
	if err != nil {
		return nil, fmt.Errorf("failed to query members of list %s: %w", listID, err)
	}

	logins, err := userRepo.LoadLogins(ctx, tx, lo.Map(rows, func(row sqlgen.ProductListMember, _ int) string {
		return row.UserID
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to query logins of list %s members: %w", listID, err)
	}

	return lo.Map(rows, func(row sqlgen.ProductListMember, _ int) list.Member {
		return memberToModel(row, logins[row.UserID])
	}), nil
}

// assemble loads the members and states of every list in rows, together with the users and
// products they point at, and builds the domain models. Everything runs inside the caller's
// transaction, which is what GORM's nested Preloads did.
func assemble(
	ctx context.Context,
	tx *sql.Tx,
	qtx *sqlgen.Queries,
	rows []sqlgen.ProductList,
) (
	map[string]list.ProductList,
	error,
) {
	models := make(map[string]list.ProductList, len(rows))
	if len(rows) == 0 {
		return models, nil
	}

	listIDs := lo.Map(rows, func(row sqlgen.ProductList, _ int) string { return row.ID })

	memberRows, err := qtx.GetMembersByListIDList(ctx, listIDs)
	if err != nil {
		return nil, fmt.Errorf("can't select product list members: %w", err)
	}

	stateRows, err := qtx.GetStatesByListIDList(ctx, listIDs)
	if err != nil {
		return nil, fmt.Errorf("can't select product list states: %w", err)
	}

	logins, err := userRepo.LoadLogins(ctx, tx, lo.Map(memberRows, func(row sqlgen.ProductListMember, _ int) string {
		return row.UserID
	}))
	if err != nil {
		return nil, fmt.Errorf("can't select product list member logins: %w", err)
	}

	products, err := productRepo.Load(ctx, tx, lo.Uniq(lo.Map(stateRows,
		func(row sqlgen.ProductListState, _ int) string { return row.ProductID },
	)), stateProductLoad)
	if err != nil {
		return nil, fmt.Errorf("can't select products of product list states: %w", err)
	}

	replacements, err := productRepo.Load(ctx, tx, lo.Uniq(lo.FilterMap(stateRows,
		func(row sqlgen.ProductListState, _ int) (string, bool) {
			return row.ReplacementProductID.String, row.ReplacementProductID.Valid
		},
	)), replacementProductLoad)
	if err != nil {
		return nil, fmt.Errorf("can't select replacement products of product list states: %w", err)
	}

	membersByList := lo.GroupBy(memberRows, func(row sqlgen.ProductListMember) string { return row.ListID })
	statesByList := lo.GroupBy(stateRows, func(row sqlgen.ProductListState) string { return row.ListID })

	for _, row := range rows {
		models[row.ID] = rowToModel(row, membersByList[row.ID], statesByList[row.ID], logins, products, replacements)
	}

	return models, nil
}

func rowToModel(
	row sqlgen.ProductList,
	memberRows []sqlgen.ProductListMember,
	stateRows []sqlgen.ProductListState,
	logins map[string]user.Login,
	products, replacements map[string]product.Product,
) list.ProductList {
	// States are placed at the index stored on the row rather than appended, so a corrupted
	// `index` column silently drops entries instead of reordering them. Keep this as it is:
	// TestListReorderStatesPartial_CurrentBehaviour asserts exactly that outcome.
	states := make([]list.ProductState, len(stateRows))
	for _, state := range stateRows {
		states[state.Index] = stateToModel(state, products, replacements)
	}

	return list.ProductList{
		States: states,
		Members: lo.Map(memberRows, func(item sqlgen.ProductListMember, _ int) list.Member {
			return memberToModel(item, logins[item.UserID])
		}),
		ListOptions: list.ListOptions{
			//nolint:gosec // status holds an ExecStatus enum, which has three values
			Status: list.ExecStatus(row.Status),
			Title:  row.Title,
		},
		ID:        id.ID[list.ProductList]{UUID: god.Believe(uuid.Parse(row.ID))},
		UpdatedAt: date.UpdateDate[list.ProductList]{Time: row.UpdatedAt},
		CreatedAt: date.CreateDate[list.ProductList]{Time: row.CreatedAt},
	}
}

func memberToModel(row sqlgen.ProductListMember, login user.Login) list.Member {
	return list.Member{
		MemberOptions: list.MemberOptions{
			UserID: id.ID[user.User]{UUID: god.Believe(uuid.Parse(row.UserID))},
			//nolint:gosec // member_type holds a MemberType enum, which has five values
			Role: list.MemberType(row.MemberType),
		},
		UserName:  login,
		CreatedAt: date.CreateDate[list.Member]{Time: row.CreatedAt},
		UpdatedAt: date.UpdateDate[list.Member]{Time: row.UpdatedAt},
	}
}

func stateToModel(
	row sqlgen.ProductListState,
	products, replacements map[string]product.Product,
) list.ProductState {
	var replacement *list.ProductStateReplacement
	if row.ReplacementProductID.Valid {
		replacement = &list.ProductStateReplacement{
			Count:     nullToOption(row.ReplacementCount),
			FormIndex: nullToOption(row.ReplacementFormIdx),
			Product:   replacements[row.ReplacementProductID.String],
		}
	}

	return list.ProductState{
		ProductStateOptions: list.ProductStateOptions{
			Count:       nullToOption(row.Count),
			FormIndex:   nullToOption(row.FormIdx),
			Status:      list.StateStatus(row.Status),
			Replacement: mo.PointerToOption(replacement),
		},
		Product:   products[row.ProductID],
		CreatedAt: date.CreateDate[list.ProductState]{Time: row.CreatedAt},
		UpdatedAt: date.UpdateDate[list.ProductState]{Time: row.UpdatedAt},
	}
}

// nullToOption keeps SQL NULL and a present zero apart, which is the whole point of the Option:
// "no count" and "count is 0" are different states and TestOptionalFieldsBecomeSQLNull says so.
func nullToOption(value sql.NullInt64) mo.Option[int32] {
	if !value.Valid {
		return mo.None[int32]()
	}

	//nolint:gosec // counts and form indexes were written from int32 in the first place
	return mo.Some(int32(value.Int64))
}

func optionToNull(value mo.Option[int32]) sql.NullInt64 {
	inner, present := value.Get()

	return sql.NullInt64{Int64: int64(inner), Valid: present}
}

// SQLite has no bulk-load statement, so rows go in one INSERT at a time; every caller already
// runs inside a transaction, which keeps this atomic.
func insertMembers(
	ctx context.Context,
	qtx *sqlgen.Queries,
	listID id.ID[list.ProductList],
	members []list.Member,
) error {
	for _, member := range members {
		err := qtx.InsertMember(ctx, sqlgen.InsertMemberParams{
			ID:         uuid.NewString(),
			UserID:     member.UserID.String(),
			ListID:     listID.String(),
			CreatedAt:  member.CreatedAt.Time,
			UpdatedAt:  member.UpdatedAt.Time,
			MemberType: int64(member.Role),
		})
		if err != nil {
			return fmt.Errorf("can't insert member %s: %w", member.UserID, err)
		}
	}

	return nil
}

func insertStates(
	ctx context.Context,
	qtx *sqlgen.Queries,
	listID id.ID[list.ProductList],
	states []list.ProductState,
) error {
	for index, state := range states {
		replacement := state.Replacement.OrEmpty()

		replacementProductID := sql.NullString{String: "", Valid: false}
		if state.Replacement.IsPresent() {
			replacementProductID = sql.NullString{String: replacement.Product.ID.String(), Valid: true}
		}

		err := qtx.InsertState(ctx, sqlgen.InsertStateParams{
			ID:                   uuid.NewString(),
			ProductID:            state.Product.ID.String(),
			ListID:               listID.String(),
			CreatedAt:            state.CreatedAt.Time,
			UpdatedAt:            state.UpdatedAt.Time,
			Index:                int64(index),
			Count:                optionToNull(state.Count),
			FormIdx:              optionToNull(state.FormIndex),
			Status:               int64(state.Status),
			ReplacementCount:     optionToNull(replacement.Count),
			ReplacementFormIdx:   optionToNull(replacement.FormIndex),
			ReplacementProductID: replacementProductID,
		})
		if err != nil {
			return fmt.Errorf("can't insert state of product %s: %w", state.Product.ID, err)
		}
	}

	return nil
}

func wrapErr(err error) error {
	if err != nil {
		return fmt.Errorf("product list storage: %w", err)
	}

	return nil
}

// typedErr additionally maps a driver constraint failure onto a myerr sentinel, so that a
// duplicate member surfaces as ErrAlreadyExists rather than a 500.
func typedErr(err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%w: %w", mysqlite.GetType(err), err)
}

func checkRollback(err error) {
	if wrapped := wrapErr(err); wrapped != nil && !errors.Is(err, sql.ErrTxDone) {
		log.Err(wrapped).Msg("rollback failed")
	}
}
