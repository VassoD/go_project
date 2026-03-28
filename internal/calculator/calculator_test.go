package calculator

import (
	"fmt"
	"testing"
)

// TestCalculate uses Go's built-in "table-driven test" pattern.
// Instead of writing one test function per case, we declare a slice of test cases and loop over them.
func TestCalculate(t *testing.T) {
	tests := []struct {
		name       string
		gross      float64
		wantNet    float64
		wantComm   float64
		wantVAT    float64
		wantUrssaf float64
	}{
		{
			name:       "simple 100 euros",
			gross:      100.0,
			wantComm:   15.0, // 15% of 100
			wantVAT:    17.0, // 20% of 85
			wantUrssaf: 17.0, // 20% of 85
			wantNet:    51.0, // 85 - 17 - 17
		},
		{
			name:       "zero amount",
			gross:      0.0,
			wantComm:   0.0,
			wantVAT:    0.0,
			wantUrssaf: 0.0,
			wantNet:    0.0,
		},
		{
			name:       "50 euros",
			gross:      50.0,
			wantComm:   7.5,  // 15% of 50
			wantVAT:    8.5,  // 20% of 42.5
			wantUrssaf: 8.5,  // 20% of 42.5
			wantNet:    25.5, // 42.5 - 8.5 -> 8.5
		},
		{
			name:       "33.33 euros (driver-5, fractional rounding)",
			gross:      33.33,
			wantComm:   5.0,   // 33.33 * 15% = 4.9995 -> 5.00
			wantVAT:    5.67,  // 28.33 * 20% = 5.666 -> 5.67
			wantUrssaf: 5.67,  // 28.33 * 20% = 5.666 -> 5.67
			wantNet:    16.99, // 28.33 - 5.67 - 5.67 = 16.99
		},
	}

	for _, tc := range tests {
		// t.Run creates a sub-test with a name
		t.Run(tc.name, func(t *testing.T) {
			result := Calculate("driver-1", "daily", tc.gross)

			// Go has no assert library in stdlib, so we check manually and call t.Errorf.
			if result.Commission != tc.wantComm {
				t.Errorf("Commission: got %.2f, want %.2f", result.Commission, tc.wantComm)
			}
			if result.VAT != tc.wantVAT {
				t.Errorf("VAT: got %.2f, want %.2f", result.VAT, tc.wantVAT)
			}
			if result.Urssaf != tc.wantUrssaf {
				t.Errorf("Urssaf: got %.2f, want %.2f", result.Urssaf, tc.wantUrssaf)
			}
			if result.NetPayout != tc.wantNet {
				t.Errorf("NetPayout: got %.2f, want %.2f", result.NetPayout, tc.wantNet)
			}
		})
	}
}

func ExampleCalculate() {
	result := Calculate("driver-1", "daily", 100.0)

	fmt.Printf("Gross: %.2f\n", result.GrossAmount)
	fmt.Printf("Commission: %.2f\n", result.Commission)
	fmt.Printf("Net after commission: %.2f\n", result.NetAfterComm)
	fmt.Printf("VAT: %.2f\n", result.VAT)
	fmt.Printf("Urssaf: %.2f\n", result.Urssaf)
	fmt.Printf("Net payout: %.2f\n", result.NetPayout)

	// Output:
	// Gross: 100.00
	// Commission: 15.00
	// Net after commission: 85.00
	// VAT: 17.00
	// Urssaf: 17.00
	// Net payout: 51.00
}
