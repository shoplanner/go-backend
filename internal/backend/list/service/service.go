package service

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/samber/lo"

	"go-backend/internal/backend/list"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/deepcopy"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

type repo interface {
	CreateList(context.Context, list.ProductList) error
	GetByListID(context.Context, id.ID[list.ProductList]) (list.ProductList, error)
	GetListMetaByUserID(context.Context, id.ID[user.User]) ([]list.ProductList, error)

	GetAndUpdate(
		context.Context,
		id.ID[list.ProductList],
		func(list.ProductList) (list.ProductList, error),
	) (
		list.ProductList,
		error,
	)

	GetAndDeleteList(context.Context, id.ID[list.ProductList], func(list.ProductList) error) error
	ApplyOrder(context.Context, list.RoleCheckFunc, id.ID[list.ProductList], []id.ID[product.Product]) error
}

type Service struct {
	channels     map[providerID]*eventProvider
	channelsLock sync.RWMutex
	repo         repo
	log          zerolog.Logger
}

func NewService(repo repo, log zerolog.Logger) *Service {
	return &Service{
		log:          log.With().Str("component", "product list service").Logger(),
		repo:         repo,
		channels:     map[providerID]*eventProvider{},
		channelsLock: sync.RWMutex{},
	}
}

func (s *Service) ReoderStates(
	ctx *gin.Context,
	userID id.ID[user.User],
	listID id.ID[list.ProductList],
	ids []id.ID[product.Product],
) error {
	checkFunc, ch := list.CheckRole(userID, list.MemberTypeEditor)
	err := s.repo.ApplyOrder(ctx, checkFunc, listID, ids)
	member := <-ch
	if err != nil {
		return fmt.Errorf("failed to apply new order to list %s: %w", listID, err)
	}

	s.sendUpdateEvent(listID, member, list.Change{
		Type: list.EventTypeStatesReordered,
		Data: list.StatesReorderedChange{IDs: ids},
	})

	return nil
}

func (s *Service) Create(
	ctx context.Context,
	ownerID id.ID[user.User],
	options list.ListOptions,
) (
	list.ProductList,
	error,
) {
	newList := list.ProductList{
		States: []list.ProductState{},
		Members: []list.Member{
			{
				MemberOptions: list.MemberOptions{
					UserID: ownerID,
					Role:   list.MemberTypeOwner,
				},
				UserName:  "",
				CreatedAt: date.NewCreateDate[list.Member](),
				UpdatedAt: date.NewUpdateDate[list.Member](),
			},
		},
		ListOptions: list.ListOptions{
			Status: list.ExecStatusPlanning,
			Title:  options.Title,
		},
		ID:        id.NewID[list.ProductList](),
		CreatedAt: date.NewCreateDate[list.ProductList](),
		UpdatedAt: date.NewUpdateDate[list.ProductList](),
	}

	if err := s.validate(newList); err != nil {
		return list.ProductList{}, err
	}

	if err := s.repo.CreateList(ctx, newList); err != nil {
		return list.ProductList{}, fmt.Errorf("can't create new list: %w", err)
	}

	return newList, nil
}

func (s *Service) Update(
	ctx context.Context,
	listID id.ID[list.ProductList],
	userID id.ID[user.User],
	options list.ListOptions,
) (
	list.ProductList,
	error,
) {
	var member list.Member
	var err error
	model, err := s.repo.GetAndUpdate(ctx, listID, func(oldList list.ProductList) (list.ProductList, error) {
		member, err = oldList.CheckRole(userID, list.MemberTypeAdmin)
		if err != nil {
			return oldList, fmt.Errorf("checking role failed: %w", err)
		}

		newList := deepcopy.MustCopy(oldList)

		newList.ListOptions = options

		if err = s.validate(newList); err != nil {
			return oldList, err
		}

		return newList, nil
	})
	if err != nil {
		return model, fmt.Errorf("can't update list %s: %w", listID, err)
	}

	s.sendUpdateEvent(listID, member, list.Change{
		Type: list.EventTypeOptsUpdated,
		Data: list.ListOptionsChange{NewOptions: options},
	})

	return model, nil
}

func (s *Service) DeleteList(ctx context.Context, userID id.ID[user.User], listID id.ID[list.ProductList]) error {
	var member list.Member
	var err error

	err = s.repo.GetAndDeleteList(ctx, listID, func(oldList list.ProductList) error {
		member, err = oldList.CheckRole(userID, list.MemberTypeOwner)
		if err != nil {
			return fmt.Errorf("role verification failed: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("can't delete product list %s: %w", listID, err)
	}

	s.sendUpdateEvent(listID, member, list.Change{Type: list.EventTypeDeleted, Data: list.ListDeletedChange{}})

	return nil
}

func (s *Service) GetByID(
	ctx context.Context,
	listID id.ID[list.ProductList],
	userID id.ID[user.User],
) (
	list.ProductList,
	error,
) {
	model, err := s.repo.GetByListID(ctx, listID)
	if err != nil {
		return list.ProductList{}, fmt.Errorf("can't get list %s from storage: %w", listID, err)
	}

	if _, err = model.CheckRole(userID, list.MemberTypeViewer); err != nil {
		return list.ProductList{}, fmt.Errorf("checking role failed: %w", err)
	}

	return model, nil
}

func (s *Service) GetByUserID(ctx context.Context, userID id.ID[user.User]) ([]list.ProductList, error) {
	models, err := s.repo.GetListMetaByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("can't get disk meta related to user %s: %w", userID, err)
	}

	return models, nil
}

func (s *Service) AppendMembers(
	ctx context.Context,
	listID id.ID[list.ProductList],
	userID id.ID[user.User],
	members []list.MemberOptions,
) (
	list.ProductList,
	error,
) {
	var member list.Member
	var err error

	newMembers := lo.Map(members, func(item list.MemberOptions, _ int) list.Member {
		return list.Member{
			MemberOptions: item,
			CreatedAt:     date.NewCreateDate[list.Member](),
			UpdatedAt:     date.NewUpdateDate[list.Member](),
			UserName:      "",
		}
	})

	model, err := s.repo.GetAndUpdate(ctx, listID, func(oldList list.ProductList) (list.ProductList, error) {
		// FIXME: may be for some reason it will be implemented on the frontend
		// if member, err = oldList.CheckRole(userID, list.MemberTypeAdmin); err != nil {
		// 	return oldList, fmt.Errorf("checking role failed: %w", err)
		// }

		newList := deepcopy.MustCopy(oldList)

		newList.Members = append(newList.Members, newMembers...)

		if err = s.validate(newList); err != nil {
			return oldList, err
		}

		return newList, nil
	})
	if err != nil {
		return list.ProductList{}, fmt.Errorf("can't update list %s: %w", listID, err)
	}

	s.sendUpdateEvent(listID, member, list.Change{
		Data: list.MembersAddedChange{NewMembers: newMembers},
		Type: list.EventTypeMembersAdded,
	})

	return model, nil
}

func (s *Service) DeleteMembers(
	ctx context.Context,
	listID id.ID[list.ProductList],
	userID id.ID[user.User],
	toDelete []id.ID[user.User],
) (
	list.ProductList,
	error,
) {
	var member list.Member
	var err error

	model, err := s.repo.GetAndUpdate(ctx, listID, func(oldList list.ProductList) (list.ProductList, error) {
		if len(toDelete) != 1 || toDelete[0] != userID { // or member deleting himself
			if member, err = oldList.CheckRole(userID, list.MemberTypeAdmin); err != nil {
				return oldList, fmt.Errorf("checking role failed: %w", err)
			}
		}

		newList := deepcopy.MustCopy(oldList)

		currentMembers := lo.SliceToMap(
			newList.Members,
			func(item list.Member) (id.ID[user.User], list.Member) { return item.UserID, item },
		)

		for _, userID := range toDelete {
			if _, found := currentMembers[userID]; !found {
				return oldList, fmt.Errorf("%w: member %s", myerr.ErrNotFound, userID)
			}

			delete(currentMembers, userID)
		}

		newList.Members = slices.Collect(maps.Values(currentMembers))

		if err = s.validate(newList); err != nil {
			return oldList, err
		}

		return newList, nil
	})
	if err != nil {
		return model, fmt.Errorf("can't delete members from list %s: %w", listID, err)
	}

	s.sendUpdateEvent(listID, member, list.Change{
		Data: list.MembersDeletedChange{UserIDs: toDelete},
		Type: list.EventTypeMembersRemoved,
	})

	return model, nil
}

func (s *Service) AppendProducts(
	ctx context.Context,
	listID id.ID[list.ProductList],
	userID id.ID[user.User],
	states map[id.ID[product.Product]]list.ProductStateOptions,
) (
	list.ProductList,
	error,
) {
	var member list.Member
	var err error

	s.log.Info().Stringer("list_id", listID).Stringer("user_id", userID).Any("states", states).Msg("appending products")

	newStates := make([]list.ProductState, 0, len(states))
	for productID, stateOpts := range states {
		newProductState := list.ProductState{
			ProductStateOptions: stateOpts,
			Product: product.Product{
				Options:   product.NewZeroOptions(),
				ID:        productID,
				CreatedAt: date.CreateDate[product.Product]{Time: time.Time{}},
				UpdatedAt: date.UpdateDate[product.Product]{Time: time.Time{}},
			},
			CreatedAt: date.NewCreateDate[list.ProductState](),
			UpdatedAt: date.NewUpdateDate[list.ProductState](),
		}

		newStates = append(newStates, newProductState)
	}

	model, err := s.repo.GetAndUpdate(ctx, listID, func(oldList list.ProductList) (list.ProductList, error) {
		if member, err = oldList.CheckRole(userID, list.MemberTypeEditor); err != nil {
			return oldList, fmt.Errorf("checking role failed: %w", err)
		}

		newList := deepcopy.MustCopy(oldList)

		newList.States = append(newList.States, newStates...)

		if err = s.validate(oldList); err != nil {
			return oldList, err
		}

		return newList, nil
	})
	if err != nil {
		return list.ProductList{}, fmt.Errorf("can't append products: %w", err)
	}

	s.log.Info().Stringer("list_id", listID).Stringer("user_id", userID).Any("model", model).Msg("updated")

	s.sendUpdateEvent(listID, member, list.Change{
		Type: list.EventTypeProductsAdded,
		Data: list.ProductsAddedChange{Products: newStates},
	})

	return model, nil
}

func (s *Service) UpdateProductState(ctx context.Context,
	listID id.ID[list.ProductList],
	userID id.ID[user.User],
	productID id.ID[product.Product],
	stateOpts list.ProductStateOptions,
) (
	list.ProductState,
	error,
) {
	var member list.Member
	var state list.ProductState

	s.log.Info().
		Any("product_state_opts", stateOpts).
		Stringer("product_id", productID).
		Stringer("user_id", userID).
		Msg("updating product state")

	_, err := s.repo.GetAndUpdate(ctx, listID, func(oldList list.ProductList) (list.ProductList, error) {
		var err error

		if member, err = oldList.CheckRole(userID, list.MemberTypeAdmin); err != nil {
			return oldList, fmt.Errorf("checking role failed: %w", err)
		}

		s.log.Debug().Any("model", oldList).Msg("got old list from repo")

		newList := deepcopy.MustCopy(oldList)

		idx := slices.IndexFunc(newList.States, func(s list.ProductState) bool { return s.Product.ID == productID })
		if idx == -1 {
			return oldList, fmt.Errorf("%w: product state with product id: %s", myerr.ErrNotFound, productID)
		}

		newList.States[idx].ProductStateOptions = stateOpts
		state = newList.States[idx]

		if err = s.validate(newList); err != nil {
			return oldList, err
		}

		return newList, nil
	})
	if err != nil {
		return list.ProductState{}, fmt.Errorf("failed to update product state %s in list %s: %w", productID, listID, err)
	}

	s.sendUpdateEvent(listID, member, list.Change{
		Type: list.EventTypeStateUpdated,
		Data: list.StateUpdatedChange{ProductID: productID, State: state},
	})

	s.log.Info().Any("product_state", state).Stringer("product_id", productID).Stringer("user_id", userID).
		Msg("updating product state")

	return state, nil
}

func (s *Service) DeleteProducts(
	ctx context.Context,
	listID id.ID[list.ProductList],
	userID id.ID[user.User],
	toDelete []id.ID[product.Product],
) (
	list.ProductList,
	error,
) {
	var member list.Member
	var err error

	model, err := s.repo.GetAndUpdate(ctx, listID, func(oldList list.ProductList) (list.ProductList, error) {
		if member, err = oldList.CheckRole(userID, list.MemberTypeEditor); err != nil {
			return oldList, fmt.Errorf("checking role failed: %w", err)
		}

		newList := deepcopy.MustCopy(oldList)

		currentStates := lo.SliceToMap(
			newList.States,
			func(item list.ProductState) (id.ID[product.Product], list.ProductState) {
				return item.Product.ID, item
			},
		)

		for _, productID := range toDelete {
			if _, found := currentStates[productID]; !found {
				return oldList, fmt.Errorf("%w: state with product id %s", myerr.ErrNotFound, productID)
			}

			delete(currentStates, productID)
		}

		newList.States = slices.Collect(maps.Values(currentStates))

		if err = s.validate(newList); err != nil {
			return oldList, err
		}

		return newList, nil
	})
	if err != nil {
		return list.ProductList{}, fmt.Errorf("can't delete products from list %s: %w", listID, err)
	}

	s.sendUpdateEvent(listID, member, list.Change{
		Type: list.EventTypeProductsRemoved,
		Data: list.ProductsRemovedChange{IDs: toDelete},
	})

	return model, nil
}
