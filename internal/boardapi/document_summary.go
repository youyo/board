package boardapi

// DocumentSummary represents a document summary returned by the projects API
// when using response_group (estimate, order, delivery, invoice, receipt, all).
type DocumentSummary struct {
	ID             int     `json:"id"`
	Message        *string `json:"message"`
	Total          string  `json:"total"`
	Tax            string  `json:"tax"`
	TaxWithholding string  `json:"tax_withholding"`
	LockFlg        int     `json:"lock_flg"`
}
