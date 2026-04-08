package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListContacts は全担当者を返す。
func (s *Service) ListContacts(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ContactEntity, error) {
	return s.contacts.List(ctx, opts)
}

// GetContact は指定 ID の担当者を返す。
func (s *Service) GetContact(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ContactEntity, error) {
	return s.contacts.GetByID(ctx, id, opts)
}

// SearchContacts はパラメータでフィルタした担当者を返す。
func (s *Service) SearchContacts(ctx context.Context, params boardapi.ContactSearchParams, opts repository.ReadOptions) ([]boardapi.ContactEntity, error) {
	return s.contacts.Search(ctx, params, opts)
}
