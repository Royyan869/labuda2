package entity

// PreparationTime represents the seller's stated preparation time for shipping.
//
// BUSINESS TRUTH:
// - Seller declares how long they need BEFORE the item can be shipped
// - This is an EXPECTATION LAYER, not a fulfillment state machine
// - Buyers see this BEFORE purchase, orders get a snapshot at creation time
// - Different koi need different preparation (karantina, puasa, stabilisasi)
//
// CANONICAL VALUES:
// - immediate: Ready to ship right away (0 days)
// - short: 1-2 days preparation
// - medium: 3-5 days preparation
// - long: 7+ days preparation
//
// This enum is domain-native to the koi business, not generic marketplace logic.
type PreparationTime string

const (
	// PreparationTimeImmediate means seller can ship immediately (0 days)
	// Display: "Siap kirim langsung"
	PreparationTimeImmediate PreparationTime = "immediate"

	// PreparationTimeShort means seller needs 1-2 days
	// Display: "1–2 hari"
	PreparationTimeShort PreparationTime = "short"

	// PreparationTimeMedium means seller needs 3-5 days
	// Display: "3–5 hari"
	PreparationTimeMedium PreparationTime = "medium"

	// PreparationTimeLong means seller needs 7+ days
	// Display: "7+ hari"
	PreparationTimeLong PreparationTime = "long"
)

// IsValid returns true if this is a valid preparation time value
func (p PreparationTime) IsValid() bool {
	switch p {
	case PreparationTimeImmediate, PreparationTimeShort, PreparationTimeMedium, PreparationTimeLong:
		return true
	}
	return false
}

// Days returns the preparation days for calculation purposes.
// For "long" (7+), we use 7 as the baseline.
func (p PreparationTime) Days() int {
	switch p {
	case PreparationTimeImmediate:
		return 0
	case PreparationTimeShort:
		return 2
	case PreparationTimeMedium:
		return 5
	case PreparationTimeLong:
		return 7
	default:
		return 0 // Default to immediate for unknown values
	}
}

// DisplayLabel returns the user-facing Indonesian label.
func (p PreparationTime) DisplayLabel() string {
	switch p {
	case PreparationTimeImmediate:
		return "Siap kirim langsung"
	case PreparationTimeShort:
		return "1–2 hari"
	case PreparationTimeMedium:
		return "3–5 hari"
	case PreparationTimeLong:
		return "7+ hari"
	default:
		return "Waktu kesiapan tidak diketahui"
	}
}

// Description returns a descriptive explanation for buyers.
func (p PreparationTime) Description() string {
	switch p {
	case PreparationTimeImmediate:
		return "Penjual siap mengirim ikan segera setelah pembayaran"
	case PreparationTimeShort:
		return "Penjual mungkin perlu 1–2 hari untuk persiapan pengiriman"
	case PreparationTimeMedium:
		return "Penjual mungkin perlu 3–5 hari untuk karantina/persiapan ikan"
	case PreparationTimeLong:
		return "Penjual mungkin perlu 7+ hari untuk karantina/stabilisasi ikan"
	default:
		return "Hubungi penjual untuk estimasi pengiriman"
	}
}

// ParsePreparationTime parses a string into PreparationTime.
// Returns PreparationTimeImmediate for invalid/empty values (safe default).
func ParsePreparationTime(s string) PreparationTime {
	switch s {
	case string(PreparationTimeImmediate):
		return PreparationTimeImmediate
	case string(PreparationTimeShort):
		return PreparationTimeShort
	case string(PreparationTimeMedium):
		return PreparationTimeMedium
	case string(PreparationTimeLong):
		return PreparationTimeLong
	default:
		return PreparationTimeImmediate // Safe default
	}
}




