// Package find2 は BOARD API の高レベル横断検索サービスを提供する。
// find パッケージのゼロベース再設計版（ADR-001 B 採択）。
// 具象 Find メソッドは N04 以降で追加される。
package find2

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ClientRepo はクライアントリポジトリインターフェース。
type ClientRepo interface {
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ClientEntity, error)
	Search(ctx context.Context, filter boardapi.ClientListOptions, opts repository.ReadOptions) ([]boardapi.ClientEntity, error)
}

// ClientBranchRepo はクライアント拠点リポジトリインターフェース（enrichment 専用）。
type ClientBranchRepo interface {
	Search(ctx context.Context, filter boardapi.ClientBranchListOptions, opts repository.ReadOptions) ([]boardapi.ClientBranchEntity, error)
}

// ContactRepo は担当者リポジトリインターフェース（enrichment 専用）。
type ContactRepo interface {
	Search(ctx context.Context, filter boardapi.ContactListOptions, opts repository.ReadOptions) ([]boardapi.ContactEntity, error)
}

// ProjectRepo はプロジェクトリポジトリインターフェース。
// reverseMapper の内部 build でも Search を使用する。
type ProjectRepo interface {
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ProjectEntity, error)
	Search(ctx context.Context, filter boardapi.ProjectListOptions, opts repository.ReadOptions) ([]boardapi.ProjectEntity, error)
	GetByIDWithGroup(ctx context.Context, id int, responseGroup string) (*boardapi.ProjectEntity, error)
}

// EstimateRepo は見積リポジトリインターフェース。
// Document 系は reverseMapper で project_id を解決するため GetByDocumentID のみ。
type EstimateRepo interface {
	GetByDocumentID(ctx context.Context, documentID int, opts repository.ReadOptions) (*boardapi.EstimateEntity, error)
}

// OrderRepo は注文リポジトリインターフェース。
type OrderRepo interface {
	GetByDocumentID(ctx context.Context, documentID int, opts repository.ReadOptions) (*boardapi.OrderEntity, error)
}

// DeliveryRepo は納品リポジトリインターフェース。
type DeliveryRepo interface {
	GetByDocumentID(ctx context.Context, documentID int, opts repository.ReadOptions) (*boardapi.DeliveryEntity, error)
}

// ReceiptRepo は領収リポジトリインターフェース。
type ReceiptRepo interface {
	GetByDocumentID(ctx context.Context, documentID int, opts repository.ReadOptions) (*boardapi.ReceiptEntity, error)
}

// InvoiceRepo は請求書リポジトリインターフェース。
type InvoiceRepo interface {
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.InvoiceEntity, error)
	Search(ctx context.Context, filter boardapi.InvoiceListOptions, opts repository.ReadOptions) ([]boardapi.InvoiceEntity, error)
}

// VendorRepo は仕入先リポジトリインターフェース。
// find2 では ListEntities は不使用（N02 §2 決定）。
type VendorRepo interface {
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.VendorEntity, error)
	Search(ctx context.Context, filter boardapi.VendorListOptions, opts repository.ReadOptions) ([]boardapi.VendorEntity, error)
}

// VendorBranchRepo は仕入先拠点リポジトリインターフェース（enrichment 専用）。
type VendorBranchRepo interface {
	Search(ctx context.Context, filter boardapi.VendorBranchListOptions, opts repository.ReadOptions) ([]boardapi.VendorBranchEntity, error)
}

// VendorContactRepo は仕入先担当者リポジトリインターフェース（enrichment 専用）。
type VendorContactRepo interface {
	Search(ctx context.Context, filter boardapi.VendorContactListOptions, opts repository.ReadOptions) ([]boardapi.VendorContactEntity, error)
}

// PurchaseOrderRepo は発注書リポジトリインターフェース。
type PurchaseOrderRepo interface {
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PurchaseOrderEntity, error)
	Search(ctx context.Context, filter boardapi.PurchaseOrderListOptions, opts repository.ReadOptions) ([]boardapi.PurchaseOrderEntity, error)
}

// PaymentRepo は支払リポジトリインターフェース。
type PaymentRepo interface {
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PaymentEntity, error)
	Search(ctx context.Context, filter boardapi.PaymentListOptions, opts repository.ReadOptions) ([]boardapi.PaymentEntity, error)
}

// UserRepo はユーザーリポジトリインターフェース。
// GroupRepo は N02 §2 の決定により削除（api_groups_list で代替）。
type UserRepo interface {
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.UserEntity, error)
	Search(ctx context.Context, filter boardapi.UserListOptions, opts repository.ReadOptions) ([]boardapi.UserEntity, error)
}

// Repos は find2 Service が依存するリポジトリ群を集約する。
type Repos struct {
	Clients        ClientRepo
	ClientBranches ClientBranchRepo
	Contacts       ContactRepo
	Projects       ProjectRepo
	Estimates      EstimateRepo
	Orders         OrderRepo
	Deliveries     DeliveryRepo
	Receipts       ReceiptRepo
	Invoices       InvoiceRepo
	Vendors        VendorRepo
	VendorBranches VendorBranchRepo
	VendorContacts VendorContactRepo
	PurchaseOrders PurchaseOrderRepo
	Payments       PaymentRepo
	Users          UserRepo
}

// Service は横断検索サービス。具象 Find メソッドは N04 以降で追加される。
type Service struct {
	clients        ClientRepo
	clientBranches ClientBranchRepo
	contacts       ContactRepo
	projects       ProjectRepo
	estimates      EstimateRepo
	orders         OrderRepo
	deliveries     DeliveryRepo
	receipts       ReceiptRepo
	invoices       InvoiceRepo
	vendors        VendorRepo
	vendorBranches VendorBranchRepo
	vendorContacts VendorContactRepo
	purchaseOrders PurchaseOrderRepo
	payments       PaymentRepo
	users          UserRepo

	// Document 4 種の逆マッピングキャッシュ（lazy build、N04 以降で registration）
	reverseMappers map[string]*reverseMapper
}

// New は新しい find2 Service を生成する。
func New(r Repos) *Service {
	return &Service{
		clients:        r.Clients,
		clientBranches: r.ClientBranches,
		contacts:       r.Contacts,
		projects:       r.Projects,
		estimates:      r.Estimates,
		orders:         r.Orders,
		deliveries:     r.Deliveries,
		receipts:       r.Receipts,
		invoices:       r.Invoices,
		vendors:        r.Vendors,
		vendorBranches: r.VendorBranches,
		vendorContacts: r.VendorContacts,
		purchaseOrders: r.PurchaseOrders,
		payments:       r.Payments,
		users:          r.Users,
		reverseMappers: map[string]*reverseMapper{
			"estimate": newReverseMapper(r.Projects, "estimate", extractEstimateIDs),
			"order":    newReverseMapper(r.Projects, "order", extractOrderIDs),
			"delivery": newReverseMapper(r.Projects, "delivery", extractDeliveryIDs),
			"receipt":  newReverseMapper(r.Projects, "receipt", extractReceiptIDs),
		},
	}
}

// repoOpts は FindCommonOpts から repository.ReadOptions を取り出すヘルパー。
// N04-N07a で各 FindXxx メソッドが使用する。
func repoOpts(o FindCommonOpts) repository.ReadOptions {
	return o.Opts
}
