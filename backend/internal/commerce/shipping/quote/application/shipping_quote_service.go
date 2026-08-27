package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	forsaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	orderEntity "github.com/labuda/backend/internal/commerce/order/entity"
	shippingQuoteEntity "github.com/labuda/backend/internal/commerce/shipping/quote/entity"
	shippingQuoteRepo "github.com/labuda/backend/internal/commerce/shipping/quote/repository"
	chatvalidator "github.com/labuda/backend/internal/interaction/chat/attachmentvalidator"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"go.uber.org/zap"
)

// Shipping quote expiry bounds (PASS_18P). A shipping quote is a seller-issued
// commerce promise and must never be eternal. Every newly created quote gets
// a bounded ExpiresAt: callers may omit expires_in_hours (defaults to 24h) or
// request an explicit value up to the 7-day maximum; anything outside that
// range is rejected fail-closed rather than silently clamped.
const (
	// DefaultShippingQuoteExpiryHours is applied when the caller omits
	// expires_in_hours entirely.
	DefaultShippingQuoteExpiryHours = 24
	// MaxShippingQuoteExpiryHours is the maximum expiry any caller may
	// request (7 days).
	MaxShippingQuoteExpiryHours = 168
)

// Service handles shipping quote business logic.
//
// VALIDATION RULES (TASK A-G):
// - Quote scope: tied to chat_id, buyer_id, and product/sale-surface binding
// - One current unsuperseded quote per canonical context - enforced via row lock + supersession
// - Lifecycle: ACTIVE -> USED/EXPIRED
// - Address lock: destination_city_id, destination_province_id stored at creation
// - Quote price overrides all shipping options when used
// - Order snapshot stores quote_id, quote_price, destination, origin
//
// SECURITY:
// - Validates buyer_id match, product_id/source_type/source_id match, status ACTIVE
// - Address locked at quote creation, validated at checkout
type Service struct {
	db          db.Transactor
	quoteRepo   shippingQuoteRepo.ShippingQuoteRepository
	roomGetter  RoomGetter
	forSaleRepo ForSaleRepository
	auctionRepo AuctionQuoteReader
	chatService ChatMessageSender
	orderRepo   OrderRepository
	log         *zap.Logger
}

// RoomGetter defines the interface for getting chat rooms.
type RoomGetter interface {
	GetRoomByID(ctx context.Context, tx db.Tx, roomID uuid.UUID) (*chatEntity.ChatRoom, error)
	GetRoomByIDForUpdate(ctx context.Context, tx db.Tx, roomID uuid.UUID) (*chatEntity.ChatRoom, error)
}

// ForSaleRepository defines the interface for getting for_sale items.
type ForSaleRepository interface {
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*forsaleEntity.ForSale, error)
}

// AuctionQuoteReader defines the narrow auction lookup needed for shipping quote validation.
type AuctionQuoteReader interface {
	GetByID(ctx context.Context, tx db.Tx, auctionID uuid.UUID) (*auctionEntity.Auction, error)
}

// ChatMessageSender defines the narrow chat message capability needed by shipping quote creation.
type ChatMessageSender interface {
	SendMessage(
		ctx context.Context,
		roomID, senderID uuid.UUID,
		messageType chatEntity.MessageType,
		body *string,
		attachmentJSON map[string]interface{},
		idempotencyKey string,
	) (*chatEntity.ChatMessage, error)
}

// OrderRepository defines the interface for order operations needed by shipping quote service.
type OrderRepository interface {
	GetByShippingQuoteID(ctx context.Context, tx db.Tx, shippingQuoteID uuid.UUID) (*orderEntity.Order, error)
	CountValidOrdersByShippingQuoteID(ctx context.Context, tx db.Tx, shippingQuoteID uuid.UUID) (int64, error)
}

// NewService creates a new shipping quote service.
func NewService(
	db db.Transactor,
	quoteRepo shippingQuoteRepo.ShippingQuoteRepository,
	roomGetter RoomGetter,
	forSaleRepo ForSaleRepository,
	auctionRepo AuctionQuoteReader,
	chatService ChatMessageSender,
	orderRepo OrderRepository,
	log *zap.Logger,
) *Service {
	return &Service{
		db:          db,
		quoteRepo:   quoteRepo,
		roomGetter:  roomGetter,
		forSaleRepo: forSaleRepo,
		auctionRepo: auctionRepo,
		chatService: chatService,
		orderRepo:   orderRepo,
		log:         log,
	}
}

// CreateShippingQuoteInput holds the input for creating a shipping quote.
type CreateShippingQuoteInput struct {
	ChatID                uuid.UUID
	ProductID             uuid.UUID
	SourceType            string
	SourceID              uuid.UUID
	AuctionID             *uuid.UUID // Optional auction reference (TASK A)
	SellerID              uuid.UUID
	Cost                  money.Money
	Note                  *string
	DestinationCityID     *string // Optional address lock (TASK D)
	DestinationProvinceID *string // Optional address lock (TASK D)
	ExpiresInHours        *int    // Optional expiration in hours (TASK C)
}

// CreateShippingQuote creates a new shipping quote and sends a chat message.
//
// Business validation:
//   - Chat must exist
//   - Seller must be a participant in the chat
//   - ForSale/Auction must exist and belong to the seller
//   - Buyer is the other participant in the chat
//   - Cost must be non-negative
//   - Supersedes any existing unsuperseded quotes for this canonical context
//     before inserting the new current revision.
//
// Transaction flow:
// 1. BEGIN
// 2. Validate chat exists and seller is participant
// 3. Validate for_sale/auction exists and belongs to seller
// 4. Determine buyer (other participant)
// 5. Lock the canonical chat context
// 6. Supersede any prior unsuperseded revisions for the same context
// 7. Create new shipping quote with ACTIVE status
// 8. Send chat message with shipping_quote type
// 9. COMMIT
func (s *Service) CreateShippingQuote(ctx context.Context, input CreateShippingQuoteInput) (*shippingQuoteEntity.ShippingQuote, error) {
	if input.Cost.IsNegative() {
		return nil, fmt.Errorf("cost cannot be negative")
	}

	// Canonicalize expiry (PASS_18P). Backend is authoritative — mobile/admin
	// are never required to send expires_in_hours. Missing value defaults to
	// DefaultShippingQuoteExpiryHours; zero/negative and over-max values are
	// rejected fail-closed rather than silently clamped, so a caller mistake
	// surfaces immediately instead of producing a quote with an unintended
	// lifetime.
	expiryHours := DefaultShippingQuoteExpiryHours
	if input.ExpiresInHours != nil {
		if *input.ExpiresInHours <= 0 {
			return nil, fmt.Errorf("expires_in_hours must be positive")
		}
		if *input.ExpiresInHours > MaxShippingQuoteExpiryHours {
			return nil, fmt.Errorf("expires_in_hours must not exceed %d (7 days)", MaxShippingQuoteExpiryHours)
		}
		expiryHours = *input.ExpiresInHours
	}
	expiresAt := time.Now().Add(time.Duration(expiryHours) * time.Hour)

	// Determine if this is an auction quote
	isAuction := input.AuctionID != nil && *input.AuctionID != uuid.Nil

	var createdQuote *shippingQuoteEntity.ShippingQuote

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		// 1. Validate chat exists and lock the canonical context row.
		room, err := s.roomGetter.GetRoomByIDForUpdate(ctx, tx, input.ChatID)
		if err != nil {
			return fmt.Errorf("chat not found: %w", err)
		}

		if !room.HasParticipant(input.SellerID) {
			return fmt.Errorf("seller is not a participant in this chat")
		}

		// 2. Determine buyer (other participant)
		buyerID := room.OtherParticipant(input.SellerID)
		if buyerID == uuid.Nil {
			return fmt.Errorf("unable to determine buyer from chat participants")
		}

		// 2. Validate product/sale-surface exists and belongs to seller.
		var forSale *forsaleEntity.ForSale
		var auction *auctionEntity.Auction
		if isAuction {
			var err error
			auction, err = s.validateAuctionForQuote(ctx, tx, *input.AuctionID, input.SellerID, buyerID)
			if err != nil {
				return err
			}
		} else {
			// Validate product exists and belongs to seller.
			var err error
			forSale, err = s.forSaleRepo.GetByID(ctx, tx, input.ProductID)
			if err != nil {
				return fmt.Errorf("product not found: %w", err)
			}

			if forSale.SellerID != input.SellerID {
				return fmt.Errorf("product does not belong to seller")
			}
			if forSale.Status != forsaleEntity.ForSaleStatusActive {
				return fmt.Errorf("product is not available for shipping quotes: status=%s", forSale.Status)
			}
		}

		// 3. Generate the new quote ID before superseding prior rows.
		var quote *shippingQuoteEntity.ShippingQuote
		if isAuction {
			quote = shippingQuoteEntity.NewAuctionShippingQuote(
				input.ChatID,
				input.ProductID,
				*input.AuctionID,
				input.SourceType,
				input.SourceID,
				input.SellerID,
				buyerID,
				input.Cost,
				input.Note,
				input.DestinationCityID,
				input.DestinationProvinceID,
				expiresAt,
			)
		} else {
			quote = shippingQuoteEntity.NewShippingQuote(
				input.ChatID,
				input.ProductID,
				input.SourceType,
				input.SourceID,
				input.SellerID,
				buyerID,
				input.Cost,
				input.Note,
				input.DestinationCityID,
				input.DestinationProvinceID,
				expiresAt,
			)
		}

		// 4. Supersede any prior unsuperseded quotes for this canonical context.
		if _, err := s.quoteRepo.SupersedeCurrentQuotes(ctx, tx, input.ChatID, input.ProductID, input.SourceType, input.SourceID, input.SellerID, buyerID, quote.ID); err != nil {
			return fmt.Errorf("failed to supersede prior shipping quotes: %w", err)
		}

		// 5. Persist shipping quote
		if err := s.quoteRepo.Create(ctx, tx, quote); err != nil {
			return fmt.Errorf("failed to create shipping quote: %w", err)
		}

		// 6. Send chat message with shipping_quote type
		// UX TRUTH HARDENING: Include complete quote data with server-authoritative status
		attachmentJSON := buildShippingQuoteAttachmentJSONV2(quote, forSale, auction)
		if err := validateCanonicalAttachmentJSON(attachmentJSON); err != nil {
			return fmt.Errorf("invalid internal shipping quote attachment: %w", err)
		}

		idempotencyKey := fmt.Sprintf("shipping-quote.%s", quote.ID.String())

		_, err = s.chatService.SendMessage(
			ctx,
			quote.ChatID,
			quote.SellerID,
			chatEntity.MessageTypeShippingQuote,
			nil, // No body for shipping quote messages
			attachmentJSON,
			idempotencyKey,
		)
		if err != nil {
			return fmt.Errorf("failed to send chat message: %w", err)
		}

		createdQuote = quote
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.log.Info("shipping quote created",
		zap.String("quote_id", createdQuote.ID.String()),
		zap.String("chat_id", createdQuote.ChatID.String()),
		zap.String("product_id", createdQuote.ProductID.String()),
		zap.String("source_type", derefString(createdQuote.SourceType)),
		zap.String("source_id", derefUUID(createdQuote.SourceID)),
		zap.String("auction_id", func() string {
			if createdQuote.AuctionID != nil {
				return createdQuote.AuctionID.String()
			}
			return ""
		}()),
		zap.String("seller_id", createdQuote.SellerID.String()),
		zap.Int64("cost", createdQuote.Cost.Int64()),
		zap.String("status", string(createdQuote.Status)),
	)

	return createdQuote, nil
}

func (s *Service) validateAuctionForQuote(
	ctx context.Context,
	tx db.Tx,
	auctionID uuid.UUID,
	sellerID uuid.UUID,
	recipientID uuid.UUID,
) (*auctionEntity.Auction, error) {
	if s.auctionRepo == nil {
		return nil, fmt.Errorf("auction validation unavailable")
	}

	auction, err := s.auctionRepo.GetByID(ctx, tx, auctionID)
	if err != nil {
		return nil, fmt.Errorf("auction not found: %w", err)
	}
	if auction == nil {
		return nil, fmt.Errorf("auction not found: %s", auctionID)
	}
	if auction.SellerID != sellerID {
		return nil, fmt.Errorf("auction does not belong to seller")
	}
	if auction.Status != auctionEntity.StatusWaitingSettlement {
		return nil, fmt.Errorf("auction status is not waiting_settlement: auction_id=%s, status=%s", auction.ID, auction.Status)
	}
	winnerID := auction.WinnerID()
	if winnerID == nil {
		return nil, fmt.Errorf("auction winner is not set")
	}
	if *winnerID != recipientID {
		return nil, fmt.Errorf("chat recipient is not auction winner: auction_id=%s", auction.ID)
	}

	return auction, nil
}

// GetLatestByChatAndSource retrieves the current ACTIVE unsuperseded shipping
// quote that is also buyer-usable at the repository's authority time.
func (s *Service) GetLatestByChatAndSource(
	ctx context.Context,
	chatID, productID uuid.UUID,
	sourceType string,
	sourceID, sellerID, buyerID uuid.UUID,
) (*shippingQuoteEntity.ShippingQuote, error) {
	var quote *shippingQuoteEntity.ShippingQuote

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		quote, err = s.quoteRepo.GetLatestByChatAndSource(ctx, tx, chatID, productID, sourceType, sourceID, sellerID, buyerID)
		return err
	})

	if err != nil {
		return nil, err
	}

	return quote, nil
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func derefUUID(v *uuid.UUID) string {
	if v == nil {
		return ""
	}
	return v.String()
}

// GetByID retrieves a shipping quote by ID.
func (s *Service) GetByID(ctx context.Context, quoteID uuid.UUID) (*shippingQuoteEntity.ShippingQuote, error) {
	var quote *shippingQuoteEntity.ShippingQuote

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		quote, err = s.quoteRepo.GetByID(ctx, tx, quoteID)
		return err
	})

	if err != nil {
		return nil, err
	}

	return quote, nil
}

func validateCanonicalAttachmentJSON(attachment map[string]interface{}) error {
	errs := chatvalidator.ValidateAttachmentJSON(attachment)
	if !chatvalidator.HasValidationErrors(errs) {
		return nil
	}
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		parts = append(parts, fmt.Sprintf("%s: %s", e.Field, e.Message))
	}
	return errors.New(strings.Join(parts, "; "))
}

func buildShippingQuoteAttachmentJSONV2(
	quote *shippingQuoteEntity.ShippingQuote,
	forSale *forsaleEntity.ForSale,
	auction *auctionEntity.Auction,
) map[string]interface{} {
	data := map[string]interface{}{
		"offer_id":            quote.ID.String(),
		"status":              string(quote.Status),
		"rate":                quote.Cost.Int64(),
		"notes":               quote.Note,
		"valid_until":         quote.ExpiresAt,
		"seller_id":           quote.SellerID.String(),
		"shipping_type":       "manual",
		"shipping_type_name":  "Ongkir Manual",
		"shipping_type_emoji": "ðŸšš",
		"estimated_days":      nil,
	}

	if quote.SourceType != nil && *quote.SourceType == "auction" {
		data["linked_item_type"] = "auction"
		data["linked_item_id"] = quote.SourceID.String()
		data["auction_id"] = quote.SourceID.String()

		if auction != nil {
			linkedItemName := ""
			if auction.Product != nil {
				linkedItemName = auction.Product.Title
			}
			data["linked_item_name"] = linkedItemName
			if auction.CurrentBid != nil {
				data["linked_item_price"] = *auction.CurrentBid
			} else if auction.BuyNowPrice != nil {
				data["linked_item_price"] = *auction.BuyNowPrice
			} else {
				data["linked_item_price"] = quote.Cost.Int64()
			}
			if auction.BuyNowPrice != nil {
				data["linked_item_buy_now_price"] = *auction.BuyNowPrice
			}
		}
	} else {
		data["linked_item_type"] = "for_sale"
		data["linked_item_id"] = quote.SourceID.String()

		if forSale != nil {
			data["linked_item_name"] = forSale.Title
			data["linked_item_price"] = forSale.PricePerUnit.Int64()
			if imageURL := extractFirstForSaleMediaURL(forSale.MediaURLs); imageURL != nil {
				data["linked_item_image"] = *imageURL
			}
		}
	}

	return map[string]interface{}{
		"type": "shipping_quote",
		"data": data,
	}
}

func extractFirstForSaleMediaURL(mediaURLs json.RawMessage) *string {
	if len(mediaURLs) == 0 {
		return nil
	}

	var urls []string
	if err := json.Unmarshal(mediaURLs, &urls); err != nil || len(urls) == 0 {
		return nil
	}

	if urls[0] == "" {
		return nil
	}

	return &urls[0]
}

// ============================================================================
// TASK C - LIFECYCLE MANAGEMENT
// ============================================================================

// MarkQuoteUsed marks a shipping quote as USED.
// Called during order creation to prevent quote reuse (TASK C).
func (s *Service) MarkQuoteUsed(ctx context.Context, quoteID uuid.UUID) error {
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		// Fetch quote first to validate status
		quote, err := s.quoteRepo.GetByID(ctx, tx, quoteID)
		if err != nil {
			return fmt.Errorf("failed to fetch quote: %w", err)
		}
		if quote == nil {
			return fmt.Errorf("quote not found: %s", quoteID)
		}

		// Mark as used using entity method
		now := time.Now()
		if err := quote.MarkUsed(now); err != nil {
			return err
		}

		// Update in database
		usedAt := interface{}(now)
		return s.quoteRepo.UpdateStatus(ctx, tx, quoteID, quote.Status, &usedAt)
	})

	if err != nil {
		return err
	}

	s.log.Info("shipping quote marked as used",
		zap.String("quote_id", quoteID.String()),
	)

	return nil
}

// MarkQuoteExpired marks a shipping quote as EXPIRED.
// Can be called manually or by a background worker (TASK C).
func (s *Service) MarkQuoteExpired(ctx context.Context, quoteID uuid.UUID) error {
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		// Fetch quote first to validate status
		quote, err := s.quoteRepo.GetByID(ctx, tx, quoteID)
		if err != nil {
			return fmt.Errorf("failed to fetch quote: %w", err)
		}
		if quote == nil {
			return fmt.Errorf("quote not found: %s", quoteID)
		}

		// Mark as expired using entity method
		if err := quote.MarkExpired(); err != nil {
			return err
		}

		// Update in database
		return s.quoteRepo.UpdateStatus(ctx, tx, quoteID, quote.Status, nil)
	})

	if err != nil {
		return err
	}

	s.log.Info("shipping quote marked as expired",
		zap.String("quote_id", quoteID.String()),
	)

	return nil
}

// ============================================================================
// TASK G - SECURITY VALIDATION FOR CHECKOUT
// ============================================================================

// ValidateQuoteForCheckout performs comprehensive validation of a shipping quote
// before it can be used for checkout.
//
// VALIDATIONS (TASK G):
// 1. Quote exists and is ACTIVE
// 2. Quote is not expired (if expires_at is set)
// 3. Quote belongs to the buyer
// 4. Quote belongs to the for_sale/auction
// 5. Checkout address matches locked destination (TASK D)
//
// Returns the validated quote for use in order creation.
func (s *Service) ValidateQuoteForCheckout(
	ctx context.Context,
	quoteID uuid.UUID,
	buyerID uuid.UUID,
	productID uuid.UUID,
	sourceType string,
	sourceID uuid.UUID,
	shippingAddressProvinceID, shippingAddressCityID string,
) (*shippingQuoteEntity.ShippingQuote, error) {
	var quote *shippingQuoteEntity.ShippingQuote

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		quote, err = s.quoteRepo.GetByID(ctx, tx, quoteID)
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("failed to fetch quote: %w", err)
	}

	if quote == nil {
		return nil, fmt.Errorf("quote not found: %s", quoteID)
	}

	// VALIDATION 1: Quote must be ACTIVE
	if !quote.IsCurrent() {
		return nil, &QuoteValidationError{
			Reason:  fmt.Sprintf("quote is not current: current_status=%s", quote.Status),
			QuoteID: quoteID,
			Field:   "status",
		}
	}

	// VALIDATION 2: Quote must not be expired
	now := time.Now()
	if quote.IsExpiredAt(now) {
		return nil, &QuoteValidationError{
			Reason:  "quote has expired",
			QuoteID: quoteID,
			Field:   "expires_at",
		}
	}

	if quote.IsSuperseded() {
		return nil, &QuoteValidationError{
			Reason:  fmt.Sprintf("quote is superseded: superseded_by=%v", quote.SupersededByID),
			QuoteID: quoteID,
			Field:   "superseded_at",
		}
	}

	// VALIDATION 3: Quote must belong to the buyer
	if quote.BuyerID != buyerID {
		return nil, &QuoteValidationError{
			Reason:  fmt.Sprintf("quote buyer mismatch: quote_buyer=%s, checkout_buyer=%s", quote.BuyerID, buyerID),
			QuoteID: quoteID,
			Field:   "buyer_id",
		}
	}

	// VALIDATION 4: Quote must belong to the product/sale-surface
	if quote.ProductID != productID {
		return nil, &QuoteValidationError{
			Reason:  fmt.Sprintf("quote product mismatch: quote_product=%s, checkout_product=%s", quote.ProductID, productID),
			QuoteID: quoteID,
			Field:   "product_id",
		}
	}
	if quote.SourceType == nil || quote.SourceID == nil || *quote.SourceType != sourceType || *quote.SourceID != sourceID {
		return nil, &QuoteValidationError{
			Reason:  fmt.Sprintf("quote source mismatch: quote=%s:%s, checkout=%s:%s", derefString(quote.SourceType), derefUUID(quote.SourceID), sourceType, sourceID.String()),
			QuoteID: quoteID,
			Field:   "source_id",
		}
	}

	// VALIDATION 5: Checkout address must match locked destination (TASK D)
	if err := quote.ValidateDestinationAddress(shippingAddressProvinceID, shippingAddressCityID); err != nil {
		return nil, &QuoteValidationError{
			Reason:  err.Error(),
			QuoteID: quoteID,
			Field:   "destination_address",
		}
	}

	return quote, nil
}

// QuoteValidationError is returned when quote validation fails during checkout.
type QuoteValidationError struct {
	Reason  string
	QuoteID uuid.UUID
	Field   string
}

func (e *QuoteValidationError) Error() string {
	return fmt.Sprintf("quote validation failed for %s: %s", e.QuoteID, e.Reason)
}

// ============================================================================
// SHIPPING QUOTE REACTIVATION (HARD FIX - IDEMPOTENCY)
// ============================================================================

// ReactivateQuoteIfEligible reactivates a USED shipping quote back to ACTIVE status
// when the associated order has failed or expired, allowing the quote to be reused.
//
// TRIGGER POINTS (all inline, no outbox handler):
// - order_expired (StatusExpired) — payment window expired
// - payment_failed / full refund (StatusRefunded) — gateway refund or dispute refund
// - buyer_cancelled (StatusCancelled) — buyer cancels before payment
// - cancelled_timeout (StatusCancelledTimeout) — seller did not ship, buyer force-cancelled
//
// DOES NOT REACTIVATE for:
// - completed, shipped, partially_refunded, dispute_open
//
// VALIDATION RULES:
// 1. Quote must be in USED status
// 2. Reactivation count must be less than max_reuse (hardening: prevent infinite reuse)
// 3. Order using the quote must be in a terminal failure status
// 4. No other valid orders should be using this quote
// 5. This prevents duplicate orders while allowing quote reuse after genuine failures
//
// UPDATE:
// - status: USED -> ACTIVE
// - used_at: cleared (set to NULL)
// - expires_at: reset to 24 hours from now
// - reactivation_count: incremented
func (s *Service) ReactivateQuoteIfEligible(
	ctx context.Context,
	tx db.Tx,
	quoteID uuid.UUID,
) error {
	// STEP 1: Load the quote once to derive canonical context.
	initialQuote, err := s.quoteRepo.GetByID(ctx, tx, quoteID)
	if err != nil {
		return fmt.Errorf("failed to fetch quote: %w", err)
	}
	if initialQuote == nil {
		return fmt.Errorf("quote not found: %s", quoteID)
	}

	// STEP 2: Lock the canonical chat context row first to serialize quote
	// activation with replacement. Lock order: chat room -> shipping_quotes.
	if _, err := s.roomGetter.GetRoomByIDForUpdate(ctx, tx, initialQuote.ChatID); err != nil {
		return fmt.Errorf("failed to lock shipping quote context: %w", err)
	}

	// STEP 3: Re-fetch the quote under lock so state-sensitive checks observe
	// the transaction's latest row image.
	quote, err := s.quoteRepo.GetByIDForUpdate(ctx, tx, quoteID)
	if err != nil {
		return fmt.Errorf("failed to fetch quote for update: %w", err)
	}
	if quote == nil {
		return fmt.Errorf("quote not found: %s", quoteID)
	}

	// STEP 4: Validate quote is in USED status.
	if quote.Status != shippingQuoteEntity.QuoteStatusUsed {
		s.log.Debug("quote reactivation skipped: quote not in USED status",
			zap.String("quote_id", quoteID.String()),
			zap.String("current_status", string(quote.Status)),
		)
		return nil
	}

	if quote.IsSuperseded() {
		s.log.Warn("shipping_quote_stale_recovery",
			zap.String("quote_id", quoteID.String()),
			zap.String("chat_id", quote.ChatID.String()),
			zap.String("reason", "superseded"),
		)
		return fmt.Errorf("stale recovery rejected: quote %s has been superseded", quoteID)
	}

	// STEP 5: HARDENING - Check reactivation limit (prevent infinite reuse)
	if !quote.CanBeReactivated() {
		s.log.Warn("shipping_quote_reuse_limit_hit",
			zap.String("quote_id", quoteID.String()),
			zap.Int("reactivation_count", quote.ReactivationCount),
			zap.Int("max_reuse", quote.MaxReuse),
		)
		return fmt.Errorf("quote reuse limit exceeded: %d/%d reactivations used",
			quote.ReactivationCount, quote.MaxReuse)
	}

	// STEP 6: Detect a newer or current quote revision for the same context.
	if quote.SourceID == nil {
		return fmt.Errorf("quote source id missing for reactivation: quote_id=%s", quoteID)
	}
	sourceID := *quote.SourceID
	currentActive, err := s.quoteRepo.GetCurrentActiveByChatAndSource(ctx, tx, quote.ChatID, quote.ProductID, derefString(quote.SourceType), sourceID, quote.SellerID, quote.BuyerID)
	if err != nil {
		return fmt.Errorf("failed to fetch current active quote revision: %w", err)
	}
	if currentActive != nil && currentActive.ID != quoteID {
		s.log.Warn("shipping_quote_stale_recovery",
			zap.String("quote_id", quoteID.String()),
			zap.String("current_quote_id", currentActive.ID.String()),
			zap.String("reason", "newer_active_revision_exists"),
		)
		return fmt.Errorf("stale recovery rejected: newer current quote %s exists", currentActive.ID)
	}

	latestRevision, err := s.quoteRepo.GetLatestRevisionByChatAndSource(ctx, tx, quote.ChatID, quote.ProductID, derefString(quote.SourceType), sourceID, quote.SellerID, quote.BuyerID)
	if err != nil {
		return fmt.Errorf("failed to fetch latest quote revision: %w", err)
	}
	if latestRevision != nil && latestRevision.ID != quoteID {
		s.log.Warn("shipping_quote_stale_recovery",
			zap.String("quote_id", quoteID.String()),
			zap.String("latest_quote_id", latestRevision.ID.String()),
			zap.String("reason", "newer_revision_exists"),
		)
		return fmt.Errorf("stale recovery rejected: newer quote revision %s exists", latestRevision.ID)
	}

	// STEP 7: Fetch order using this quote.
	order, err := s.orderRepo.GetByShippingQuoteID(ctx, tx, quoteID)
	if err != nil {
		return fmt.Errorf("failed to fetch order by shipping quote: %w", err)
	}

	// STEP 8: Validate order status.
	// Only reactivate if order is in terminal failure states where shipping
	// was never successfully consumed (no delivery occurred).
	if order != nil {
		switch order.Status {
		case orderEntity.StatusRefunded,
			orderEntity.StatusExpired,
			orderEntity.StatusCancelled,
			orderEntity.StatusCancelledTimeout:
			// Terminal failure — quote should be reactivated
		default:
			// Order is still active, completed, or partially refunded — don't reactivate
			s.log.Debug("quote reactivation skipped: order not in terminal failure status",
				zap.String("quote_id", quoteID.String()),
				zap.String("order_id", order.ID.String()),
				zap.String("order_status", string(order.Status)),
			)
			return nil
		}
	}

	// STEP 9: Safety check - ensure no other valid orders are using this quote.
	validOrderCount, err := s.orderRepo.CountValidOrdersByShippingQuoteID(ctx, tx, quoteID)
	if err != nil {
		return fmt.Errorf("failed to count valid orders by shipping quote: %w", err)
	}

	if validOrderCount > 0 {
		// There are other valid orders using this quote, don't reactivate
		return fmt.Errorf("quote reactivation blocked: %d valid order(s) still using this quote", validOrderCount)
	}

	// STEP 10: Reactivate quote with full hardening.
	newExpiry := time.Now().Add(DefaultShippingQuoteExpiryHours * time.Hour)
	if err := quote.Reactivate(newExpiry); err != nil {
		return err
	}
	if err := s.quoteRepo.ReactivateQuote(ctx, tx, quoteID); err != nil {
		return fmt.Errorf("failed to reactivate quote: %w", err)
	}

	// STEP 11: Log the reactivation.
	s.log.Info("shipping_quote_reactivated",
		zap.String("quote_id", quoteID.String()),
		zap.String("previous_status", string(shippingQuoteEntity.QuoteStatusUsed)),
		zap.String("new_status", string(shippingQuoteEntity.QuoteStatusActive)),
		zap.Int("reactivation_count", quote.ReactivationCount),
		zap.Int("max_reuse", quote.MaxReuse),
		zap.String("trigger", "order_failure_or_expiry"),
	)

	return nil
}
