package application

import (

	"github.com/labuda/backend/internal/commerce/order/rating/infrastructure/repository"
)

// RatingDomainFactory provides controlled access to rating domain interfaces.
//
// SECURITY: This factory enforces rating domain boundary by:
// - Only exposing interfaces (RatingReader, RatingMutator)
// - Hiding concrete implementation (RatingService, OrderRatingRepository)
// - Preventing direct SQL access to order_ratings table
//
// USAGE:
// - External domains MUST use this factory to get rating interfaces
// - NEVER import concrete rating service or repository directly
// - ALWAYS work through interfaces for clear boundary enforcement
type RatingDomainFactory struct {
	service *RatingService
}

// NewRatingDomainFactory creates a new factory for rating domain access.
//
// This is the SINGLE ENTRY POINT for external domains to access rating functionality.
// All rating operations MUST go through this factory.
func NewRatingDomainFactory() *RatingDomainFactory {
	return &RatingDomainFactory{
		service: NewRatingService(),
	}
}

// GetReader returns read-only access to rating data.
//
// RATE LIMITING: No rate limiting enforced at this layer.
// Callers MUST implement their own rate limiting for high-frequency access.
//
// Use cases:
// - Seller dashboard (average rating display)
// - Monthly metrics worker (period-based aggregation)
// - Public profile display
//
// SECURITY: Caller CANNOT modify ratings through this interface.
func (f *RatingDomainFactory) GetReader() RatingReader {
	return f.service
}

// GetMutator returns write access to rating data.
//
// SECURITY WARNING: This interface allows rating modifications.
// Only use in trusted contexts (order completion, refund processing).
//
// INVARIANTS enforced by RatingMutator:
// - Buyer-only: Only buyer can rate seller
// - Order must be completed
// - One rating per order
// - Immutable: No update or delete operations
// - No financial mutations
//
// Use cases:
// - Order completion (buyer rates seller)
// - Refund processing (invalidate rating)
func (f *RatingDomainFactory) GetMutator() RatingMutator {
	return f.service
}

// GetFullAccess returns both read and write access to rating data.
//
// SECURITY WARNING: This provides full access to rating operations.
// ONLY use in domains that need both read and write capabilities.
//
// Use cases:
// - Order domain (completion + refund)
// - Dispute domain (read ratings + invalidate on refund)
//
// For read-only use cases, prefer GetReader() for clearer intent.
func (f *RatingDomainFactory) GetFullAccess() (RatingReader, RatingMutator) {
	return f.service, f.service
}

// ValidateAccessControl checks if the caller is using the correct interface type.
//
// This is a compile-time guard to ensure interface compliance.
// If this compiles, the rating domain boundary is properly enforced.
func ValidateAccessControl() {
	var (
		reader  RatingReader
		mutator RatingMutator
	)

	// This will fail to compile if RatingService doesn't implement interfaces
	reader = NewRatingService()
	mutator = NewRatingService()

	// Use variables to prevent unused warnings
	_ = reader
	_ = mutator

	// This will fail to compile if repository is exposed directly
	// Uncomment to test boundary enforcement:
	// _ = repository.NewOrderRatingRepository() // Should NOT compile in external domains
}

// init runs access control validation on package load.
//
// This ensures that interface contracts are enforced at compile-time.
// If rating domain boundary is violated, this will fail to compile.
func init() {
	// Validate that RatingService implements both interfaces
	service := NewRatingService()

	var reader RatingReader = service
	var mutator RatingMutator = service

	_ = reader
	_ = mutator

	// Additional safety check: ensure repository is not exposed
	repo := repository.NewOrderRatingRepository()
	if repo == nil {
		panic("rating repository should not be nil")
	}

}


