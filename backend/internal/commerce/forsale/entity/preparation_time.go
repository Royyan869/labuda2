package entity

// PreparationTime represents the seller's stated preparation time for shipping.
//
// BUSINESS TRUTH:
// - Preparation time is WHEN THE SELLER CAN SHIP AFTER CHECKOUT
//   (koi sometimes need karantina/quarantine before transport)
// - This is an EXPECTATION LAYER, not a fulfillment state machine
// - Buyers see this BEFORE purchase, orders get a snapshot at creation time
// - It tells the buyer the maximum time (upper bound) the seller needs to prepare the koi
//
// CANONICAL VALUES (owner decision 2026-10-02 — exactly 3 ranges, default 1–3):
// - 1_3_days: seller needs 1-3 days (DEFAULT)
// - 4_7_days: seller needs 4-7 days
// - 8_15_days: seller needs 8-15 days
//
// This enum is domain-native to the koi business, not generic marketplace logic.
type PreparationTime string

const (
	// PreparationTime1To3Days means seller needs 1-3 days (the default range).
	// Display: "1–3 hari"
	PreparationTime1To3Days PreparationTime = "1_3_days"

	// PreparationTime4To7Days means seller needs 4-7 days.
	// Display: "4–7 hari"
	PreparationTime4To7Days PreparationTime = "4_7_days"

	// PreparationTime8To15Days means seller needs 8-15 days.
	// Display: "8–15 hari"
	PreparationTime8To15Days PreparationTime = "8_15_days"
)

// IsValid returns true if this is a valid preparation time value
func (p PreparationTime) IsValid() bool {
	switch p {
	case PreparationTime1To3Days, PreparationTime4To7Days, PreparationTime8To15Days:
		return true
	}
	return false
}

// Days returns the preparation days for calculation purposes.
// The deadline is the UPPER bound of the promised range: the seller may take
// up to N days (1-3 → 3, 4-7 → 7, 8-15 → 15).
func (p PreparationTime) Days() int {
	switch p {
	case PreparationTime4To7Days:
		return 7
	case PreparationTime8To15Days:
		return 15
	case PreparationTime1To3Days:
		return 3
	default:
		return 3 // Default to the 1-3 day range for unknown values
	}
}

// DisplayLabel returns the user-facing Indonesian label.
func (p PreparationTime) DisplayLabel() string {
	switch p {
	case PreparationTime1To3Days:
		return "1–3 hari"
	case PreparationTime4To7Days:
		return "4–7 hari"
	case PreparationTime8To15Days:
		return "8–15 hari"
	default:
		return "Waktu kesiapan tidak diketahui"
	}
}

// Description returns a descriptive explanation for buyers.
func (p PreparationTime) Description() string {
	switch p {
	case PreparationTime1To3Days:
		return "Penjual perlu 1–3 hari untuk menyiapkan ikan setelah pembayaran"
	case PreparationTime4To7Days:
		return "Penjual perlu 4–7 hari untuk karantina/persiapan ikan"
	case PreparationTime8To15Days:
		return "Penjual perlu 8–15 hari untuk karantina/stabilisasi ikan"
	default:
		return "Hubungi penjual untuk estimasi pengiriman"
	}
}

// ParsePreparationTime parses a string into PreparationTime.
// Returns PreparationTime1To3Days for invalid/empty values (owner default).
func ParsePreparationTime(s string) PreparationTime {
	switch s {
	case string(PreparationTime1To3Days):
		return PreparationTime1To3Days
	case string(PreparationTime4To7Days):
		return PreparationTime4To7Days
	case string(PreparationTime8To15Days):
		return PreparationTime8To15Days
	default:
		return PreparationTime1To3Days // Safe default: owner-set 1–3 days
	}
}
