package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	"github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	forSaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	forSaleRepoImpl "github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	forSaleRepo "github.com/labuda/backend/internal/commerce/forsale/repository"
	"github.com/labuda/backend/internal/identity/auth"
	savedItemEntity "github.com/labuda/backend/internal/interaction/saved_item/entity"
	savedItemRepo "github.com/labuda/backend/internal/interaction/saved_item/repository"
	"github.com/labuda/backend/pkg/db"
)

// SavedItemService handles saved item operations (unified shortlist + auction watch)
type SavedItemService struct {
	savedItemRepo        savedItemRepo.SavedItemRepository
	forSaleRepo          forSaleRepo.ForSaleRepository
	auctionRepo          repository.AuctionRepository
	accountStatusChecker auth.AccountStatusChecker
	db                   *db.DB
}

// NewSavedItemService creates a new SavedItemService
func NewSavedItemService(accountStatusChecker auth.AccountStatusChecker) *SavedItemService {
	return &SavedItemService{
		forSaleRepo:          forSaleRepoImpl.NewForSaleRepository(),
		accountStatusChecker: accountStatusChecker,
	}
}

// SetSavedItemRepository sets the repository (for dependency injection)
func (s *SavedItemService) SetSavedItemRepository(repo savedItemRepo.SavedItemRepository) {
	s.savedItemRepo = repo
}

// SetAuctionRepository sets the auction repository (for dependency injection)
func (s *SavedItemService) SetAuctionRepository(repo repository.AuctionRepository) {
	s.auctionRepo = repo
}

// SetDB sets the database for read-only validation queries
func (s *SavedItemService) SetDB(d *db.DB) {
	s.db = d
}

// AddForSaleInput contains parameters for adding a forSale to saved items
type AddForSaleInput struct {
	UserID    uuid.UUID
	ForSaleID uuid.UUID
}

// AddAuctionInput contains parameters for adding an auction to saved items
type AddAuctionInput struct {
	UserID    uuid.UUID
	AuctionID uuid.UUID
}

// AddForSale adds a forSale to the user's saved items
// Validation:
// - forSale.status = active
// - for_sale.visibility = public
// - User cannot save their own forSale
// Returns existing item if already saved (idempotent)
func (s *SavedItemService) AddForSale(ctx context.Context, input AddForSaleInput) (*savedItemEntity.SavedItem, error) {
	// Get forSale for validation using a read-only transaction
	readTx, err := s.db.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin read tx: %w", err)
	}
	defer readTx.Rollback(ctx)
	forSale, err := s.forSaleRepo.GetByID(ctx, readTx, input.ForSaleID)
	if err != nil {
		return nil, fmt.Errorf("forSale not found: %w", err)
	}

	// Guard: ForSale must be active
	if forSale.Status != forSaleEntity.ForSaleStatusActive {
		return nil, &forSaleEntity.ForSaleNotActiveError{Status: forSale.Status}
	}

	// Guard: ForSale must be public
	if forSale.Visibility != forSaleEntity.ForSaleVisibilityPublic {
		return nil, &forSaleEntity.ForSaleNotAvailableError{
			ForSaleID: forSale.ID,
			Reason:    fmt.Sprintf("forSale not public: visibility=%s", forSale.Visibility),
		}
	}

	// Guard: User cannot add their own forSale to saved items
	if forSale.SellerID == input.UserID {
		return nil, fmt.Errorf("cannot save your own forSale")
	}

	// Check if item already exists
	existing, err := s.savedItemRepo.GetByUserAndTarget(ctx, input.UserID, savedItemEntity.TargetTypeForSale, input.ForSaleID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing saved item: %w", err)
	}

	if existing != nil {
		// Already saved - return existing (idempotent)
		return existing, nil
	}

	// Create new saved item
	item := savedItemEntity.NewSavedItem(input.UserID, savedItemEntity.TargetTypeForSale, input.ForSaleID, &forSale.SellerID)

	if err := s.savedItemRepo.Create(ctx, item); err != nil {
		return nil, fmt.Errorf("failed to create saved item: %w", err)
	}

	return item, nil
}

// AddAuction adds an auction to the user's saved items
// Validation:
// - Auction must exist
// - Auction status must not be "ended" or "cancelled"
// Returns existing item if already saved (idempotent)
func (s *SavedItemService) AddAuction(ctx context.Context, input AddAuctionInput) (*savedItemEntity.SavedItem, error) {
	// Get auction for validation using a read-only transaction
	readTx, err := s.db.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin read tx: %w", err)
	}
	defer readTx.Rollback(ctx)
	auction, err := s.auctionRepo.GetByID(ctx, readTx, input.AuctionID)
	if err != nil {
		return nil, fmt.Errorf("auction not found: %w", err)
	}

	// Guard: Auction must not be ended
	if auction.Status == auctionEntity.StatusEnded {
		return nil, fmt.Errorf("cannot watch ended auction")
	}

	// Guard: Auction must not be cancelled
	if auction.Status == auctionEntity.StatusCancelled {
		return nil, fmt.Errorf("cannot watch cancelled auction")
	}

	// Check if item already exists
	existing, err := s.savedItemRepo.GetByUserAndTarget(ctx, input.UserID, savedItemEntity.TargetTypeAuction, input.AuctionID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing saved item: %w", err)
	}

	if existing != nil {
		// Already saved - return existing (idempotent)
		return existing, nil
	}

	// Create new saved item (no seller_id for auctions)
	item := savedItemEntity.NewSavedItem(input.UserID, savedItemEntity.TargetTypeAuction, input.AuctionID, nil)

	if err := s.savedItemRepo.Create(ctx, item); err != nil {
		return nil, fmt.Errorf("failed to create saved item: %w", err)
	}

	return item, nil
}

// RemoveItem removes a saved item
func (s *SavedItemService) RemoveItem(ctx context.Context, userID uuid.UUID, targetType savedItemEntity.TargetType, targetID uuid.UUID) error {
	if err := s.savedItemRepo.Delete(ctx, userID, targetType, targetID); err != nil {
		return fmt.Errorf("failed to remove saved item: %w", err)
	}
	return nil
}

// RemoveForSale removes a saved forSale (convenience method)
func (s *SavedItemService) RemoveForSale(ctx context.Context, userID, forSaleID uuid.UUID) error {
	return s.RemoveItem(ctx, userID, savedItemEntity.TargetTypeForSale, forSaleID)
}

// RemoveAuction removes a saved auction (convenience method)
func (s *SavedItemService) RemoveAuction(ctx context.Context, userID, auctionID uuid.UUID) error {
	return s.RemoveItem(ctx, userID, savedItemEntity.TargetTypeAuction, auctionID)
}

// ClearByType removes all saved items of a type for a user
func (s *SavedItemService) ClearByType(ctx context.Context, userID uuid.UUID, targetType savedItemEntity.TargetType) error {
	if err := s.savedItemRepo.DeleteByType(ctx, userID, targetType); err != nil {
		return fmt.Errorf("failed to clear saved items by type: %w", err)
	}
	return nil
}

// ClearAll removes all saved items for a user
func (s *SavedItemService) ClearAll(ctx context.Context, userID uuid.UUID) error {
	if err := s.savedItemRepo.DeleteAll(ctx, userID); err != nil {
		return fmt.Errorf("failed to clear all saved items: %w", err)
	}
	return nil
}

// GetSavedItems retrieves the user's saved items with details
func (s *SavedItemService) GetSavedItems(ctx context.Context, userID uuid.UUID) (*savedItemEntity.SavedItemList, error) {
	forSales, err := s.savedItemRepo.GetByUserWithForSales(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get saved forSales: %w", err)
	}

	auctions, err := s.savedItemRepo.GetByUserWithAuctions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get saved auctions: %w", err)
	}

	list := &savedItemEntity.SavedItemList{
		UserID:   userID,
		Items:    forSales,
		Auctions: auctions,
		Total:    len(forSales) + len(auctions),
	}

	return list, nil
}

// Count returns the total number of saved items for a user
func (s *SavedItemService) Count(ctx context.Context, userID uuid.UUID) (int, error) {
	count, err := s.savedItemRepo.Count(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to count saved items: %w", err)
	}
	return count, nil
}

// CountByType returns the number of saved items by type
func (s *SavedItemService) CountByType(ctx context.Context, userID uuid.UUID, targetType savedItemEntity.TargetType) (int, error) {
	count, err := s.savedItemRepo.CountByType(ctx, userID, targetType)
	if err != nil {
		return 0, fmt.Errorf("failed to count saved items by type: %w", err)
	}
	return count, nil
}

// IsSaved checks if a user has saved an item
func (s *SavedItemService) IsSaved(ctx context.Context, userID uuid.UUID, targetType savedItemEntity.TargetType, targetID uuid.UUID) (bool, error) {
	item, err := s.savedItemRepo.GetByUserAndTarget(ctx, userID, targetType, targetID)
	if err != nil {
		return false, fmt.Errorf("failed to check if item is saved: %w", err)
	}
	return item != nil, nil
}
