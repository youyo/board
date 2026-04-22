package boardapi

// DocumentSummary represents a document summary returned by the projects API
// when using response_group (estimate, order, delivery, invoice, receipt, all).
// 単一 struct + omitempty + nullable pointer 方式で全 rg バリアントを統一表現する（M43 同方針）。
// 欠落フィールドは zero-value／nil として自然に扱われる。
type DocumentSummary struct {
	// 既存フィールド（6）
	ID             int     `json:"id"`
	Message        *string `json:"message"`
	Total          string  `json:"total"`
	Tax            string  `json:"tax"`
	TaxWithholding string  `json:"tax_withholding"`
	LockFlg        int     `json:"lock_flg"`

	// 共通追加 5 フィールド（estimate/order/delivery/invoice/receipt 全てで出現）
	SealApprovalStatus    int                    `json:"seal_approval_status"`
	DocumentAmountDispKbn int                    `json:"document_amount_disp_kbn"`
	BlankDateFlg          int                    `json:"blank_date_flg"`
	DeliveryPlace         *string                `json:"delivery_place"`
	Details               []DocumentDetailEntity `json:"details"`

	// type-specific 追加 12 フィールド（omitempty で該当 rg のみ埋まる）

	// estimate のみ
	ValidPeriod *string `json:"valid_period,omitempty"`

	// order のみ
	DispOrderDate        *string `json:"disp_order_date,omitempty"`
	DispOrderReceiveDate *string `json:"disp_order_receive_date,omitempty"`

	// delivery のみ
	DeliveryDate            *string `json:"delivery_date,omitempty"`
	DispDeliveryDate        *string `json:"disp_delivery_date,omitempty"`
	DispDeliveryReceiveDate *string `json:"disp_delivery_receive_date,omitempty"`

	// invoice のみ
	InvoiceDate      *string `json:"invoice_date,omitempty"`
	PaymentLimitDate *string `json:"payment_limit_date,omitempty"`
	DispInvoiceDate  *string `json:"disp_invoice_date,omitempty"`
	MultiBankInfoFlg *int    `json:"multi_bank_info_flg,omitempty"`

	// receipt のみ
	ReceiptDate     *string `json:"receipt_date,omitempty"`
	DispReceiptDate *string `json:"disp_receipt_date,omitempty"`
}
