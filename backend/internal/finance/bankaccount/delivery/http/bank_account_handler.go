package http

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	bankaccountApp "github.com/labuda/backend/internal/finance/bankaccount/application"
	bankaccountEntity "github.com/labuda/backend/internal/finance/bankaccount/entity"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// BankAccountManagementService is the interface the handler depends on.
// *application.BankAccountService satisfies it.
type BankAccountManagementService interface {
	CreateBankAccount(ctx context.Context, tx db.Tx, input bankaccountApp.CreateBankAccountInput) (*bankaccountEntity.BankAccount, error)
	GetBankAccount(ctx context.Context, tx db.Tx, bankAccountID uuid.UUID, sellerID uuid.UUID) (*bankaccountEntity.BankAccount, error)
	ListSellerBankAccounts(ctx context.Context, tx db.Tx, sellerID uuid.UUID) ([]*bankaccountEntity.BankAccount, error)
	SetDefaultBankAccount(ctx context.Context, tx db.Tx, bankAccountID uuid.UUID, sellerID uuid.UUID) error
	DeleteBankAccount(ctx context.Context, tx db.Tx, bankAccountID uuid.UUID, sellerID uuid.UUID) error
}

// BankAccountHandler handles HTTP requests for seller bank account management.
// Actor ID is always sourced from the auth context — never from request body.
type BankAccountHandler struct {
	service BankAccountManagementService
	db      *db.DB
	log     *zap.Logger
}

// NewBankAccountHandler creates a new BankAccountHandler.
func NewBankAccountHandler(
	service BankAccountManagementService,
	database *db.DB,
	log *zap.Logger,
) *BankAccountHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &BankAccountHandler{
		service: service,
		db:      database,
		log:     log,
	}
}

// ============================================================================
// Request / response DTOs
// ============================================================================

// CreateBankAccountRequest is the body for POST /api/v1/bank-accounts.
type CreateBankAccountRequest struct {
	BankName          string `json:"bank_name" binding:"required"`
	BankCode          string `json:"bank_code" binding:"required"`
	AccountNumber     string `json:"account_number" binding:"required"`
	AccountHolderName string `json:"account_holder_name" binding:"required"`
	IsDefault         bool   `json:"is_default"`
}

// BankAccountResponse is the response shape for a single bank account.
type BankAccountResponse struct {
	ID                string    `json:"id"`
	BankName          string    `json:"bank_name"`
	BankCode          string    `json:"bank_code"`
	AccountNumber     string    `json:"account_number"`
	AccountHolderName string    `json:"account_holder_name"`
	IsDefault         bool      `json:"is_default"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func toBankAccountResponse(ba *bankaccountEntity.BankAccount) BankAccountResponse {
	return BankAccountResponse{
		ID:                ba.ID.String(),
		BankName:          ba.BankName,
		BankCode:          ba.BankCode,
		AccountNumber:     ba.AccountNumber,
		AccountHolderName: ba.AccountHolderName,
		IsDefault:         ba.IsDefault,
		Status:            string(ba.Status),
		CreatedAt:         ba.CreatedAt,
		UpdatedAt:         ba.UpdatedAt,
	}
}

// ============================================================================
// Handlers
// ============================================================================

// CreateBankAccount handles POST /api/v1/bank-accounts.
//
// Actor ID is sourced exclusively from the auth context (set by AuthMiddleware).
// Any seller_id field in the request body is ignored.
func (h *BankAccountHandler) CreateBankAccount(c *gin.Context) {
	ctx := c.Request.Context()

	sellerID, ok := h.actorSellerID(c)
	if !ok {
		return
	}

	var req CreateBankAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	var created *bankaccountEntity.BankAccount
	if err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		created, err = h.service.CreateBankAccount(ctx, tx, bankaccountApp.CreateBankAccountInput{
			UserID:            sellerID,
			BankName:          req.BankName,
			BankCode:          req.BankCode,
			AccountNumber:     req.AccountNumber,
			AccountHolderName: req.AccountHolderName,
			IsDefault:         req.IsDefault,
		})
		return err
	}); err != nil {
		h.log.Warn("bank_account_create_failed",
			zap.String("seller_id", sellerID.String()),
			zap.Error(err),
		)
		response.BadRequest(c, err.Error())
		return
	}

	h.log.Info("bank_account_created",
		zap.String("seller_id", sellerID.String()),
		zap.String("bank_account_id", created.ID.String()),
	)
	response.Created(c, toBankAccountResponse(created))
}

// ListBankAccounts handles GET /api/v1/bank-accounts.
// Returns only accounts owned by the authenticated seller.
func (h *BankAccountHandler) ListBankAccounts(c *gin.Context) {
	ctx := c.Request.Context()

	sellerID, ok := h.actorSellerID(c)
	if !ok {
		return
	}

	var accounts []*bankaccountEntity.BankAccount
	if err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		accounts, err = h.service.ListSellerBankAccounts(ctx, tx, sellerID)
		return err
	}); err != nil {
		h.log.Error("bank_account_list_failed",
			zap.String("seller_id", sellerID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to list bank accounts")
		return
	}

	resp := make([]BankAccountResponse, 0, len(accounts))
	for _, a := range accounts {
		resp = append(resp, toBankAccountResponse(a))
	}
	response.Success(c, resp)
}

// GetBankAccount handles GET /api/v1/bank-accounts/:id.
// Validates that the account belongs to the authenticated seller.
func (h *BankAccountHandler) GetBankAccount(c *gin.Context) {
	ctx := c.Request.Context()

	sellerID, ok := h.actorSellerID(c)
	if !ok {
		return
	}

	accountID, ok := h.parseAccountID(c)
	if !ok {
		return
	}

	var account *bankaccountEntity.BankAccount
	if err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		account, err = h.service.GetBankAccount(ctx, tx, accountID, sellerID)
		return err
	}); err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "not belong") {
			response.NotFound(c, "Bank account not found")
			return
		}
		response.InternalServerError(c, "Failed to get bank account")
		return
	}

	response.Success(c, toBankAccountResponse(account))
}

// SetDefaultBankAccount handles PATCH /api/v1/bank-accounts/:id/default.
// Sets the specified account as the seller's default for withdrawals.
// Actor ID is sourced from auth context.
func (h *BankAccountHandler) SetDefaultBankAccount(c *gin.Context) {
	ctx := c.Request.Context()

	sellerID, ok := h.actorSellerID(c)
	if !ok {
		return
	}

	accountID, ok := h.parseAccountID(c)
	if !ok {
		return
	}

	if err := h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.service.SetDefaultBankAccount(ctx, tx, accountID, sellerID)
	}); err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "not active") {
			response.NotFound(c, "Bank account not found")
			return
		}
		h.log.Warn("bank_account_set_default_failed",
			zap.String("seller_id", sellerID.String()),
			zap.String("account_id", accountID.String()),
			zap.Error(err),
		)
		response.BadRequest(c, err.Error())
		return
	}

	h.log.Info("bank_account_set_default",
		zap.String("seller_id", sellerID.String()),
		zap.String("account_id", accountID.String()),
	)
	response.NoContent(c)
}

// DeleteBankAccount handles DELETE /api/v1/bank-accounts/:id.
// Soft-deletes the account. Blocked if an active withdrawal exists.
// Actor ID is sourced from auth context.
func (h *BankAccountHandler) DeleteBankAccount(c *gin.Context) {
	ctx := c.Request.Context()

	sellerID, ok := h.actorSellerID(c)
	if !ok {
		return
	}

	accountID, ok := h.parseAccountID(c)
	if !ok {
		return
	}

	if err := h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.service.DeleteBankAccount(ctx, tx, accountID, sellerID)
	}); err != nil {
		if strings.Contains(err.Error(), "not found") {
			response.NotFound(c, "Bank account not found")
			return
		}
		if strings.Contains(err.Error(), "active withdrawal") || strings.Contains(err.Error(), "already deleted") {
			response.BadRequest(c, err.Error())
			return
		}
		h.log.Warn("bank_account_delete_failed",
			zap.String("seller_id", sellerID.String()),
			zap.String("account_id", accountID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to delete bank account")
		return
	}

	h.log.Info("bank_account_deleted",
		zap.String("seller_id", sellerID.String()),
		zap.String("account_id", accountID.String()),
	)
	response.NoContent(c)
}

// ============================================================================
// Internal helpers
// ============================================================================

// actorSellerID extracts the authenticated user's ID from context.
// Returns false and writes an error response if the ID is missing or invalid.
func (h *BankAccountHandler) actorSellerID(c *gin.Context) (uuid.UUID, bool) {
	id, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return uuid.Nil, false
	}
	return id, true
}

// parseAccountID parses the :id path parameter as a UUID.
func (h *BankAccountHandler) parseAccountID(c *gin.Context) (uuid.UUID, bool) {
	raw := c.Param("id")
	id, err := uuid.Parse(raw)
	if err != nil {
		response.BadRequest(c, "Invalid bank account ID")
		return uuid.Nil, false
	}
	return id, true
}


