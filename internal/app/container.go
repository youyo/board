package app

import (
	"time"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/cache"
	"github.com/youyo/board/internal/refresh"
	"github.com/youyo/board/internal/repository"
)

// Repositories is a container holding repositories for all 22 resources.
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

// newRepositories initializes all 22 repositories and returns a Repositories.
func newRepositories(
	profile string,
	api *boardapi.Client,
	rc *cache.ResourceCache,
	ss *cache.SyncStateStore,
	refresher *refresh.Refresher,
	lm *refresh.LockManager,
	tz *time.Location,
) *Repositories {
	return &Repositories{
		Clients:              repository.NewClientRepository(profile, api, rc, ss, refresher, lm, tz),
		ClientBranches:       repository.NewClientBranchRepository(profile, api, rc, ss, refresher, lm, tz),
		Contacts:             repository.NewContactRepository(profile, api, rc, ss, refresher, lm, tz),
		Projects:             repository.NewProjectRepository(profile, api, rc, ss, refresher, lm, tz),
		ProjectCosts:         repository.NewProjectCostRepository(profile, api, rc, ss, refresher, lm, tz),
		Estimates:            repository.NewEstimateRepository(profile, api, rc),
		Invoices:             repository.NewInvoiceRepository(profile, api, rc, ss, refresher, lm, tz),
		Orders:               repository.NewOrderRepository(profile, api, rc),
		Deliveries:           repository.NewDeliveryRepository(profile, api, rc),
		Receipts:             repository.NewReceiptRepository(profile, api, rc),
		Vendors:              repository.NewVendorRepository(profile, api, rc, ss, refresher, lm, tz),
		VendorBranches:       repository.NewVendorBranchRepository(profile, api, rc, ss, refresher, lm, tz),
		VendorContacts:       repository.NewVendorContactRepository(profile, api, rc, ss, refresher, lm, tz),
		PurchaseOrders:       repository.NewPurchaseOrderRepository(profile, api, rc, ss, refresher, lm, tz),
		Payments:             repository.NewPaymentRepository(profile, api, rc, ss, refresher, lm, tz),
		Users:                repository.NewUserRepository(profile, api, rc, ss, refresher, lm, tz),
		Groups:               repository.NewGroupRepository(profile, api, rc, ss, refresher, lm, tz),
		PaymentTerms:         repository.NewPaymentTermRepository(profile, api, rc, ss, refresher, lm, tz),
		ProjectTypes:         repository.NewProjectTypeRepository(profile, api, rc, ss, refresher, lm, tz),
		PurchaseTypes:        repository.NewPurchaseTypeRepository(profile, api, rc, ss, refresher, lm, tz),
		AccountingTypes:      repository.NewAccountingTypeRepository(profile, api, rc, ss, refresher, lm, tz),
		DocumentSendChannels: repository.NewDocumentSendChannelRepository(profile, api, rc, ss, refresher, lm, tz),
	}
}
