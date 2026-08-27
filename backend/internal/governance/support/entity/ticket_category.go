package entity

// Category represents the category of a support ticket.
type Category string

const (
	// CategoryPayment is for payment-related issues.
	CategoryPayment Category = "payment"

	// CategoryOrder is for order-related issues.
	CategoryOrder Category = "order"

	// CategoryTechnical is for technical issues.
	CategoryTechnical Category = "technical"

	// CategoryAccount is for account-related issues.
	CategoryAccount Category = "account"

	// CategoryGeneral is for general inquiries.
	CategoryGeneral Category = "general"
)

// String returns the string representation of the category.
func (c Category) String() string {
	return string(c)
}

// IsValid checks if the category is valid.
func (c Category) IsValid() bool {
	switch c {
	case CategoryPayment, CategoryOrder, CategoryTechnical, CategoryAccount, CategoryGeneral:
		return true
	default:
		return false
	}
}


