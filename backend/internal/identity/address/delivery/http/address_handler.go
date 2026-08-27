package http

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	addressApp "github.com/labuda/backend/internal/identity/address/application"
	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// AddressHandler handles address CRUD endpoints.
type AddressHandler struct {
	service     *addressApp.AddressService
	roleChecker auth.RoleChecker
	db          *db.DB
	log         *zap.Logger
}

// NewAddressHandler creates a new AddressHandler.
func NewAddressHandler(
	service *addressApp.AddressService,
	roleChecker auth.RoleChecker,
	database *db.DB,
	log *zap.Logger,
) *AddressHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &AddressHandler{
		service:     service,
		roleChecker: roleChecker,
		db:          database,
		log:         log,
	}
}

// ============================================================================
// DTOs
// ============================================================================

type createAddressRequest struct {
	Purpose       string   `json:"purpose" binding:"required"`
	Nickname      string   `json:"nickname"`
	RecipientName string   `json:"recipient_name" binding:"required"`
	Phone         string   `json:"phone" binding:"required"`
	ProvinceID    string   `json:"province_id" binding:"required"`
	ProvinceName  string   `json:"province_name"`
	CityID        string   `json:"city_id" binding:"required"`
	CityName      string   `json:"city_name"`
	DistrictID    string   `json:"district_id"`
	DistrictName  string   `json:"district_name"`
	VillageID     string   `json:"village_id"`
	VillageName   string   `json:"village_name"`
	StreetAddress string   `json:"street_address" binding:"required"`
	PostalCode    string   `json:"postal_code"`
	Notes         string   `json:"notes"`
	IsPrimary     bool     `json:"is_primary"`
	Latitude      *float64 `json:"latitude"`
	Longitude     *float64 `json:"longitude"`
}

type updateAddressRequest struct {
	Nickname      *string  `json:"nickname"`
	RecipientName *string  `json:"recipient_name"`
	Phone         *string  `json:"phone"`
	ProvinceID    *string  `json:"province_id"`
	ProvinceName  *string  `json:"province_name"`
	CityID        *string  `json:"city_id"`
	CityName      *string  `json:"city_name"`
	DistrictID    *string  `json:"district_id"`
	DistrictName  *string  `json:"district_name"`
	VillageID     *string  `json:"village_id"`
	VillageName   *string  `json:"village_name"`
	StreetAddress *string  `json:"street_address"`
	PostalCode    *string  `json:"postal_code"`
	Notes         *string  `json:"notes"`
	Latitude      *float64 `json:"latitude"`
	Longitude     *float64 `json:"longitude"`
}

type addressResponse struct {
	ID                     string   `json:"id"`
	UserID                 string   `json:"user_id"`
	Purpose                string   `json:"purpose"`
	PurposeLabel           string   `json:"purpose_label"`
	Nickname               string   `json:"nickname"`
	DisplayLabel           string   `json:"display_label"`
	RecipientName          string   `json:"recipient_name"`
	Phone                  string   `json:"phone"`
	ProvinceID             string   `json:"province_id"`
	ProvinceName           string   `json:"province_name"`
	CityID                 string   `json:"city_id"`
	CityName               string   `json:"city_name"`
	DistrictID             string   `json:"district_id"`
	DistrictName           string   `json:"district_name"`
	VillageID              string   `json:"village_id"`
	VillageName            string   `json:"village_name"`
	StreetAddress          string   `json:"street_address"`
	PostalCode             string   `json:"postal_code"`
	Notes                  string   `json:"notes"`
	IsPrimary              bool     `json:"is_primary"`
	IsAvailableForCheckout bool     `json:"is_available_for_checkout"`
	Latitude               *float64 `json:"latitude"`
	Longitude              *float64 `json:"longitude"`
	HasCoordinates         bool     `json:"has_coordinates"`
	FullAddress            string   `json:"full_address"`
	CreatedAt              string   `json:"created_at"`
	UpdatedAt              string   `json:"updated_at"`
}

type addressListResponse struct {
	Data  []addressResponse `json:"data"`
	Total int               `json:"total"`
}

type addressCountResponse struct {
	Total         int64 `json:"total"`
	ShippingCount int64 `json:"shipping_count"`
	SenderCount   int64 `json:"sender_count"`
}

// ============================================================================
// HELPERS
// ============================================================================

func purposeLabel(purpose string) string {
	switch purpose {
	case "shipping":
		return "Alamat Pengiriman"
	case "sender":
		return "Alamat Pengirim"
	default:
		return purpose
	}
}

func displayLabel(nickname, purpose string) string {
	label := purposeLabel(purpose)
	if nickname != "" {
		return fmt.Sprintf("%s (%s)", nickname, label)
	}
	return label
}

func buildFullAddress(a *addressEntity.Address) string {
	parts := []string{}
	if a.StreetAddress != "" {
		parts = append(parts, a.StreetAddress)
	}
	if a.VillageName != "" {
		parts = append(parts, a.VillageName)
	}
	if a.DistrictName != "" {
		parts = append(parts, a.DistrictName)
	}
	if a.CityName != "" {
		parts = append(parts, a.CityName)
	}
	if a.ProvinceName != "" {
		parts = append(parts, a.ProvinceName)
	}
	if a.PostalCode != "" {
		parts = append(parts, a.PostalCode)
	}
	result := strings.Join(parts, ", ")
	if result != "" {
		result += ", Indonesia"
	}
	return result
}

func toAddressResponse(a *addressEntity.Address) addressResponse {
	return addressResponse{
		ID:                     a.ID.String(),
		UserID:                 a.UserID.String(),
		Purpose:                string(a.Purpose),
		PurposeLabel:           purposeLabel(string(a.Purpose)),
		Nickname:               a.Nickname,
		DisplayLabel:           displayLabel(a.Nickname, string(a.Purpose)),
		RecipientName:          a.RecipientName,
		Phone:                  a.Phone,
		ProvinceID:             a.ProvinceID,
		ProvinceName:           a.ProvinceName,
		CityID:                 a.CityID,
		CityName:               a.CityName,
		DistrictID:             a.DistrictID,
		DistrictName:           a.DistrictName,
		VillageID:              a.VillageID,
		VillageName:            a.VillageName,
		StreetAddress:          a.StreetAddress,
		PostalCode:             a.PostalCode,
		Notes:                  a.Notes,
		IsPrimary:              a.IsPrimary,
		IsAvailableForCheckout: a.IsAvailableForCheckout,
		Latitude:               a.Latitude,
		Longitude:              a.Longitude,
		HasCoordinates:         a.Latitude != nil && a.Longitude != nil,
		FullAddress:            buildFullAddress(a),
		CreatedAt:              a.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:              a.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// ============================================================================
// HANDLERS
// ============================================================================

// CreateAddress handles POST /addresses
func (h *AddressHandler) CreateAddress(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return
	}

	var req createAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Validate purpose
	if !addressEntity.IsValidPurpose(req.Purpose) {
		response.BadRequest(c, "Invalid purpose: must be 'shipping' or 'sender'")
		return
	}

	tx, err := h.db.BeginTx(ctx)
	if err != nil {
		response.InternalError(c, "Failed to start transaction")
		return
	}
	defer tx.Rollback(ctx)

	address, err := h.service.CreateAddress(ctx, tx, addressApp.CreateAddressInput{
		UserID:        userID,
		Purpose:       req.Purpose,
		Nickname:      req.Nickname,
		RecipientName: req.RecipientName,
		Phone:         req.Phone,
		ProvinceID:    req.ProvinceID,
		ProvinceName:  req.ProvinceName,
		CityID:        req.CityID,
		CityName:      req.CityName,
		DistrictID:    req.DistrictID,
		DistrictName:  req.DistrictName,
		VillageID:     req.VillageID,
		VillageName:   req.VillageName,
		StreetAddress: req.StreetAddress,
		PostalCode:    req.PostalCode,
		Latitude:      req.Latitude,
		Longitude:     req.Longitude,
		Notes:         req.Notes,
		IsPrimary:     req.IsPrimary,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := tx.Commit(ctx); err != nil {
		response.InternalError(c, "Failed to commit transaction")
		return
	}

	response.Created(c, toAddressResponse(address))
}

// ListAddresses handles GET /addresses
func (h *AddressHandler) ListAddresses(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return
	}

	purpose := c.Query("purpose")
	if purpose != "" && !addressEntity.IsValidPurpose(purpose) {
		response.BadRequest(c, "Invalid purpose filter: must be 'shipping' or 'sender'")
		return
	}

	tx, err := h.db.BeginTx(ctx)
	if err != nil {
		response.InternalError(c, "Failed to start transaction")
		return
	}
	defer tx.Rollback(ctx)

	addresses, err := h.service.ListUserAddressesFiltered(ctx, tx, userID, purpose)
	if err != nil {
		response.InternalError(c, "Failed to list addresses")
		return
	}

	_ = tx.Commit(ctx)

	data := make([]addressResponse, len(addresses))
	for i, a := range addresses {
		data[i] = toAddressResponse(a)
	}

	response.Success(c, addressListResponse{
		Data:  data,
		Total: len(data),
	})
}

// GetPrimary handles GET /addresses/primary
func (h *AddressHandler) GetPrimary(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return
	}

	purpose := c.Query("purpose")
	if purpose != "" && !addressEntity.IsValidPurpose(purpose) {
		response.BadRequest(c, "Invalid purpose filter: must be 'shipping' or 'sender'")
		return
	}

	tx, err := h.db.BeginTx(ctx)
	if err != nil {
		response.InternalError(c, "Failed to start transaction")
		return
	}
	defer tx.Rollback(ctx)

	address, err := h.service.GetPrimaryFiltered(ctx, tx, userID, purpose)
	if err != nil {
		response.InternalError(c, "Failed to get primary address")
		return
	}

	_ = tx.Commit(ctx)

	if address == nil {
		response.NotFound(c, "No primary address set")
		return
	}

	response.Success(c, toAddressResponse(address))
}

// GetCount handles GET /addresses/count
func (h *AddressHandler) GetCount(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return
	}

	tx, err := h.db.BeginTx(ctx)
	if err != nil {
		response.InternalError(c, "Failed to start transaction")
		return
	}
	defer tx.Rollback(ctx)

	count, err := h.service.CountByUserID(ctx, tx, userID)
	if err != nil {
		response.InternalError(c, "Failed to count addresses")
		return
	}

	_ = tx.Commit(ctx)

	response.Success(c, addressCountResponse{
		Total:         count.Total,
		ShippingCount: count.ShippingCount,
		SenderCount:   count.SenderCount,
	})
}

// GetAddress handles GET /addresses/:id
func (h *AddressHandler) GetAddress(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return
	}

	addressID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid address ID")
		return
	}

	tx, err := h.db.BeginTx(ctx)
	if err != nil {
		response.InternalError(c, "Failed to start transaction")
		return
	}
	defer tx.Rollback(ctx)

	address, err := h.service.GetAddress(ctx, tx, addressID, userID)
	if err != nil {
		if addressApp.IsAddressNotOwnedError(err) {
			response.Forbidden(c, "Address not owned by user")
			return
		}
		response.NotFound(c, "Address not found")
		return
	}

	_ = tx.Commit(ctx)

	response.Success(c, toAddressResponse(address))
}

// UpdateAddress handles PUT /addresses/:id
func (h *AddressHandler) UpdateAddress(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return
	}

	addressID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid address ID")
		return
	}

	var req updateAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	tx, err := h.db.BeginTx(ctx)
	if err != nil {
		response.InternalError(c, "Failed to start transaction")
		return
	}
	defer tx.Rollback(ctx)

	// Get existing address to merge partial updates
	existing, err := h.service.GetAddress(ctx, tx, addressID, userID)
	if err != nil {
		if addressApp.IsAddressNotOwnedError(err) {
			response.Forbidden(c, "Address not owned by user")
			return
		}
		response.NotFound(c, "Address not found")
		return
	}

	// Build update input: merge existing with partial request
	input := addressApp.UpdateAddressInput{
		AddressID:     addressID,
		UserID:        userID,
		Purpose:       string(existing.Purpose),
		Nickname:      existing.Nickname,
		RecipientName: existing.RecipientName,
		Phone:         existing.Phone,
		ProvinceID:    existing.ProvinceID,
		ProvinceName:  existing.ProvinceName,
		CityID:        existing.CityID,
		CityName:      existing.CityName,
		DistrictID:    existing.DistrictID,
		DistrictName:  existing.DistrictName,
		VillageID:     existing.VillageID,
		VillageName:   existing.VillageName,
		StreetAddress: existing.StreetAddress,
		PostalCode:    existing.PostalCode,
		Latitude:      existing.Latitude,
		Longitude:     existing.Longitude,
		Notes:         existing.Notes,
	}

	if req.Nickname != nil {
		input.Nickname = *req.Nickname
	}
	if req.RecipientName != nil {
		input.RecipientName = *req.RecipientName
	}
	if req.Phone != nil {
		input.Phone = *req.Phone
	}
	if req.ProvinceID != nil {
		input.ProvinceID = *req.ProvinceID
	}
	if req.ProvinceName != nil {
		input.ProvinceName = *req.ProvinceName
	}
	if req.CityID != nil {
		input.CityID = *req.CityID
	}
	if req.CityName != nil {
		input.CityName = *req.CityName
	}
	if req.DistrictID != nil {
		input.DistrictID = *req.DistrictID
	}
	if req.DistrictName != nil {
		input.DistrictName = *req.DistrictName
	}
	if req.VillageID != nil {
		input.VillageID = *req.VillageID
	}
	if req.VillageName != nil {
		input.VillageName = *req.VillageName
	}
	if req.StreetAddress != nil {
		input.StreetAddress = *req.StreetAddress
	}
	if req.PostalCode != nil {
		input.PostalCode = *req.PostalCode
	}
	if req.Notes != nil {
		input.Notes = *req.Notes
	}
	if req.Latitude != nil {
		input.Latitude = req.Latitude
	}
	if req.Longitude != nil {
		input.Longitude = req.Longitude
	}

	address, err := h.service.UpdateAddress(ctx, tx, input)
	if err != nil {
		if addressApp.IsAddressNotOwnedError(err) {
			response.Forbidden(c, "Address not owned by user")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	if err := tx.Commit(ctx); err != nil {
		response.InternalError(c, "Failed to commit transaction")
		return
	}

	response.Success(c, toAddressResponse(address))
}

// DeleteAddress handles DELETE /addresses/:id
func (h *AddressHandler) DeleteAddress(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return
	}

	addressID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid address ID")
		return
	}

	tx, err := h.db.BeginTx(ctx)
	if err != nil {
		response.InternalError(c, "Failed to start transaction")
		return
	}
	defer tx.Rollback(ctx)

	if err := h.service.DeleteAddress(ctx, tx, addressID, userID); err != nil {
		if addressApp.IsAddressNotOwnedError(err) {
			response.Forbidden(c, "Address not owned by user")
			return
		}
		response.NotFound(c, "Address not found")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		response.InternalError(c, "Failed to commit transaction")
		return
	}

	response.NoContent(c)
}

// SetPrimary handles POST /addresses/:id/primary
func (h *AddressHandler) SetPrimary(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return
	}

	addressID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid address ID")
		return
	}

	tx, err := h.db.BeginTx(ctx)
	if err != nil {
		response.InternalError(c, "Failed to start transaction")
		return
	}
	defer tx.Rollback(ctx)

	if err := h.service.SetPrimary(ctx, tx, addressID, userID); err != nil {
		if addressApp.IsAddressNotOwnedError(err) {
			response.Forbidden(c, "Address not owned by user")
			return
		}
		response.NotFound(c, "Address not found")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		response.InternalError(c, "Failed to commit transaction")
		return
	}

	response.Success(c, gin.H{"message": "Primary address updated"})
}


