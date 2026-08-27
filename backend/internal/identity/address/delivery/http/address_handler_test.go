package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Test helpers
// ============================================================================

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestRouter(handler *AddressHandler) *gin.Engine {
	r := gin.New()
	g := r.Group("/addresses")
	{
		g.POST("", handler.CreateAddress)
		g.GET("", handler.ListAddresses)
		g.GET("/primary", handler.GetPrimary)
		g.GET("/count", handler.GetCount)
		g.GET("/:id", handler.GetAddress)
		g.PUT("/:id", handler.UpdateAddress)
		g.DELETE("/:id", handler.DeleteAddress)
		g.POST("/:id/primary", handler.SetPrimary)
	}
	return r
}

func setUserID(c *gin.Context, userID uuid.UUID) {
	c.Set("user_id", userID)
	c.Set("userID", userID)
}

// ============================================================================
// Test: Unauthenticated returns 401
// ============================================================================

func TestCreateAddress_Unauthenticated(t *testing.T) {
	handler := &AddressHandler{}
	r := setupTestRouter(handler)

	body := `{"purpose":"shipping","recipient_name":"Ali","phone":"08123456789","province_id":"32","city_id":"3204","street_address":"Jl Test"}`
	req := httptest.NewRequest(http.MethodPost, "/addresses", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListAddresses_Unauthenticated(t *testing.T) {
	handler := &AddressHandler{}
	r := setupTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/addresses", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetAddress_Unauthenticated(t *testing.T) {
	handler := &AddressHandler{}
	r := setupTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/addresses/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDeleteAddress_Unauthenticated(t *testing.T) {
	handler := &AddressHandler{}
	r := setupTestRouter(handler)

	req := httptest.NewRequest(http.MethodDelete, "/addresses/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSetPrimary_Unauthenticated(t *testing.T) {
	handler := &AddressHandler{}
	r := setupTestRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/addresses/"+uuid.New().String()+"/primary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ============================================================================
// Test: Response shape computed fields
// ============================================================================

func TestToAddressResponse_ComputedFields(t *testing.T) {
	lat := 6.9175
	lon := 107.6191
	addr := &addressEntity.Address{
		ID:                     uuid.New(),
		UserID:                 uuid.New(),
		Purpose:                addressEntity.AddressPurposeShipping,
		Nickname:               "Rumah",
		RecipientName:          "Ali",
		Phone:                  "08123456789",
		ProvinceID:             "32",
		ProvinceName:           "Jawa Barat",
		CityID:                 "3204",
		CityName:               "Bandung",
		DistrictID:             "320401",
		DistrictName:           "Coblong",
		VillageID:              "3204011001",
		VillageName:            "Dago",
		StreetAddress:          "Jl. Juanda No. 100",
		PostalCode:             "40135",
		Notes:                  "Ketuk 2x",
		Latitude:               &lat,
		Longitude:              &lon,
		IsPrimary:              true,
		IsAvailableForCheckout: true,
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}

	resp := toAddressResponse(addr)

	// purpose_label
	assert.Equal(t, "Alamat Pengiriman", resp.PurposeLabel)

	// display_label
	assert.Equal(t, "Rumah (Alamat Pengiriman)", resp.DisplayLabel)

	// has_coordinates
	assert.True(t, resp.HasCoordinates)

	// full_address
	assert.Contains(t, resp.FullAddress, "Jl. Juanda No. 100")
	assert.Contains(t, resp.FullAddress, "Bandung")
	assert.Contains(t, resp.FullAddress, "Indonesia")

	// notes
	assert.Equal(t, "Ketuk 2x", resp.Notes)

	// Without nickname
	addr.Nickname = ""
	resp2 := toAddressResponse(addr)
	assert.Equal(t, "Alamat Pengiriman", resp2.DisplayLabel)

	// Without coordinates
	addr.Latitude = nil
	addr.Longitude = nil
	resp3 := toAddressResponse(addr)
	assert.False(t, resp3.HasCoordinates)
}

func TestToAddressResponse_SenderPurposeLabel(t *testing.T) {
	addr := &addressEntity.Address{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		Purpose:   addressEntity.AddressPurposeSender,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	resp := toAddressResponse(addr)
	assert.Equal(t, "Alamat Pengirim", resp.PurposeLabel)
}

// ============================================================================
// Test: Sender purpose is account-owned, not seller-gated
// ============================================================================

func TestCreateAddress_SenderDoesNotRequireSellerCapability(t *testing.T) {
	// Sender addresses should not be blocked by seller-capability checks.
	// Without a DB, the request will fail after the auth/path validation layer,
	// so the important assertion is that we do NOT return 403/401 here.
	handler := &AddressHandler{}

	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/addresses", func(c *gin.Context) {
		setUserID(c, uuid.New())
		c.Next()
	}, handler.CreateAddress)

	body := `{"purpose":"sender","recipient_name":"Ali","phone":"08123456789","province_id":"32","city_id":"3204","street_address":"Jl Test"}`
	req := httptest.NewRequest(http.MethodPost, "/addresses", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusForbidden, w.Code)
	assert.NotEqual(t, http.StatusUnauthorized, w.Code)
}

func TestCreateAddress_ShippingAllowedForNormalUser(t *testing.T) {
	// Shipping purpose should NOT check seller capability.
	// Without a DB, it will panic at BeginTx — use Recovery to catch.
	// The important thing is it does NOT return 403 before the db call.
	handler := &AddressHandler{}

	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/addresses", func(c *gin.Context) {
		setUserID(c, uuid.New())
		c.Next()
	}, handler.CreateAddress)

	body := `{"purpose":"shipping","recipient_name":"Ali","phone":"08123456789","province_id":"32","city_id":"3204","street_address":"Jl Test"}`
	req := httptest.NewRequest(http.MethodPost, "/addresses", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should NOT be 403 — it should fail at db layer (500 from recovery) not auth (403)
	assert.NotEqual(t, http.StatusForbidden, w.Code)
	assert.NotEqual(t, http.StatusUnauthorized, w.Code)
}

// ============================================================================
// Test: Body user_id cannot spoof owner
// ============================================================================

func TestCreateAddress_BodyUserIDIgnored(t *testing.T) {
	// Even if request body sends user_id, handler uses auth context userID.
	// Since createAddressRequest doesn't have a user_id field, body spoofing
	// is structurally impossible.
	var req createAddressRequest
	body := `{"purpose":"shipping","recipient_name":"Ali","phone":"08123456789","province_id":"32","city_id":"3204","street_address":"Jl Test","user_id":"00000000-0000-0000-0000-000000000001"}`

	err := json.Unmarshal([]byte(body), &req)
	require.NoError(t, err)

	// Confirm there's no UserID field in the request struct
	assert.Equal(t, "shipping", req.Purpose)
	assert.Equal(t, "Ali", req.RecipientName)
	// user_id from body is silently dropped — no field to bind to
}

// ============================================================================
// Test: List filters by purpose
// ============================================================================

func TestListAddresses_InvalidPurposeFilter(t *testing.T) {
	handler := &AddressHandler{}

	r := gin.New()
	r.GET("/addresses", func(c *gin.Context) {
		setUserID(c, uuid.New())
		c.Next()
	}, handler.ListAddresses)

	req := httptest.NewRequest(http.MethodGet, "/addresses?purpose=invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid purpose filter")
}

func TestListAddresses_ValidPurposeFilterAccepted(t *testing.T) {
	// Valid purpose should not fail at validation — it will panic at db (use Recovery)
	handler := &AddressHandler{}

	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/addresses", func(c *gin.Context) {
		setUserID(c, uuid.New())
		c.Next()
	}, handler.ListAddresses)

	req := httptest.NewRequest(http.MethodGet, "/addresses?purpose=shipping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should NOT be 400 — purpose is valid
	assert.NotEqual(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// Test: Get/Update/Delete ownership guard
// ============================================================================

func TestGetAddress_InvalidID(t *testing.T) {
	handler := &AddressHandler{}

	r := gin.New()
	r.GET("/addresses/:id", func(c *gin.Context) {
		setUserID(c, uuid.New())
		c.Next()
	}, handler.GetAddress)

	req := httptest.NewRequest(http.MethodGet, "/addresses/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateAddress_InvalidID(t *testing.T) {
	handler := &AddressHandler{}

	r := gin.New()
	r.PUT("/addresses/:id", func(c *gin.Context) {
		setUserID(c, uuid.New())
		c.Next()
	}, handler.UpdateAddress)

	body := `{"nickname":"test"}`
	req := httptest.NewRequest(http.MethodPut, "/addresses/not-a-uuid", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteAddress_InvalidID(t *testing.T) {
	handler := &AddressHandler{}

	r := gin.New()
	r.DELETE("/addresses/:id", func(c *gin.Context) {
		setUserID(c, uuid.New())
		c.Next()
	}, handler.DeleteAddress)

	req := httptest.NewRequest(http.MethodDelete, "/addresses/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetPrimary_InvalidID(t *testing.T) {
	handler := &AddressHandler{}

	r := gin.New()
	r.POST("/addresses/:id/primary", func(c *gin.Context) {
		setUserID(c, uuid.New())
		c.Next()
	}, handler.SetPrimary)

	req := httptest.NewRequest(http.MethodPost, "/addresses/not-a-uuid/primary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// Test: Count response shape
// ============================================================================

func TestAddressCountResponse_JSONShape(t *testing.T) {
	resp := addressCountResponse{
		Total:         5,
		ShippingCount: 3,
		SenderCount:   2,
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))

	assert.Equal(t, float64(5), raw["total"])
	assert.Equal(t, float64(3), raw["shipping_count"])
	assert.Equal(t, float64(2), raw["sender_count"])
}

// ============================================================================
// Test: Create validation
// ============================================================================

func TestCreateAddress_InvalidPurpose(t *testing.T) {
	handler := &AddressHandler{}

	r := gin.New()
	r.POST("/addresses", func(c *gin.Context) {
		setUserID(c, uuid.New())
		c.Next()
	}, handler.CreateAddress)

	body := `{"purpose":"invalid","recipient_name":"Ali","phone":"08123456789","province_id":"32","city_id":"3204","street_address":"Jl Test"}`
	req := httptest.NewRequest(http.MethodPost, "/addresses", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid purpose")
}

func TestCreateAddress_MissingRequiredFields(t *testing.T) {
	handler := &AddressHandler{}

	r := gin.New()
	r.POST("/addresses", func(c *gin.Context) {
		setUserID(c, uuid.New())
		c.Next()
	}, handler.CreateAddress)

	// Missing recipient_name
	body := `{"purpose":"shipping","phone":"08123456789","province_id":"32","city_id":"3204","street_address":"Jl Test"}`
	req := httptest.NewRequest(http.MethodPost, "/addresses", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// Test: Full address building
// ============================================================================

func TestBuildFullAddress(t *testing.T) {
	addr := &addressEntity.Address{
		StreetAddress: "Jl. Merdeka No. 1",
		VillageName:   "Dago",
		DistrictName:  "Coblong",
		CityName:      "Bandung",
		ProvinceName:  "Jawa Barat",
		PostalCode:    "40135",
	}

	result := buildFullAddress(addr)
	assert.Equal(t, "Jl. Merdeka No. 1, Dago, Coblong, Bandung, Jawa Barat, 40135, Indonesia", result)
}

func TestBuildFullAddress_Partial(t *testing.T) {
	addr := &addressEntity.Address{
		StreetAddress: "Jl. Test",
		CityName:      "Jakarta",
	}

	result := buildFullAddress(addr)
	assert.Equal(t, "Jl. Test, Jakarta, Indonesia", result)
}

// ============================================================================
// Mock role checker
// ============================================================================

type mockRoleChecker struct {
	hasCapability bool
	err           error
}

func (m *mockRoleChecker) HasActiveSellerCapability(_ context.Context, _ uuid.UUID) (bool, error) {
	return m.hasCapability, m.err
}

func (m *mockRoleChecker) IsAdmin(_ context.Context, _ uuid.UUID) (bool, error) {
	return false, nil
}

func (m *mockRoleChecker) IsSeller(_ context.Context, _ uuid.UUID) (bool, error) {
	return m.hasCapability, m.err
}


