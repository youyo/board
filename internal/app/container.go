package app

import (
	"time"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/cache"
	"github.com/youyo/board/internal/refresh"
	"github.com/youyo/board/internal/repository"
)

// Repositories は全22リソースの Repository を保持するコンテナ。
type Repositories struct {
	Clients              *repository.ClientRepository
	ClientBranches       *repository.ClientBranchRepository
	Contacts             *repository.ContactRepository
	Projects             *repository.ProjectRepository
	ProjectCosts         *repository.ProjectCostRepository
	Estimates            *repository.EstimateRepository
	Invoices             *repository.InvoiceRepository
	Orders               *repository.OrderRepository
	Deliveries           *repository.DeliveryRepository
	Receipts             *repository.ReceiptRepository
	Vendors              *repository.VendorRepository
	VendorBranches       *repository.VendorBranchRepository
	VendorContacts       *repository.VendorContactRepository
	PurchaseOrders       *repository.PurchaseOrderRepository
	Payments             *repository.PaymentRepository
	Users                *repository.UserRepository
	Groups               *repository.GroupRepository
	PaymentTerms         *repository.PaymentTermRepository
	ProjectTypes         *repository.ProjectTypeRepository
	PurchaseTypes        *repository.PurchaseTypeRepository
	AccountingTypes      *repository.AccountingTypeRepository
	DocumentSendChannels *repository.DocumentSendChannelRepository
}

// newRepositories は全22リポジトリを初期化して Repositories を返す。
func newRepositories(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
	autoRefresh bool,
) *Repositories {
	return &Repositories{
		Clients:              repository.NewClientRepository(profile, api, rc, ss, refresher, lm, tz, autoRefresh),
		ClientBranches:       repository.NewClientBranchRepository(profile, api, rc, ss, refresher, lm, tz, autoRefresh),
		Contacts:             repository.NewContactRepository(profile, api, rc, ss, refresher, lm, tz, autoRefresh),
		Projects:             repository.NewProjectRepository(profile, api, rc, ss, refresher, lm, tz, autoRefresh),
		ProjectCosts:         repository.NewProjectCostRepository(profile, api, rc, ss, refresher, lm, tz, autoRefresh),
		Estimates:            repository.NewEstimateRepository(profile, api, rc, ss, refresher, lm, tz, autoRefresh),
		Invoices:             repository.NewInvoiceRepository(profile, api, rc, ss, refresher, lm, tz, autoRefresh),
		Orders:               repository.NewOrderRepository(profile, api, rc, ss, refresher, lm, tz, autoRefresh),
		Deliveries:           repository.NewDeliveryRepository(profile, api, rc, ss, refresher, lm, tz, autoRefresh),
		Receipts:             repository.NewReceiptRepository(profile, api, rc, ss, refresher, lm, tz, autoRefresh),
		Vendors:              repository.NewVendorRepository(profile, api, rc, ss, refresher, lm, tz, autoRefresh),
		VendorBranches:       repository.NewVendorBranchRepository(profile, api, rc, ss, refresher, lm, tz, autoRefresh),
		VendorContacts:       repository.NewVendorContactRepository(profile, api, rc, ss, refresher, lm, tz, autoRefresh),
		PurchaseOrders:       repository.NewPurchaseOrderRepository(profile, api, rc, ss, refresher, lm, tz, autoRefresh),
		Payments:             repository.NewPaymentRepository(profile, api, rc, ss, refresher, lm, tz, autoRefresh),
		Users:                repository.NewUserRepository(profile, api, rc, ss, refresher, lm, tz, autoRefresh),
		Groups:               repository.NewGroupRepository(profile, api, rc, ss, refresher, lm, tz, autoRefresh),
		PaymentTerms:         repository.NewPaymentTermRepository(profile, api, rc, ss, refresher, lm, tz, autoRefresh),
		ProjectTypes:         repository.NewProjectTypeRepository(profile, api, rc, ss, refresher, lm, tz, autoRefresh),
		PurchaseTypes:        repository.NewPurchaseTypeRepository(profile, api, rc, ss, refresher, lm, tz, autoRefresh),
		AccountingTypes:      repository.NewAccountingTypeRepository(profile, api, rc, ss, refresher, lm, tz, autoRefresh),
		DocumentSendChannels: repository.NewDocumentSendChannelRepository(profile, api, rc, ss, refresher, lm, tz, autoRefresh),
	}
}
