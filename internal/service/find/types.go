package find

import (
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// FindClientQuery holds parameters for FindClient.
// Field priority: ID > Name > Text. If ID is set, Name and Text are ignored.
// If Name is set, Text is ignored. At least one field must be set.
type FindClientQuery struct {
	ID    int    // Direct lookup by ID (highest priority, ignores Name/Text)
	Name  string // Substring match on client name (ignores Text)
	Text  string // Free-text search across name, code, memo (lowest priority)
	Limit int    // Max results to return (0 = unlimited). Applied at find layer.
	Opts  repository.ReadOptions
}

// FindProjectQuery holds parameters for FindProject.
// Field priority: ID > ClientName > Name > Text. Higher priority fields
// override lower ones. Status is an additional filter applied on top.
type FindProjectQuery struct {
	ID         int    // Direct lookup by ID (highest priority)
	ClientName string // Resolve client name -> client IDs -> filter projects
	Name       string // Project name substring search
	Text       string // Free-text search across name, code, memo (lowest priority)
	Status     string // Additional filter (applied on top of any search mode)
	Limit      int    // Max results to return (0 = unlimited). Applied at find layer.
	Opts       repository.ReadOptions
}

// ClientResult is the aggregated result for a client search.
type ClientResult struct {
	Client   boardapi.ClientEntity         `json:"client"`
	Branches []boardapi.ClientBranchEntity `json:"branches"`
	Contacts []boardapi.ContactEntity      `json:"contacts"`
}

// ProjectResult is the aggregated result for a project search.
type ProjectResult struct {
	Project boardapi.ProjectEntity `json:"project"`
	Client  *boardapi.ClientEntity `json:"client,omitempty"`
}

// FindEstimateQuery holds parameters for FindEstimate.
// Field priority: ID > ClientName > ProjectName > Text > Status(standalone).
// Status also acts as a post-filter when combined with other criteria.
type FindEstimateQuery struct {
	ID          int    // Direct lookup by ID (highest priority)
	ClientName  string // Resolve client name -> client IDs -> search estimates
	ProjectName string // Resolve project name -> project IDs -> search estimates
	Text        string // Free-text search across title, memo
	Status      string // Standalone filter or post-filter on top of other modes
	Limit       int    // Max results to return (0 = unlimited). Applied at find layer.
	Opts        repository.ReadOptions
}

// EstimateResult is the aggregated result for an estimate search.
type EstimateResult struct {
	Estimate boardapi.EstimateEntity `json:"estimate"`
	Client   *boardapi.ClientEntity  `json:"client,omitempty"`
	Project  *boardapi.ProjectEntity `json:"project,omitempty"`
}

// FindInvoiceQuery holds parameters for FindInvoice.
// Field priority: ID > ClientName > ProjectName > Text > Status(standalone).
type FindInvoiceQuery struct {
	ID          int
	ClientName  string
	ProjectName string
	Text        string
	Status      string
	Limit       int
	Opts        repository.ReadOptions
}

// InvoiceResult is the aggregated result for an invoice search.
type InvoiceResult struct {
	Invoice boardapi.InvoiceEntity  `json:"invoice"`
	Client  *boardapi.ClientEntity  `json:"client,omitempty"`
	Project *boardapi.ProjectEntity `json:"project,omitempty"`
}

// FindOrderQuery holds parameters for FindOrder.
// Field priority: ID > ClientName > ProjectName > Text > Status(standalone).
type FindOrderQuery struct {
	ID          int
	ClientName  string
	ProjectName string
	Text        string
	Status      string
	Limit       int
	Opts        repository.ReadOptions
}

// OrderResult is the aggregated result for an order search.
type OrderResult struct {
	Order   boardapi.OrderEntity    `json:"order"`
	Client  *boardapi.ClientEntity  `json:"client,omitempty"`
	Project *boardapi.ProjectEntity `json:"project,omitempty"`
}

// FindDeliveryQuery holds parameters for FindDelivery.
// Field priority: ID > ClientName > ProjectName > Text > Status(standalone).
type FindDeliveryQuery struct {
	ID          int
	ClientName  string
	ProjectName string
	Text        string
	Status      string
	Limit       int
	Opts        repository.ReadOptions
}

// DeliveryResult is the aggregated result for a delivery search.
type DeliveryResult struct {
	Delivery boardapi.DeliveryEntity `json:"delivery"`
	Client   *boardapi.ClientEntity  `json:"client,omitempty"`
	Project  *boardapi.ProjectEntity `json:"project,omitempty"`
}

// FindReceiptQuery holds parameters for FindReceipt.
// Field priority: ID > ClientName > ProjectName > Text > Status(standalone).
type FindReceiptQuery struct {
	ID          int
	ClientName  string
	ProjectName string
	Text        string
	Status      string
	Limit       int
	Opts        repository.ReadOptions
}

// ReceiptResult is the aggregated result for a receipt search.
type ReceiptResult struct {
	Receipt boardapi.ReceiptEntity  `json:"receipt"`
	Client  *boardapi.ClientEntity  `json:"client,omitempty"`
	Project *boardapi.ProjectEntity `json:"project,omitempty"`
}

// FindVendorQuery holds parameters for FindVendor.
// Field priority: ID > Name > Text. If ID is set, Name and Text are ignored.
type FindVendorQuery struct {
	ID    int    // Direct lookup by ID (highest priority, ignores Name/Text)
	Name  string // Substring match on vendor name (ignores Text)
	Text  string // Free-text search across name, code, memo (lowest priority)
	Limit int    // Max results to return (0 = unlimited). Applied at find layer.
	Opts  repository.ReadOptions
}

// VendorResult is the aggregated result for a vendor search.
type VendorResult struct {
	Vendor   boardapi.VendorEntity          `json:"vendor"`
	Branches []boardapi.VendorBranchEntity  `json:"branches"`
	Contacts []boardapi.VendorContactEntity `json:"contacts"`
}

// FindPurchaseOrderQuery holds parameters for FindPurchaseOrder.
// Field priority: ID > VendorName > ProjectName > Text > Status(standalone).
// Status also acts as a post-filter when combined with other criteria.
type FindPurchaseOrderQuery struct {
	ID          int    // Direct lookup by ID (highest priority)
	VendorName  string // Resolve vendor name -> vendor IDs -> search purchase orders
	ProjectName string // Resolve project name -> project IDs -> search purchase orders
	Text        string // Free-text search across title, memo
	Status      string // Standalone filter or post-filter on top of other modes
	Limit       int    // Max results to return (0 = unlimited). Applied at find layer.
	Opts        repository.ReadOptions
}

// PurchaseOrderResult is the aggregated result for a purchase order search.
type PurchaseOrderResult struct {
	PurchaseOrder boardapi.PurchaseOrderEntity `json:"purchase_order"`
	Vendor        *boardapi.VendorEntity       `json:"vendor,omitempty"`
	Project       *boardapi.ProjectEntity      `json:"project,omitempty"`
}

// FindPaymentQuery holds parameters for FindPayment.
// Field priority: ID > VendorName > PurchaseOrderID > Text > Status(standalone).
// Status also acts as a post-filter when combined with other criteria.
type FindPaymentQuery struct {
	ID              int    // Direct lookup by ID (highest priority)
	VendorName      string // Resolve vendor name -> vendor IDs -> search payments
	PurchaseOrderID int    // Search payments by purchase order ID
	Text            string // Free-text search across memo
	Status          string // Standalone filter or post-filter on top of other modes
	Limit           int    // Max results to return (0 = unlimited). Applied at find layer.
	Opts            repository.ReadOptions
}

// PaymentResult is the aggregated result for a payment search.
type PaymentResult struct {
	Payment boardapi.PaymentEntity `json:"payment"`
	Vendor  *boardapi.VendorEntity `json:"vendor,omitempty"`
}

// FindUserQuery holds parameters for FindUser.
// Field priority: ID > Name > Text.
type FindUserQuery struct {
	ID    int    // Direct lookup by ID (highest priority, ignores Name/Text)
	Name  string // Substring match on user name (ignores Text)
	Text  string // Free-text search across name, email (lowest priority)
	Limit int    // Max results to return (0 = unlimited). Applied at find layer.
	Opts  repository.ReadOptions
}

// UserResult is the result for a user search.
type UserResult struct {
	User boardapi.UserEntity `json:"user"`
}

// FindGroupQuery holds parameters for FindGroup.
// Field priority: ID > Name > Text.
type FindGroupQuery struct {
	ID    int    // Direct lookup by ID (highest priority, ignores Name/Text)
	Name  string // Substring match on group name (ignores Text)
	Text  string // Free-text search across name, memo (lowest priority)
	Limit int    // Max results to return (0 = unlimited). Applied at find layer.
	Opts  repository.ReadOptions
}

// GroupResult is the result for a group search.
type GroupResult struct {
	Group boardapi.GroupEntity `json:"group"`
}
