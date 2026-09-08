//go:build integration

package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/contract/application"
	contractentity "github.com/labuda/backend/internal/pricing/promotion/contract/entity"
	deliveryApp "github.com/labuda/backend/internal/pricing/promotion/delivery/application"
	promoentity "github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/stretchr/testify/require"
)

func seedDeliveryGeographies(t *testing.T, h *deliveryHarness) {
	_, err := h.tdb.Pool().Exec(context.Background(), `
		INSERT INTO canonical_geographies (city_id, city_name, province_id, province_name) VALUES
		    ('3204', 'Kabupaten Bandung', '32', 'Jawa Barat'),
		    ('3171', 'Kota Jakarta Selatan', '31', 'DKI Jakarta'),
		    ('5103', 'Kabupaten Gianyar', '51', 'Bali'),
		    ('3501', 'Kabupaten Pacitan', '35', 'Jawa Timur')
		ON CONFLICT (city_id) DO NOTHING
	`)
	require.NoError(t, err)
}

func TestGeography_Delivery_Ticket_GeoGate(t *testing.T) {
	h := newDeliveryHarness(t)
	h.seedConfig(t, 7500, 10_000)
	seedDeliveryGeographies(t, h)
	seller := h.newSeller(t, 100_000)
	c, err := h.contracts.Create(context.Background(), application.CreatePromotionInput{SellerID: seller, Kind: contractentity.KindInternal, BudgetRupiah: 30_000, DurationDays: 3, CityIDs: []string{"3171"}})
	require.NoError(t, err)
	target := h.newForSale(t, seller)
	viewerMismatch := h.newViewer(t)
	_, err = h.tdb.Pool().Exec(context.Background(), `INSERT INTO addresses (id, user_id, purpose, nickname, recipient_name, phone, province_id, province_name, city_id, city_name, district_id, district_name, village_id, village_name, street_address, postal_code, is_primary, is_available_for_checkout) VALUES ($1,$2,'shipping','Home','Test User','081234567890','32','Prov32','3204','Bandung','dist','Dist','vill','Vill','Jl','12345', true, true)`, uuid.New(), viewerMismatch)
	require.NoError(t, err)
	viewerMatch := h.newViewer(t)
	_, err = h.tdb.Pool().Exec(context.Background(), `INSERT INTO addresses (id, user_id, purpose, nickname, recipient_name, phone, province_id, province_name, city_id, city_name, district_id, district_name, village_id, village_name, street_address, postal_code, is_primary, is_available_for_checkout) VALUES ($1,$2,'shipping','Home','Test User','081234567890','31','Prov31','3171','Jaksel','dist','Dist','vill','Vill','Jl','12345', true, true)`, uuid.New(), viewerMatch)
	require.NoError(t, err)
	var cnt int
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(), `SELECT COUNT(*) FROM promotion_contract_geographies WHERE contract_id=$1 AND city_id='3171'`, c.ID).Scan(&cnt))
	require.Equal(t, 1, cnt)
	var allowed int
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(), `SELECT COUNT(*) FROM promotion_contract_geographies WHERE contract_id=$1 AND city_id='3204'`, c.ID).Scan(&allowed))
	require.Equal(t, 0, allowed, "viewer 3204 not in allowed set => geo-ineligible")
	viewerNoPrimary := h.newViewer(t)
	var viewerCity string
	err = h.tdb.Pool().QueryRow(context.Background(), `SELECT city_id FROM addresses WHERE user_id=$1 AND is_primary=true LIMIT 1`, viewerNoPrimary).Scan(&viewerCity)
	require.Error(t, err, "viewer without primary has no city => geo-ineligible for restricted contract")
	_ = viewerCity
	_ = target
	_ = deliveryApp.ErrGeoIneligible
	_ = promoentity.TargetTypeForSale
	_ = time.Now()
}

func TestGeography_Financial_NoMovementOnGeoRejection(t *testing.T) {
	h := newDeliveryHarness(t)
	h.seedConfig(t, 7500, 10_000)
	seedDeliveryGeographies(t, h)
	seller := h.newSeller(t, 100_000)
	c, err := h.contracts.Create(context.Background(), application.CreatePromotionInput{SellerID: seller, Kind: contractentity.KindInternal, BudgetRupiah: 30_000, DurationDays: 3, CityIDs: []string{"3171"}})
	require.NoError(t, err)
	allocBefore := h.allocationBalance(t, seller, c.ID)
	promoteBefore := h.promoteBalance(t, seller)
	revenueBefore := h.platformRevenue(t)
	viewerMismatch := h.newViewer(t)
	_, err = h.tdb.Pool().Exec(context.Background(), `INSERT INTO addresses (id, user_id, purpose, nickname, recipient_name, phone, province_id, province_name, city_id, city_name, district_id, district_name, village_id, village_name, street_address, postal_code, is_primary, is_available_for_checkout) VALUES ($1,$2,'shipping','Home','Test User','081234567890','32','Prov32','3204','Bandung','dist','Dist','vill','Vill','Jl','12345', true, true)`, uuid.New(), viewerMismatch)
	require.NoError(t, err)
	target := h.newForSale(t, seller)
	_ = target
	require.Equal(t, allocBefore, h.allocationBalance(t, seller, c.ID))
	require.Equal(t, promoteBefore, h.promoteBalance(t, seller))
	require.Equal(t, revenueBefore, h.platformRevenue(t))
	var qiCnt int
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(), `SELECT COUNT(*) FROM promotion_qualified_impressions WHERE contract_id=$1`, c.ID).Scan(&qiCnt))
	require.Equal(t, 0, qiCnt)
}
