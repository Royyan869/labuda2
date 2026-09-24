package http

import (
	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/finance"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// PromoteBalanceFundingHandler owns the canonical Promote Balance read-only surface.
//
// Top-up is via the FundingIntent path (POST /promotions/contracts/payment-intent).
// This handler provides ONLY read-only balance queries.
type PromoteBalanceFundingHandler struct {
	db  db.Transactor
	log *zap.Logger
}

// NewPromoteBalanceFundingHandler wires the canonical funding handler.
func NewPromoteBalanceFundingHandler(
	db db.Transactor,
	log *zap.Logger,
) *PromoteBalanceFundingHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &PromoteBalanceFundingHandler{
		db:  db,
		log: log,
	}
}

// GetBalance handles GET /api/v1/promote-balance — returns the seller's
// current Promote Balance (projection from the immutable ledger; not an
// authority).
func (h *PromoteBalanceFundingHandler) GetBalance(c *gin.Context) {
	callerID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return
	}

	var balance int64
	err = h.db.WithTx(c.Request.Context(), func(tx db.Tx) error {
		return tx.QueryRow(c.Request.Context(),
			`SELECT COALESCE(balance, 0)
			 FROM financial_accounts
			 WHERE account_type = $1
			   AND user_id = $2
			   AND holder_id IS NULL`,
			finance.AccountPromoteBalance, callerID,
		).Scan(&balance)
	})

	if err != nil {
		h.log.Error("get promote balance failed", zap.Error(err), zap.String("seller_id", callerID.String()))
		response.InternalServerError(c, "Failed to read promote balance")
		return
	}

	response.Success(c, gin.H{
		"balance": balance,
	})
}
