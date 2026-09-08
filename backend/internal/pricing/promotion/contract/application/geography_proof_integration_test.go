//go:build integration

package application_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/contract/application"
	"github.com/labuda/backend/internal/pricing/promotion/contract/entity"
	"github.com/stretchr/testify/require"
)

func createAddressForGeo(t *testing.T, h *contractHarness, userID uuid.UUID, cityID, provinceID, cityName string) {
	_, err := h.tdb.Pool().Exec(context.Background(), `
		INSERT INTO addresses (id, user_id, purpose, nickname, recipient_name, phone, province_id, province_name, city_id, city_name, district_id, district_name, village_id, village_name, street_address, postal_code, is_primary, is_available_for_checkout)
		VALUES ($1,$2,'shipping','Home','Test User','081234567890',$3,$4,$5,$6,'dist1','District','vill1','Village','Jl Test', '12345', true, true)
		ON CONFLICT (id) DO NOTHING
	`, uuid.New(), userID, provinceID, "Prov"+provinceID, cityID, cityName)
	require.NoError(t, err)
}

func seedCanonicalGeographies(t *testing.T, h *contractHarness) {
	_, err := h.tdb.Pool().Exec(context.Background(), `
		INSERT INTO canonical_geographies (city_id, city_name, province_id, province_name) VALUES
		    ('3204', 'Kabupaten Bandung', '32', 'Jawa Barat'),
		    ('3171', 'Kota Jakarta Selatan', '31', 'DKI Jakarta'),
		    ('3172', 'Kota Jakarta Timur', '31', 'DKI Jakarta'),
		    ('3173', 'Kota Jakarta Pusat', '31', 'DKI Jakarta'),
		    ('5103', 'Kabupaten Gianyar', '51', 'Bali'),
		    ('3501', 'Kabupaten Pacitan', '35', 'Jawa Timur'),
		    ('3502', 'Kabupaten Ponorogo', '35', 'Jawa Timur')
		ON CONFLICT (city_id) DO NOTHING
	`)
	require.NoError(t, err)
}

func TestGeography_Nationwide_And_SingleCity(t *testing.T) {
	h := newContractHarness(t)
	h.seedConfig(t, 7500, 10_000)
	seedCanonicalGeographies(t, h)
	seller := h.newSeller(t, 100_000)

	// Nationwide: empty city_ids => 0 rows
	cNationwide, err := h.svc.Create(context.Background(), application.CreatePromotionInput{SellerID: seller, Kind: entity.KindInternal, BudgetRupiah: 30_000, DurationDays: 3, CityIDs: []string{}})
	require.NoError(t, err)
	var cnt int
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(), `SELECT COUNT(*) FROM promotion_contract_geographies WHERE contract_id=$1`, cNationwide.ID).Scan(&cnt))
	require.Equal(t, 0, cnt)

	// Need new seller for second contract because slot occupied (1 internal per seller)
	seller2 := h.newSeller(t, 100_000)
	cSingle, err := h.svc.Create(context.Background(), application.CreatePromotionInput{SellerID: seller2, Kind: entity.KindInternal, BudgetRupiah: 30_000, DurationDays: 3, CityIDs: []string{"3171"}})
	require.NoError(t, err)
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(), `SELECT COUNT(*) FROM promotion_contract_geographies WHERE contract_id=$1`, cSingle.ID).Scan(&cnt))
	require.Equal(t, 1, cnt)
	var cityID, provinceID string
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(), `SELECT city_id, province_id FROM promotion_contract_geographies WHERE contract_id=$1`, cSingle.ID).Scan(&cityID, &provinceID))
	require.Equal(t, "3171", cityID)
	require.NotEmpty(t, provinceID)
}

func TestGeography_DuplicateDedup(t *testing.T) {
	h := newContractHarness(t)
	h.seedConfig(t, 7500, 10_000)
	seedCanonicalGeographies(t, h)
	seller := h.newSeller(t, 100_000)
	c, err := h.svc.Create(context.Background(), application.CreatePromotionInput{SellerID: seller, Kind: entity.KindInternal, BudgetRupiah: 30_000, DurationDays: 3, CityIDs: []string{"3171", "3171", "3171", "3204", "3204"}})
	require.NoError(t, err)
	var cnt int
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(), `SELECT COUNT(*) FROM promotion_contract_geographies WHERE contract_id=$1`, c.ID).Scan(&cnt))
	require.Equal(t, 2, cnt, "duplicate city_ids must be deduped deterministically")
}

func TestGeography_MultiCity(t *testing.T) {
	h := newContractHarness(t)
	h.seedConfig(t, 7500, 10_000)
	seedCanonicalGeographies(t, h)
	seller := h.newSeller(t, 100_000)
	c, err := h.svc.Create(context.Background(), application.CreatePromotionInput{SellerID: seller, Kind: entity.KindInternal, BudgetRupiah: 30_000, DurationDays: 3, CityIDs: []string{"3204", "3171", "5103"}})
	require.NoError(t, err)
	var cnt int
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(), `SELECT COUNT(*) FROM promotion_contract_geographies WHERE contract_id=$1`, c.ID).Scan(&cnt))
	require.Equal(t, 3, cnt)
	// verify all cities present
	rows, err := h.tdb.Pool().Query(context.Background(), `SELECT city_id FROM promotion_contract_geographies WHERE contract_id=$1 ORDER BY city_id`, c.ID)
	require.NoError(t, err)
	defer rows.Close()
	var got []string
	for rows.Next() {
		var cid string
		require.NoError(t, rows.Scan(&cid))
		got = append(got, cid)
	}
	require.Equal(t, []string{"3171", "3204", "5103"}, got)
}

func TestGeography_Atomicity_NoPartialOnFailure(t *testing.T) {
	h := newContractHarness(t)
	h.seedConfig(t, 7500, 10_000)
	seedCanonicalGeographies(t, h)
	seller := h.newSeller(t, 5_000) // insufficient balance
	_, err := h.svc.Create(context.Background(), application.CreatePromotionInput{SellerID: seller, Kind: entity.KindInternal, BudgetRupiah: 30_000, DurationDays: 3, CityIDs: []string{"3171", "3204"}})
	require.Error(t, err)
	// no contract
	require.Equal(t, 0, h.countContracts(t, seller, entity.KindInternal))
	// no geography rows (would be orphan if not atomic)
	var geoCnt int
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(), `SELECT COUNT(*) FROM promotion_contract_geographies`).Scan(&geoCnt))
	require.Equal(t, 0, geoCnt)
}

func TestGeography_InvalidCity_Rejected(t *testing.T) {
	h := newContractHarness(t)
	h.seedConfig(t, 7500, 10_000)
	seedCanonicalGeographies(t, h)
	seller := h.newSeller(t, 100_000)
	for _, invalid := range []string{"9999", "abc", "foobar", "123456789"} {
		_, err := h.svc.Create(context.Background(), application.CreatePromotionInput{SellerID: seller, Kind: entity.KindInternal, BudgetRupiah: 30_000, DurationDays: 3, CityIDs: []string{invalid}})
		require.Error(t, err, "invalid city %s should be rejected", invalid)
		require.Contains(t, err.Error(), "not in canonical geography vocabulary")
		// ensure no partial contract
		require.Equal(t, 0, h.countContracts(t, seller, entity.KindInternal))
		// need new seller for next iteration because slot may still be free but we check 0
		seller = h.newSeller(t, 100_000)
	}
}

func TestGeography_ValidCityAbsentFromAddresses_StillSucceeds(t *testing.T) {
	h := newContractHarness(t)
	h.seedConfig(t, 7500, 10_000)
	seedCanonicalGeographies(t, h)
	seller := h.newSeller(t, 100_000)
	// 5103 is in canonical_geographies but no user address currently uses it
	var addrCnt int
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(), `SELECT COUNT(*) FROM addresses WHERE city_id='5103'`).Scan(&addrCnt))
	require.Equal(t, 0, addrCnt, "no address uses 5103 yet")
	c, err := h.svc.Create(context.Background(), application.CreatePromotionInput{SellerID: seller, Kind: entity.KindInternal, BudgetRupiah: 30_000, DurationDays: 3, CityIDs: []string{"5103"}})
	require.NoError(t, err)
	var cnt int
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(), `SELECT COUNT(*) FROM promotion_contract_geographies WHERE contract_id=$1 AND city_id='5103'`, c.ID).Scan(&cnt))
	require.Equal(t, 1, cnt)
	var cityName, provinceID string
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(), `SELECT city_name, province_id FROM promotion_contract_geographies WHERE contract_id=$1`, c.ID).Scan(&cityName, &provinceID))
	require.Equal(t, "Kabupaten Gianyar", cityName)
	require.Equal(t, "51", provinceID)
}
