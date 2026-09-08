package http

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/audit"
	"github.com/labuda/backend/internal/middleware"
	promotionApp "github.com/labuda/backend/internal/pricing/promotion/application"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// PromotionHandler owns the canonical external product HTTP surface
// (seller CRUD + admin review). It is a promotion ASSET surface only — the
// canonical promotion lifecycle is promotion_contracts (/promotions/contracts).
type PromotionHandler struct {
	promotionService *promotionApp.PromotionService
	db               db.Transactor
	log              *zap.Logger
	adminAuditLogger audit.AdminAuditLogger
}

// NewPromotionHandler wires the external product handler. adminAuditLogger is
// optional (nil disables the admin review audit trail).
func NewPromotionHandler(
	promotionService *promotionApp.PromotionService,
	database db.Transactor,
	log *zap.Logger,
	auditLogger ...interface{},
) *PromotionHandler {
	if log == nil {
		log = zap.NewNop()
	}
	h := &PromotionHandler{
		promotionService: promotionService,
		db:               database,
		log:              log,
	}
	if len(auditLogger) > 0 {
		if al, ok := auditLogger[0].(audit.AdminAuditLogger); ok {
			h.adminAuditLogger = al
		}
	}
	return h
}

func (h *PromotionHandler) getUserID(c *gin.Context) (uuid.UUID, error) {
	return middleware.GetUserIDFromContext(c)
}