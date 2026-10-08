// Package http exposes the canonical Geography read API. This is the ONE
// access path for geographic reference data; consumers must not ship their
// own.
package http

import (
	"github.com/gin-gonic/gin"
	geography "github.com/labuda/backend/internal/platform/geography"
	"github.com/labuda/backend/internal/platform/response"
	"go.uber.org/zap"
)

// Handler serves the canonical Geography read API.
type Handler struct {
	service *geography.Service
	log     *zap.Logger
}

// NewHandler constructs the canonical geography HTTP handler.
func NewHandler(service *geography.Service, log *zap.Logger) *Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{service: service, log: log}
}

// geographyDTO is the wire shape of one canonical geography row.
type geographyDTO struct {
	Code       string `json:"code"`
	Level      string `json:"level"`
	Name       string `json:"name"`
	ParentCode string `json:"parent_code,omitempty"`
	PostalCode string `json:"postal_code,omitempty"`
}

func toDTO(e geography.Entity) geographyDTO {
	dto := geographyDTO{
		Code:  e.Code,
		Level: string(e.Level),
		Name:  e.Name,
	}
	if e.ParentCode != nil {
		dto.ParentCode = *e.ParentCode
	}
	if e.PostalCode != nil {
		dto.PostalCode = *e.PostalCode
	}
	return dto
}

func toDTOs(entities []geography.Entity) []geographyDTO {
	out := make([]geographyDTO, len(entities))
	for i, e := range entities {
		out[i] = toDTO(e)
	}
	return out
}

// ListProvinces handles GET /geographies/provinces.
func (h *Handler) ListProvinces(c *gin.Context) {
	entities, err := h.service.Provinces(c.Request.Context())
	if err != nil {
		h.log.Error("list provinces failed", zap.Error(err))
		response.InternalError(c, "Failed to load provinces")
		return
	}
	response.Success(c, toDTOs(entities))
}

// ListRegencies handles GET /geographies/provinces/:code/regencies.
func (h *Handler) ListRegencies(c *gin.Context) {
	code := c.Param("code")
	entities, err := h.service.Regencies(c.Request.Context(), code)
	if err != nil {
		h.log.Error("list regencies failed", zap.Error(err))
		response.InternalError(c, "Failed to load regencies")
		return
	}
	response.Success(c, toDTOs(entities))
}

// ListDistricts handles GET /geographies/regencies/:code/districts.
func (h *Handler) ListDistricts(c *gin.Context) {
	code := c.Param("code")
	entities, err := h.service.Districts(c.Request.Context(), code)
	if err != nil {
		h.log.Error("list districts failed", zap.Error(err))
		response.InternalError(c, "Failed to load districts")
		return
	}
	response.Success(c, toDTOs(entities))
}

// ListVillages handles GET /geographies/districts/:code/villages.
func (h *Handler) ListVillages(c *gin.Context) {
	code := c.Param("code")
	entities, err := h.service.Villages(c.Request.Context(), code)
	if err != nil {
		h.log.Error("list villages failed", zap.Error(err))
		response.InternalError(c, "Failed to load villages")
		return
	}
	response.Success(c, toDTOs(entities))
}

// GetByCode handles GET /geographies/villages/:code (single row, postal lookup).
func (h *Handler) GetByCode(c *gin.Context) {
	code := c.Param("code")
	entity, err := h.service.ByCode(c.Request.Context(), code)
	if err != nil {
		if err == geography.ErrGeographyNotFound {
			response.NotFound(c, "Geography not found")
			return
		}
		h.log.Error("get geography failed", zap.Error(err))
		response.InternalError(c, "Failed to load geography")
		return
	}
	response.Success(c, toDTO(*entity))
}
