package calculator

import (
	"math"
	"vtc-service/internal/model"
)

// rates
const (
	commissionRate = 0.15 // 15% platform commission
	vatRate        = 0.20 // 20% VAT on net-after-commission
	urssafRate     = 0.20 // 20% Urssaf on net-after-commission
)

// Calculate takes a gross fare and returns a Balance with all deductions applied.
//
// The test specifies three deductions but not their exact base. My hypothesis is that:
//   - Commission (15%) is deducted first.
//   - VAT (20%) and Urssaf (20%) are both applied to net-after-commission
//
// Example for 100.00€ gross:
//   - Commission: 15% of 100.00 = 15.00
//   - Net after commission: 85.00
//   - VAT: 20% of 85.00 = 17.00
//   - Urssaf: 20% of 85.00 = 17.00
//   - Net payout: 85.00 - 17.00 - 17.00 = 51.00
func Calculate(driverID, period string, grossAmount float64) model.Balance {
	commission := round2(grossAmount * commissionRate)
	netAfterComm := round2(grossAmount - commission)

	vat := round2(netAfterComm * vatRate)
	urssaf := round2(netAfterComm * urssafRate)
	netPayout := round2(netAfterComm - vat - urssaf)

	return model.Balance{
		DriverID:     driverID,
		Period:       period,
		GrossAmount:  round2(grossAmount),
		Commission:   commission,
		NetAfterComm: netAfterComm,
		VAT:          vat,
		Urssaf:       urssaf,
		NetPayout:    netPayout,
	}
}

// this is to avoid float64 precision issues: 85 * 0.20 = 17.000000000000004 -> 17.0
func round2(x float64) float64 {
	return math.Round(x*100) / 100
}
