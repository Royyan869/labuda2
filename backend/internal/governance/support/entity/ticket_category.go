package entity

// Category represents the category of a support ticket.
//
// CANONICAL TAXONOMY: these values are the single vocabulary used by the
// database enum (ticket_category_enum), the Go entity, HTTP binding/validation,
// request/response DTOs, the admin client and the mobile client. There is no
// translation layer between competing taxonomies.
type Category string

const (
	// CategoryOrderIssue is for order-related issues.
	CategoryOrderIssue Category = "order_issue"

	// CategoryPaymentIssue is for payment-related issues.
	CategoryPaymentIssue Category = "payment_issue"

	// CategoryAccountIssue is for account-related issues.
	CategoryAccountIssue Category = "account_issue"

	// CategoryListingIssue is for product/listing-related issues.
	CategoryListingIssue Category = "listing_issue"

	// CategoryShippingIssue is for shipping/delivery-related issues.
	CategoryShippingIssue Category = "shipping_issue"

	// CategoryRefundRequest is for refund-related requests.
	CategoryRefundRequest Category = "refund_request"

	// CategoryDispute is for dispute-related requests.
	CategoryDispute Category = "dispute"

	// CategoryTechnicalIssue is for technical issues.
	CategoryTechnicalIssue Category = "technical_issue"

	// CategoryOther is for anything not covered above.
	CategoryOther Category = "other"
)

// AllCategories is the canonical ordered list of valid categories. It is the
// single source consumed by validation and by clients' dropdowns.
var AllCategories = []Category{
	CategoryOrderIssue,
	CategoryPaymentIssue,
	CategoryAccountIssue,
	CategoryListingIssue,
	CategoryShippingIssue,
	CategoryRefundRequest,
	CategoryDispute,
	CategoryTechnicalIssue,
	CategoryOther,
}

// String returns the string representation of the category.
func (c Category) String() string {
	return string(c)
}

// IsValid checks if the category is valid.
func (c Category) IsValid() bool {
	for _, valid := range AllCategories {
		if c == valid {
			return true
		}
	}
	return false
}
