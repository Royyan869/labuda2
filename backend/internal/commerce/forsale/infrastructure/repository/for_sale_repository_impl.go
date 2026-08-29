package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	entity "github.com/labuda/backend/internal/commerce/forsale/entity"
	for_saleRepo "github.com/labuda/backend/internal/commerce/forsale/repository"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	productInfraRepo "github.com/labuda/backend/internal/commerce/product/infrastructure/repository"
	productRepo "github.com/labuda/backend/internal/commerce/product/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// ForSaleRepositoryImpl persists canonical fixed-price sale rows.
type ForSaleRepositoryImpl struct {
	productRepo productRepo.ProductRepository
}

// NewForSaleRepository creates a new fixed-price sale repository.
func NewForSaleRepository() *ForSaleRepositoryImpl {
	return &ForSaleRepositoryImpl{
		productRepo: productInfraRepo.NewProductRepository(),
	}
}

func (r *ForSaleRepositoryImpl) Create(ctx context.Context, tx db.Tx, for_sale *entity.ForSale) error {
	if for_sale == nil {
		return fmt.Errorf("for_sale is nil")
	}

	// Reuse path: an explicit ProductID attaches the sale to an existing
	// Product (stable Product identity). The product must exist, be owned
	// by this seller, and be unattached to any selling surface.
	if for_sale.ProductID != uuid.Nil {
		product, err := r.productRepo.GetByID(ctx, tx, for_sale.ProductID)
		if err != nil {
			return fmt.Errorf("reuse product failed: %w", err)
		}
		if product.SellerID != for_sale.SellerID {
			return fmt.Errorf("cannot attach fixed-price sale to product owned by another seller")
		}
		// INVARIANT: Product must not already belong to any selling surface.
		// ClaimSellingSurface uses SELECT ... FOR UPDATE to prevent concurrent
		// attachment to both ForSale and Auction.
		if err := r.productRepo.ClaimSellingSurface(ctx, tx, for_sale.ProductID, productEntity.SellingSurfaceForSale); err != nil {
			return fmt.Errorf("cannot attach for_sale to product: %w", err)
		}
		for_sale.Product = product
		for_sale.Visibility = derivedVisibility(for_sale.Status, for_sale.PublishedAt)
		if err := r.insertForSaleRow(ctx, tx, for_sale); err != nil {
			return err
		}
		return nil
	}

	// Mint path: a Product is created atomically with the sale when the
	// caller did not supply an existing ProductID.
	product := buildProductFromSale(for_sale)
	if product.ID == uuid.Nil {
		product.ID = uuid.New()
	}
	if for_sale.ID == uuid.Nil {
		for_sale.ID = uuid.New()
	}

	// Set selling_surface atomically with Product creation.
	product.SellingSurface = productEntity.SellingSurfaceForSale

	if err := r.productRepo.Create(ctx, tx, product); err != nil {
		return fmt.Errorf("create product failed: %w", err)
	}

	for_sale.ProductID = product.ID
	for_sale.Product = product
	for_sale.Visibility = derivedVisibility(for_sale.Status, for_sale.PublishedAt)

	if err := r.insertForSaleRow(ctx, tx, for_sale); err != nil {
		return err
	}

	return nil
}

// insertForSaleRow persists the for_sales row. Shared by the
// mint path (new product created inline) and the reuse path (explicit
// ProductID attached to an existing product).
func (r *ForSaleRepositoryImpl) insertForSaleRow(ctx context.Context, tx db.Tx, for_sale *entity.ForSale) error {
	if for_sale.ID == uuid.Nil {
		for_sale.ID = uuid.New()
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO for_sales (
			id, product_id, seller_id, price_per_unit, negotiation_enabled,
			status, published_at, sold_at, withdrawn_at,
			quantity_available,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`,
		for_sale.ID,
		for_sale.ProductID,
		for_sale.SellerID,
		for_sale.PricePerUnit.Int64(),
		for_sale.NegotiationEnabled,
		string(for_sale.Status),
		for_sale.PublishedAt,
		for_sale.SoldAt,
		for_sale.WithdrawnAt,
		for_sale.QuantityAvailable,
		for_sale.CreatedAt,
		for_sale.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create fixed price sale failed: %w", err)
	}
	return nil
}

func (r *ForSaleRepositoryImpl) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.ForSale, error) {
	row := tx.QueryRow(ctx, joinedSaleByIDQuery(), id)
	return scanJoinedSale(row)
}

func (r *ForSaleRepositoryImpl) GetByProductID(ctx context.Context, tx db.Tx, productID uuid.UUID) (*entity.ForSale, error) {
	row := tx.QueryRow(ctx, joinedSaleByProductIDQuery(), productID)
	return scanJoinedSale(row)
}

func (r *ForSaleRepositoryImpl) GetForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.ForSale, error) {
	row := tx.QueryRow(ctx, joinedSaleByIDQuery()+" FOR UPDATE OF fps, p", id)
	return scanJoinedSale(row)
}

func (r *ForSaleRepositoryImpl) Update(ctx context.Context, tx db.Tx, for_sale *entity.ForSale) error {
	if for_sale == nil {
		return fmt.Errorf("for_sale is nil")
	}

	product := buildProductFromSale(for_sale)
	product.ID = for_sale.ProductID
	if product.ID == uuid.Nil && for_sale.Product != nil {
		product.ID = for_sale.Product.ID
	}
	if product.ID == uuid.Nil {
		return fmt.Errorf("product id is required")
	}

	if err := r.productRepo.Update(ctx, tx, product); err != nil {
		return fmt.Errorf("update product failed: %w", err)
	}

	for_sale.Product = product
	for_sale.ProductID = product.ID
	for_sale.Visibility = derivedVisibility(for_sale.Status, for_sale.PublishedAt)

	_, err := tx.Exec(ctx, `
		UPDATE for_sales
		SET seller_id = $2,
		    price_per_unit = $3,
		    negotiation_enabled = $4,
		    status = $5,
		    published_at = $6,
		    sold_at = $7,
		    withdrawn_at = $8,
		    quantity_available = $9,
		    updated_at = $10
		WHERE id = $1
	`,
		for_sale.ID,
		for_sale.SellerID,
		for_sale.PricePerUnit.Int64(),
		for_sale.NegotiationEnabled,
		string(for_sale.Status),
		for_sale.PublishedAt,
		for_sale.SoldAt,
		for_sale.WithdrawnAt,
		for_sale.QuantityAvailable,
		for_sale.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update fixed price sale failed: %w", err)
	}

	return nil
}

func (r *ForSaleRepositoryImpl) UpdateStock(ctx context.Context, tx db.Tx, for_sale *entity.ForSale) error {
	if for_sale == nil {
		return fmt.Errorf("for_sale is nil")
	}
	if for_sale.ProductID == uuid.Nil && for_sale.Product != nil {
		for_sale.ProductID = for_sale.Product.ID
	}
	if for_sale.ProductID == uuid.Nil {
		return fmt.Errorf("product id is required")
	}

	_, err := tx.Exec(ctx, `
		UPDATE for_sales
		SET status = $2,
		    sold_at = $3,
		    quantity_available = $4,
		    updated_at = $5
		WHERE id = $1
	`,
		for_sale.ID,
		string(for_sale.Status),
		for_sale.SoldAt,
		for_sale.QuantityAvailable,
		for_sale.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update fixed price sale stock failed: %w", err)
	}

	return nil
}

func (r *ForSaleRepositoryImpl) UpdateStatus(ctx context.Context, tx db.Tx, for_sale *entity.ForSale) error {
	if for_sale == nil {
		return fmt.Errorf("for_sale is nil")
	}
	if for_sale.ProductID == uuid.Nil && for_sale.Product != nil {
		for_sale.ProductID = for_sale.Product.ID
	}
	if for_sale.ProductID == uuid.Nil {
		return fmt.Errorf("product id is required")
	}

	_, err := tx.Exec(ctx, `
		UPDATE for_sales
		SET status = $2,
		    published_at = $3,
		    sold_at = $4,
		    withdrawn_at = $5,
		    updated_at = $6
		WHERE id = $1
	`,
		for_sale.ID,
		string(for_sale.Status),
		for_sale.PublishedAt,
		for_sale.SoldAt,
		for_sale.WithdrawnAt,
		for_sale.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update fixed price sale status failed: %w", err)
	}

	return nil
}

func (r *ForSaleRepositoryImpl) GetBySellerID(ctx context.Context, tx db.Tx, sellerID uuid.UUID, includeWithdrawn bool) ([]*entity.ForSale, error) {
	query := joinedSalesQueryBase() + ` WHERE fps.seller_id = $1`
	args := []any{sellerID}
	if !includeWithdrawn {
		query += ` AND fps.status != 'withdrawn'`
	}
	query += ` ORDER BY fps.created_at DESC`
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get fixed price sales by seller failed: %w", err)
	}
	defer rows.Close()
	return scanJoinedSaleRows(rows)
}

func (r *ForSaleRepositoryImpl) GetBySellerIDPaginated(ctx context.Context, tx db.Tx, sellerID uuid.UUID, limit, offset int, includeWithdrawn bool) ([]*entity.ForSale, error) {
	query := joinedSalesQueryBase() + ` WHERE fps.seller_id = $1`
	args := []any{sellerID}
	if !includeWithdrawn {
		query += ` AND fps.status != 'withdrawn'`
	}
	query += ` ORDER BY fps.created_at DESC LIMIT $2 OFFSET $3`
	args = append(args, limit, offset)
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get fixed price sales by seller paginated failed: %w", err)
	}
	defer rows.Close()
	return scanJoinedSaleRows(rows)
}

func (r *ForSaleRepositoryImpl) GetPublic(ctx context.Context, tx db.Tx, limit, offset int) ([]*entity.ForSale, error) {
	rows, err := tx.Query(ctx, `
		SELECT `+joinedSaleSelectColumns()+`
		FROM for_sales fps
		JOIN products p ON p.id = fps.product_id
		JOIN users u ON u.id = fps.seller_id
		WHERE fps.status = 'active'
		  AND fps.quantity_available > 0
		  AND u.account_status = 'active'
		  AND u.deleted_at IS NULL
		ORDER BY fps.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get public fixed price sales failed: %w", err)
	}
	defer rows.Close()
	return scanJoinedSaleRows(rows)
}

// GetPublicBySellerID returns publicly discoverable fixed-price sales of one
// seller: active + in-stock + seller account active/not-deleted. Used by the
// public browsable seller page. This is NOT a seller-inventory query — draft,
// sold and withdrawn surfaces are excluded by construction.
func (r *ForSaleRepositoryImpl) GetPublicBySellerID(ctx context.Context, tx db.Tx, sellerID uuid.UUID, limit, offset int) ([]*entity.ForSale, error) {
	rows, err := tx.Query(ctx, `
		SELECT `+joinedSaleSelectColumns()+`
		FROM for_sales fps
		JOIN products p ON p.id = fps.product_id
		JOIN users u ON u.id = fps.seller_id
		WHERE fps.seller_id = $1
		  AND fps.status = 'active'
		  AND fps.quantity_available > 0
		  AND u.account_status = 'active'
		  AND u.deleted_at IS NULL
		ORDER BY fps.created_at DESC
		LIMIT $2 OFFSET $3
	`, sellerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get public fixed price sales by seller failed: %w", err)
	}
	defer rows.Close()
	return scanJoinedSaleRows(rows)
}

func (r *ForSaleRepositoryImpl) Search(ctx context.Context, tx db.Tx, filters for_saleRepo.SearchFilters) ([]*entity.ForSale, *time.Time, error) {
	limit := filters.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	query := joinedSalesQueryBase() + ` WHERE fps.status = 'active' AND fps.quantity_available > 0 AND u.account_status = 'active' AND u.deleted_at IS NULL`
	args := make([]any, 0, 4)
	argIdx := 1

	if filters.Query != "" {
		query += fmt.Sprintf(` AND (
			LOWER(p.title) LIKE LOWER($%d) OR
			LOWER(p.description) LIKE LOWER($%d) OR
			LOWER(p.variety) LIKE LOWER($%d)
		)`, argIdx, argIdx, argIdx)
		q := "%" + filters.Query + "%"
		args = append(args, q)
		argIdx++
	}
	if filters.PriceMin != nil {
		query += fmt.Sprintf(` AND fps.price_per_unit >= $%d`, argIdx)
		args = append(args, *filters.PriceMin)
		argIdx++
	}
	if filters.PriceMax != nil {
		query += fmt.Sprintf(` AND fps.price_per_unit <= $%d`, argIdx)
		args = append(args, *filters.PriceMax)
		argIdx++
	}
	if filters.Variety != nil && *filters.Variety != "" {
		query += fmt.Sprintf(` AND p.variety = $%d`, argIdx)
		args = append(args, *filters.Variety)
		argIdx++
	}
	if filters.SellerID != nil {
		query += fmt.Sprintf(` AND fps.seller_id = $%d`, argIdx)
		args = append(args, *filters.SellerID)
		argIdx++
	}
	if filters.Cursor != nil {
		query += fmt.Sprintf(` AND fps.created_at < $%d`, argIdx)
		args = append(args, *filters.Cursor)
		argIdx++
	}

	sortBy := strings.ToLower(filters.SortBy)
	sortDir := strings.ToLower(filters.SortDir)
	if sortDir != "asc" {
		sortDir = "desc"
	}
	switch sortBy {
	case "price":
		query += fmt.Sprintf(` ORDER BY fps.price_per_unit %s, fps.created_at %s`, sortDir, sortDir)
	default:
		query += fmt.Sprintf(` ORDER BY fps.created_at %s`, sortDir)
	}
	query += fmt.Sprintf(` LIMIT %d`, limit+1)

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("search fixed price sales failed: %w", err)
	}
	defer rows.Close()

	sales, err := scanJoinedSaleRows(rows)
	if err != nil {
		return nil, nil, err
	}
	var nextCursor *time.Time
	if len(sales) > limit {
		nextCursor = &sales[limit-1].CreatedAt
		sales = sales[:limit]
	}
	return sales, nextCursor, nil
}

var _ for_saleRepo.ForSaleRepository = (*ForSaleRepositoryImpl)(nil)

func joinedSaleByIDQuery() string {
	return `SELECT ` + joinedSaleSelectColumns() + `
		FROM for_sales fps
		JOIN products p ON p.id = fps.product_id
		JOIN users u ON u.id = fps.seller_id
		WHERE fps.id = $1`
}

func joinedSaleByProductIDQuery() string {
	return `SELECT ` + joinedSaleSelectColumns() + `
		FROM for_sales fps
		JOIN products p ON p.id = fps.product_id
		JOIN users u ON u.id = fps.seller_id
		WHERE fps.product_id = $1`
}

func joinedSalesQueryBase() string {
	return `SELECT ` + joinedSaleSelectColumns() + `
		FROM for_sales fps
		JOIN products p ON p.id = fps.product_id
		JOIN users u ON u.id = fps.seller_id`
}

func joinedSaleSelectColumns() string {
	return `fps.id,
		fps.product_id,
		fps.seller_id,
		fps.price_per_unit,
		fps.negotiation_enabled,
		fps.status,
		fps.published_at,
		fps.sold_at,
		fps.withdrawn_at,
		fps.quantity_available,
		fps.created_at,
		fps.updated_at,
		p.id,
		p.seller_id,
		p.title,
		p.description,
		p.media_urls,
		p.variety,
		p.size_cm,
		p.age_months,
		p.gender,
		p.breeder,
		p.bloodline,
		p.certificates,
		p.farm_address_id,
		p.preparation_time,
		p.preparation_note,
		p.selling_surface,
		p.created_at,
		p.updated_at,
		u.id,
		u.account_status,
		u.deleted_at`
}

func scanJoinedSaleRows(rows pgx.Rows) ([]*entity.ForSale, error) {
	sales := make([]*entity.ForSale, 0)
	for rows.Next() {
		sale, err := scanJoinedSaleFromRows(rows)
		if err != nil {
			return nil, err
		}
		sales = append(sales, sale)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate fixed price sales failed: %w", err)
	}
	return sales, nil
}

func scanJoinedSale(row pgx.Row) (*entity.ForSale, error) {
	sale, err := scanJoinedSaleFromRow(row)
	if err != nil {
		return nil, err
	}
	return sale, nil
}

func scanJoinedSaleFromRows(rows pgx.Rows) (*entity.ForSale, error) {
	return scanJoinedSaleFromRow(rows)
}

func scanJoinedSaleFromRow(scanner interface {
	Scan(dest ...any) error
}) (*entity.ForSale, error) {
	var sale entity.ForSale
	var saleProductID uuid.UUID
	var saleStatus string
	var mediaURLsRaw json.RawMessage
	var sizeCM, ageMonths *int
	var gender, breeder, bloodline, preparationNote *string
	var certificates []string
	var publishedAt, soldAt, withdrawnAt *time.Time
	var quantityAvailable int
	var productCreatedAt, productUpdatedAt time.Time
	var saleCreatedAt, saleUpdatedAt time.Time
	var productFarmAddressID *uuid.UUID
	var productID uuid.UUID
	var productSellerID uuid.UUID
	var sellingSurfaceRaw *string
	var deletedAt *time.Time
	var userAccountStatus string
	var userID uuid.UUID

	if err := scanner.Scan(
		&sale.ID,
		&saleProductID,
		&sale.SellerID,
		&sale.PricePerUnit,
		&sale.NegotiationEnabled,
		&saleStatus,
		&publishedAt,
		&soldAt,
		&withdrawnAt,
		&quantityAvailable,
		&saleCreatedAt,
		&saleUpdatedAt,
		&productID,
		&productSellerID,
		&sale.Title,
		&sale.Description,
		&mediaURLsRaw,
		&sale.Variety,
		&sizeCM,
		&ageMonths,
		&gender,
		&breeder,
		&bloodline,
		&certificates,
		&productFarmAddressID,
		&sale.PreparationTime,
		&preparationNote,
		&sellingSurfaceRaw,
		&productCreatedAt,
		&productUpdatedAt,
		&userID,
		&userAccountStatus,
		&deletedAt,
	); err != nil {
		return nil, fmt.Errorf("scan fixed price sale failed: %w", err)
	}

	var mediaURLs []string
	if len(mediaURLsRaw) > 0 && string(mediaURLsRaw) != "null" {
		if err := json.Unmarshal(mediaURLsRaw, &mediaURLs); err != nil {
			return nil, fmt.Errorf("unmarshal product media urls failed: %w", err)
		}
	}

	var sellingSurface productEntity.SellingSurface
	if sellingSurfaceRaw != nil {
		sellingSurface = productEntity.SellingSurface(*sellingSurfaceRaw)
	}

	product := &productEntity.Product{
		ID:              productID,
		SellerID:        productSellerID,
		Title:           sale.Title,
		Description:     sale.Description,
		MediaURLs:       mediaURLs,
		Variety:         sale.Variety,
		SizeCm:          sizeCM,
		AgeMonths:       ageMonths,
		Gender:          gender,
		Breeder:         breeder,
		Bloodline:       bloodline,
		Certificates:    certificates,
		FarmAddressID:   productFarmAddressID,
		PreparationTime: string(sale.PreparationTime),
		PreparationNote: preparationNote,
		SellingSurface:  sellingSurface,
		CreatedAt:       productCreatedAt,
		UpdatedAt:       productUpdatedAt,
	}

	sale.ProductID = saleProductID
	sale.Product = product
	sale.ForSaleType = entity.ForSaleTypeFixedPrice
	sale.QuantityAvailable = quantityAvailable
	sale.Visibility = derivedVisibility(entity.ForSaleStatus(saleStatus), publishedAt)
	sale.Status = entity.ForSaleStatus(saleStatus)
	sale.PublishedAt = publishedAt
	sale.SoldAt = soldAt
	sale.WithdrawnAt = withdrawnAt
	sale.CreatedAt = saleCreatedAt
	sale.UpdatedAt = saleUpdatedAt
	sale.Origin = entity.ForSaleOriginDirectCreate
	sale.FarmAddressID = productFarmAddressID
	sale.PreparationNote = preparationNote
	sale.PricePerUnit = money.New(sale.PricePerUnit.Int64())
	_ = userID
	_ = userAccountStatus
	_ = deletedAt

	return &sale, nil
}

func buildProductFromSale(for_sale *entity.ForSale) *productEntity.Product {
	product := for_sale.Product
	if product == nil {
		product = &productEntity.Product{}
	}
	product.ID = for_sale.ProductID
	product.SellerID = for_sale.SellerID
	product.Title = for_sale.Title
	product.Description = for_sale.Description
	product.MediaURLs = rawMediaURLs(for_sale.MediaURLs)
	product.Variety = for_sale.Variety
	product.SizeCm = for_sale.SizeCM
	product.AgeMonths = for_sale.AgeMonths
	product.Gender = for_sale.Gender
	product.Breeder = for_sale.Breeder
	product.Bloodline = for_sale.Bloodline
	product.Certificates = for_sale.Certificates
	product.FarmAddressID = for_sale.FarmAddressID
	product.PreparationTime = string(for_sale.PreparationTime)
	product.PreparationNote = for_sale.PreparationNote
	product.UpdatedAt = for_sale.UpdatedAt
	if product.CreatedAt.IsZero() {
		product.CreatedAt = for_sale.CreatedAt
	}
	// INVARIANT: SellingSurface is NEVER set by buildProductFromSale.
	// It is set exclusively by Create() (mint path) and ClaimSellingSurface()
	// (reuse path). If the source Product already has a SellingSurface, it
	// is preserved via the copy from for_sale.Product above. If the source
	// Product is nil (mint path), SellingSurface defaults to zero value and
	// is explicitly set by the caller (Create method).
	return product
}

func rawMediaURLs(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return []string{}
	}
	var urls []string
	if err := json.Unmarshal(raw, &urls); err != nil {
		return []string{}
	}
	return urls
}

func derivedVisibility(status entity.ForSaleStatus, publishedAt *time.Time) entity.ForSaleVisibility {
	if status == entity.ForSaleStatusActive && publishedAt != nil {
		return entity.ForSaleVisibilityPublic
	}
	return entity.ForSaleVisibilityPrivate
}
