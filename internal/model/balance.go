package model

// Balance is the response type for GET /balances.
// It shows how much a driver earns after all deductions.
type Balance struct {
	DriverID     string  `json:"driver_id"`
	Period       string  `json:"period"`
	GrossAmount  float64 `json:"gross_amount"`
	Commission   float64 `json:"commission"`
	NetAfterComm float64 `json:"net_after_commission"`
	VAT          float64 `json:"vat"`
	Urssaf       float64 `json:"urssaf"`
	NetPayout    float64 `json:"net_payout"`
}
