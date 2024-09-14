package shopmap

import (
	"context"
	productModel "go-backend/internal/product/models"
	"go-backend/internal/shopmap/models"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

func TestNewService(t *testing.T) {
	tests := []struct {
		name string
		want *Service
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewService(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewService() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestService_Create(t *testing.T) {
	type fields struct {
		users     userService
		repo      repo
		log       *zerolog.Logger
		validator *validate.Validator
	}
	type args struct {
		ctx        context.Context
		ownerID    uuid.UUID
		categories []productModel.Category
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    models.ShopMap
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{
				users:     tt.fields.users,
				repo:      tt.fields.repo,
				log:       tt.fields.log,
				validator: tt.fields.validator,
			}
			got, err := s.Create(tt.args.ctx, tt.args.ownerID, tt.args.categories)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Service.Create() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestService_Update(t *testing.T) {
	type fields struct {
		users     userService
		repo      repo
		log       *zerolog.Logger
		validator *validate.Validator
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{
				users:     tt.fields.users,
				repo:      tt.fields.repo,
				log:       tt.fields.log,
				validator: tt.fields.validator,
			}
			s.Update()
		})
	}
}
