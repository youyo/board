package boardapi

// DocumentDetailEntity は document 系 Entity（estimate/order/delivery/receipt）
// の details 配列要素。4 docType で構造が完全に一致するため共通化する。
type DocumentDetailEntity struct {
	No                    int     `json:"no"`
	DetailDate            *string `json:"detail_date"`
	Description           string  `json:"description"`
	Quantity              *string `json:"quantity"`
	Unit                  *string `json:"unit"`
	UnitPrice             *string `json:"unit_price"`
	Price                 *string `json:"price"`
	TaxRate               string  `json:"tax_rate"`
	TaxWithholdingFlg     int     `json:"tax_withholding_flg"`
	TaxIncludedFlg        int     `json:"tax_included_flg"`
	ReducedTaxRateKbn     int     `json:"reduced_tax_rate_kbn"`
	SectionDescription    *string `json:"section_description"`
	SectionSubtotal       *string `json:"section_subtotal"`
	DocumentDetailKbn     int     `json:"document_detail_kbn"`
	DocumentDetailKbnName string  `json:"document_detail_kbn_name"`
	DeductionApplicable   bool    `json:"deduction_applicable"`
}
