package find2

import (
	"errors"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// FindCommonOpts は全 Find クエリ共通のオプション。
type FindCommonOpts struct {
	// Limit は返却件数上限。0 は無制限（エラーなし）。負数は validation error。
	Limit int
	// Opts はリポジトリへ渡す読取オプション（Refresh/ForceRefresh 等）。
	Opts repository.ReadOptions
}

// validate は共通オプションを検証する。
// Limit=0 は無制限として許容する（T15a）。Limit<0 は error（T15b）。
func (o FindCommonOpts) validate() error {
	if o.Limit < 0 {
		return errors.New("limit must be >= 0")
	}
	return nil
}

// validatable は validate を持つ型の制約インターフェース。
type validatable interface {
	validate() error
}

// validateQuery は共通 opts + クエリ固有の validate をまとめて実行する。
func validateQuery(common FindCommonOpts, specific validatable) error {
	if err := common.validate(); err != nil {
		return err
	}
	return specific.validate()
}

// validateStatusFields は Status/Statuses 排他チェックと Statuses 上限チェックを行う。
func validateStatusFields(status string, statuses []string) error {
	if status != "" && len(statuses) > 0 {
		return errors.New("Status and Statuses are mutually exclusive")
	}
	if len(statuses) > 10 {
		return errors.New("at most 10 statuses allowed")
	}
	return nil
}

// ========== Query 型 ==========

// FindClientQuery はクライアント検索クエリ。
type FindClientQuery struct {
	FindCommonOpts
	ID   int
	Name string
	Text string
}

func (q FindClientQuery) validate() error {
	if q.ID == 0 && q.Name == "" && q.Text == "" {
		return errors.New("at least one field required")
	}
	return nil
}

// FindProjectQuery はプロジェクト検索クエリ。
type FindProjectQuery struct {
	FindCommonOpts
	ID       int
	Name     string
	ClientID int
	Text     string
	Status   string
	Statuses []string
}

func (q FindProjectQuery) validate() error {
	if err := validateStatusFields(q.Status, q.Statuses); err != nil {
		return err
	}
	if q.ID == 0 && q.Name == "" && q.ClientID == 0 && q.Text == "" &&
		q.Status == "" && len(q.Statuses) == 0 {
		return errors.New("at least one field required")
	}
	// advisor R3: Status/Statuses-only は API delegation 不可で全件取得を要するため reject。
	// Status/Statuses を使う場合は ID/Name/ClientID/Text のいずれかによる narrowing が必須。
	hasNarrow := q.ID != 0 || q.Name != "" || q.ClientID != 0 || q.Text != ""
	hasStatus := q.Status != "" || len(q.Statuses) > 0
	if hasStatus && !hasNarrow {
		return errors.New("Status/Statuses requires at least one of ID, Name, ClientID, or Text to narrow results")
	}
	return nil
}

// FindEstimateQuery は見積書検索クエリ。
// EstimateEntity には Status フィールドがないため、SealApprovalStatus による絞り込みを想定。
// Status/Statuses は N04 以降の実装時に要否を再評価する（N03 では形 only）。
type FindEstimateQuery struct {
	FindCommonOpts
	ID          int
	ProjectID   int
	ClientName  string
	ProjectName string
	Text        string
}

func (q FindEstimateQuery) validate() error {
	if q.ID == 0 && q.ProjectID == 0 && q.ClientName == "" &&
		q.ProjectName == "" && q.Text == "" {
		return errors.New("at least one field required")
	}
	return nil
}

// FindOrderQuery は注文書検索クエリ。
type FindOrderQuery struct {
	FindCommonOpts
	ID          int
	ProjectID   int
	ClientName  string
	ProjectName string
	Text        string
}

func (q FindOrderQuery) validate() error {
	if q.ID == 0 && q.ProjectID == 0 && q.ClientName == "" &&
		q.ProjectName == "" && q.Text == "" {
		return errors.New("at least one field required")
	}
	return nil
}

// FindDeliveryQuery は納品書検索クエリ。
type FindDeliveryQuery struct {
	FindCommonOpts
	ID          int
	ProjectID   int
	ClientName  string
	ProjectName string
	Text        string
}

func (q FindDeliveryQuery) validate() error {
	if q.ID == 0 && q.ProjectID == 0 && q.ClientName == "" &&
		q.ProjectName == "" && q.Text == "" {
		return errors.New("at least one field required")
	}
	return nil
}

// FindReceiptQuery は領収書検索クエリ。
type FindReceiptQuery struct {
	FindCommonOpts
	ID          int
	ProjectID   int
	ClientName  string
	ProjectName string
	Text        string
}

func (q FindReceiptQuery) validate() error {
	if q.ID == 0 && q.ProjectID == 0 && q.ClientName == "" &&
		q.ProjectName == "" && q.Text == "" {
		return errors.New("at least one field required")
	}
	return nil
}

// FindInvoiceQuery は請求書検索クエリ。
type FindInvoiceQuery struct {
	FindCommonOpts
	ID       int
	ClientID int
	Text     string
	Status   string
	Statuses []string
}

func (q FindInvoiceQuery) validate() error {
	if err := validateStatusFields(q.Status, q.Statuses); err != nil {
		return err
	}
	if q.ID == 0 && q.ClientID == 0 && q.Text == "" &&
		q.Status == "" && len(q.Statuses) == 0 {
		return errors.New("at least one field required")
	}
	return nil
}

// FindVendorQuery は仕入先検索クエリ。
type FindVendorQuery struct {
	FindCommonOpts
	ID   int
	Name string
	Text string
}

func (q FindVendorQuery) validate() error {
	if q.ID == 0 && q.Name == "" && q.Text == "" {
		return errors.New("at least one field required")
	}
	return nil
}

// FindPurchaseOrderQuery は発注書検索クエリ。
type FindPurchaseOrderQuery struct {
	FindCommonOpts
	ID       int
	VendorID int
	Text     string
	Status   string
	Statuses []string
}

func (q FindPurchaseOrderQuery) validate() error {
	if err := validateStatusFields(q.Status, q.Statuses); err != nil {
		return err
	}
	if q.ID == 0 && q.VendorID == 0 && q.Text == "" &&
		q.Status == "" && len(q.Statuses) == 0 {
		return errors.New("at least one field required")
	}
	return nil
}

// FindPaymentQuery は支払検索クエリ。
type FindPaymentQuery struct {
	FindCommonOpts
	ID       int
	VendorID int
	Text     string
	Status   string
	Statuses []string
}

func (q FindPaymentQuery) validate() error {
	if err := validateStatusFields(q.Status, q.Statuses); err != nil {
		return err
	}
	if q.ID == 0 && q.VendorID == 0 && q.Text == "" &&
		q.Status == "" && len(q.Statuses) == 0 {
		return errors.New("at least one field required")
	}
	return nil
}

// FindUserQuery はユーザー検索クエリ。
type FindUserQuery struct {
	FindCommonOpts
	ID   int
	Name string
	Text string
}

func (q FindUserQuery) validate() error {
	if q.ID == 0 && q.Name == "" && q.Text == "" {
		return errors.New("at least one field required")
	}
	return nil
}

// ========== Result 型 ==========

// ClientResult はクライアント検索結果。
type ClientResult struct {
	Client   boardapi.ClientEntity
	Branches []boardapi.ClientBranchEntity
	Contacts []boardapi.ContactEntity
}

// ProjectResult はプロジェクト検索結果。
type ProjectResult struct {
	Project boardapi.ProjectEntity
	Client  *boardapi.ClientEntity
}

// EstimateResult は見積書検索結果。
// ProjectID/ClientID は逆マッピング（reverseMapper）で解決される。
type EstimateResult struct {
	Estimate  boardapi.EstimateEntity
	ProjectID int
	ClientID  int
	Project   *boardapi.ProjectEntity
	Client    *boardapi.ClientEntity
}

// OrderResult は注文書検索結果。
type OrderResult struct {
	Order     boardapi.OrderEntity
	ProjectID int
	ClientID  int
	Project   *boardapi.ProjectEntity
	Client    *boardapi.ClientEntity
}

// DeliveryResult は納品書検索結果。
type DeliveryResult struct {
	Delivery  boardapi.DeliveryEntity
	ProjectID int
	ClientID  int
	Project   *boardapi.ProjectEntity
	Client    *boardapi.ClientEntity
}

// ReceiptResult は領収書検索結果。
type ReceiptResult struct {
	Receipt   boardapi.ReceiptEntity
	ProjectID int
	ClientID  int
	Project   *boardapi.ProjectEntity
	Client    *boardapi.ClientEntity
}

// InvoiceResult は請求書検索結果。
type InvoiceResult struct {
	Invoice boardapi.InvoiceEntity
	Project *boardapi.ProjectEntity
	Client  *boardapi.ClientEntity
}

// VendorResult は仕入先検索結果。
type VendorResult struct {
	Vendor   boardapi.VendorEntity
	Branches []boardapi.VendorBranchEntity
	Contacts []boardapi.VendorContactEntity
}

// PurchaseOrderResult は発注書検索結果。
type PurchaseOrderResult struct {
	PurchaseOrder boardapi.PurchaseOrderEntity
	Vendor        *boardapi.VendorEntity
	Project       *boardapi.ProjectEntity
}

// PaymentResult は支払検索結果。
type PaymentResult struct {
	Payment boardapi.PaymentEntity
	Vendor  *boardapi.VendorEntity
	Project *boardapi.ProjectEntity
}

// UserResult はユーザー検索結果。
type UserResult struct {
	User boardapi.UserEntity
}
