package http

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/presence"
	"github.com/labuda/backend/internal/platform/response"
	"go.uber.org/zap"
)

const maxPresenceBatch = 100

// Handler is thin HTTP transport for initial Presence state.
// It delegates to presence.Service.BuildSnapshot as sole authority.
type Handler struct {
	svc *presence.Service
	log *zap.Logger
}

func NewHandler(svc *presence.Service, log *zap.Logger) *Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{svc: svc, log: log}
}

type presenceStateResponse struct {
	UserID     string  `json:"user_id"`
	IsOnline   bool    `json:"is_online"`
	LastSeenAt *string `json:"last_seen_at"`
	Version    int64   `json:"version"`
}

// GetPresence handles GET /api/v1/users/presence?user_ids=id1,id2
func (h *Handler) GetPresence(c *gin.Context) {
	ctx := c.Request.Context()

	viewerIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	viewerID, ok := viewerIDVal.(uuid.UUID)
	if !ok || viewerID == uuid.Nil {
		response.InternalServerError(c, "Invalid user context")
		return
	}

	// Support both `user_ids` (canonical) and `userIds` (legacy camel) and repeated param.
	// Primary: ?user_ids=id1,id2  Secondary: ?user_ids=id1&user_ids=id2
	rawValues := c.QueryArray("user_ids")
	if len(rawValues) == 0 {
		rawValues = c.QueryArray("userIds")
	}
	// Gin QueryArray returns single comma-joined value if only one param with commas; split further.
	var rawParts []string
	for _, v := range rawValues {
		parts := strings.Split(v, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				rawParts = append(rawParts, p)
			}
		}
	}
	// Fallback: single query param without QueryArray capture (gin quirk)
	if len(rawParts) == 0 {
		single := strings.TrimSpace(c.Query("user_ids"))
		if single == "" {
			single = strings.TrimSpace(c.Query("userIds"))
		}
		if single != "" {
			for _, p := range strings.Split(single, ",") {
				p = strings.TrimSpace(p)
				if p != "" {
					rawParts = append(rawParts, p)
				}
			}
		}
	}

	if len(rawParts) == 0 {
		response.Error(c, http.StatusBadRequest, "INVALID_INPUT", "user_ids is required")
		return
	}
	if len(rawParts) > maxPresenceBatch {
		response.Error(c, http.StatusBadRequest, "INVALID_INPUT", "too many user_ids: max 100")
		return
	}

	targetIDs := make([]uuid.UUID, 0, len(rawParts))
	for _, s := range rawParts {
		parsed, err := uuid.Parse(s)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "INVALID_INPUT", "invalid user_id: "+s)
			return
		}
		if parsed == uuid.Nil {
			response.Error(c, http.StatusBadRequest, "INVALID_INPUT", "invalid user_id: "+s)
			return
		}
		targetIDs = append(targetIDs, parsed)
	}

	if h.svc == nil {
		h.log.Error("Presence service unavailable")
		response.InternalServerError(c, "Presence service unavailable")
		return
	}

	states, err := h.svc.BuildSnapshot(ctx, viewerID, targetIDs)
	if err != nil {
		h.log.Error("BuildSnapshot failed", zap.Error(err), zap.String("viewer_id", viewerID.String()))
		response.InternalServerError(c, "Failed to retrieve presence")
		return
	}

	// Map to response contract; BuildSnapshot already viewer-scoped (block/lifecycle/privacy fail-closed)
	resp := make([]presenceStateResponse, 0, len(states))
	for _, st := range states {
		var lastSeenStr *string
		if st.LastSeenAt != nil {
			s := st.LastSeenAt.UTC().Format(time.RFC3339)
			lastSeenStr = &s
		}
		resp = append(resp, presenceStateResponse{
			UserID:     st.UserID.String(),
			IsOnline:   st.IsOnline,
			LastSeenAt: lastSeenStr,
			Version:    st.Version,
		})
	}

	response.Success(c, gin.H{"presences": resp})
}
