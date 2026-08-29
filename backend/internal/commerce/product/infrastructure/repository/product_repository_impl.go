package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/commerce/product/entity"
	"github.com/labuda/backend/pkg/db"
)

// ProductRepositoryImpl persists canonical products.
type ProductRepositoryImpl struct{}

// NewProductRepository creates a new ProductRepositoryImpl.
func NewProductRepository() *ProductRepositoryImpl {
	return &ProductRepositoryImpl{}
}

func (r *ProductRepositoryImpl) Create(ctx context.Context, tx db.Tx, product *entity.Product) error {
	if product == nil {
		return fmt.Errorf("product is nil")
	}
	if product.ID == uuid.Nil {
		product.ID = uuid.New()
	}
	if product.CreatedAt.IsZero() {
		product.CreatedAt = time.Now()
	}
	if product.UpdatedAt.IsZero() {
		product.UpdatedAt = product.CreatedAt
	}

	mediaURLs := product.MediaURLs
	if mediaURLs == nil {
		mediaURLs = []string{}
	}
	certificates := product.Certificates
	if certificates == nil {
		certificates = []string{}
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO products (
			id, seller_id, title, description, media_urls,
			variety, size_cm, age_months, gender, breeder, bloodline, certificates,
			farm_address_id, preparation_time, preparation_note,
			selling_surface,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`,
		product.ID,
		product.SellerID,
		product.Title,
		product.Description,
		mustMarshalJSON(mediaURLs),
		product.Variety,
		product.SizeCm,
		product.AgeMonths,
		product.Gender,
		product.Breeder,
		product.Bloodline,
		certificates,
		product.FarmAddressID,
		product.PreparationTime,
		product.PreparationNote,
		nullString(string(product.SellingSurface)),
		product.CreatedAt,
		product.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create product failed: %w", err)
	}
	product.MediaURLs = mediaURLs
	product.Certificates = certificates
	return nil
}

func (r *ProductRepositoryImpl) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Product, error) {
	row := tx.QueryRow(ctx, `
		SELECT id, seller_id, title, description, media_urls,
		       variety, size_cm, age_months, gender, breeder, bloodline, certificates,
		       farm_address_id, preparation_time, preparation_note,
		       selling_surface,
		       created_at, updated_at
		FROM products
		WHERE id = $1
	`, id)
	return scanProductRow(row)
}

func (r *ProductRepositoryImpl) Update(ctx context.Context, tx db.Tx, product *entity.Product) error {
	if product == nil {
		return fmt.Errorf("product is nil")
	}
	if product.ID == uuid.Nil {
		return fmt.Errorf("product id is required")
	}
	if product.UpdatedAt.IsZero() {
		product.UpdatedAt = time.Now()
	}

	mediaURLs := product.MediaURLs
	if mediaURLs == nil {
		mediaURLs = []string{}
	}
	certificates := product.Certificates
	if certificates == nil {
		certificates = []string{}
	}

	// INVARIANT: selling_surface is NEVER written by Update().
	// It is set exclusively by Create() (mint path) and ClaimSellingSurface()
	// (reuse path). Once claimed (non-NULL), it is immutable forever.
	_, err := tx.Exec(ctx, `
		UPDATE products
		SET seller_id = $2,
		    title = $3,
		    description = $4,
		    media_urls = $5,
		    variety = $6,
		    size_cm = $7,
		    age_months = $8,
		    gender = $9,
		    breeder = $10,
		    bloodline = $11,
		    certificates = $12,
		    farm_address_id = $13,
		    preparation_time = $14,
		    preparation_note = $15,
		    updated_at = $16
		WHERE id = $1
	`,
		product.ID,
		product.SellerID,
		product.Title,
		product.Description,
		mustMarshalJSON(mediaURLs),
		product.Variety,
		product.SizeCm,
		product.AgeMonths,
		product.Gender,
		product.Breeder,
		product.Bloodline,
		certificates,
		product.FarmAddressID,
		product.PreparationTime,
		product.PreparationNote,
		product.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update product failed: %w", err)
	}
	product.MediaURLs = mediaURLs
	product.Certificates = certificates
	return nil
}

func scanProductRow(row pgx.Row) (*entity.Product, error) {
	var product entity.Product
	var mediaURLsRaw json.RawMessage
	var certificates []string
	var sizeCM, ageMonths *int
	var gender, breeder, bloodline, preparationNote *string
	var farmAddressID *uuid.UUID
	var sellingSurfaceRaw *string
	var createdAt, updatedAt time.Time
	if err := row.Scan(
		&product.ID,
		&product.SellerID,
		&product.Title,
		&product.Description,
		&mediaURLsRaw,
		&product.Variety,
		&sizeCM,
		&ageMonths,
		&gender,
		&breeder,
		&bloodline,
		&certificates,
		&farmAddressID,
		&product.PreparationTime,
		&preparationNote,
		&sellingSurfaceRaw,
		&createdAt,
		&updatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("product not found")
		}
		return nil, fmt.Errorf("get product failed: %w", err)
	}

	var mediaURLs []string
	if len(mediaURLsRaw) > 0 && string(mediaURLsRaw) != "null" {
		if err := json.Unmarshal(mediaURLsRaw, &mediaURLs); err != nil {
			return nil, fmt.Errorf("unmarshal product media urls failed: %w", err)
		}
	}

	product.MediaURLs = mediaURLs
	product.SizeCm = sizeCM
	product.AgeMonths = ageMonths
	product.Gender = gender
	product.Breeder = breeder
	product.Bloodline = bloodline
	product.Certificates = certificates
	product.FarmAddressID = farmAddressID
	product.PreparationNote = preparationNote
	if sellingSurfaceRaw != nil {
		product.SellingSurface = entity.SellingSurface(*sellingSurfaceRaw)
	}
	product.CreatedAt = createdAt
	product.UpdatedAt = updatedAt

	return &product, nil
}

// ClaimSellingSurface atomically claims a selling surface for a Product.
// It uses SELECT ... FOR UPDATE to serialize concurrent transactions,
// then checks that selling_surface is not the OPPOSITE type before setting it.
//
// INVARIANT: A Product may belong to exactly one selling surface
// (for_sale OR auction). Cross-type attachment and second ForSale rows are rejected.
// ForSale row uniqueness is enforced by the database migration.
//
// Returns:
//   - nil on success (surface claimed)
//   - error if Product already has a selling surface
//   - error if surface is the opposite type
//   - error if product does not exist
func (r *ProductRepositoryImpl) ClaimSellingSurface(
	ctx context.Context,
	tx db.Tx,
	productID uuid.UUID,
	surface entity.SellingSurface,
) error {
	var currentSurface *string
	err := tx.QueryRow(ctx, `
		SELECT selling_surface
		FROM products
		WHERE id = $1
		FOR UPDATE
	`, productID).Scan(&currentSurface)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("product not found: %w", err)
		}
		return fmt.Errorf("claim selling surface: %w", err)
	}

	if currentSurface != nil {
		current := entity.SellingSurface(*currentSurface)
		// Reject CROSS-TYPE attachment (ForSale→Auction or Auction→ForSale).
		if current != surface {
			return fmt.Errorf("product %s is already attached to %s, cannot attach to %s", productID, current, surface)
		}
		return fmt.Errorf("product %s already has selling surface %s", productID, current)
	}

	_, err = tx.Exec(ctx, `
		UPDATE products
		SET selling_surface = $2, updated_at = NOW()
		WHERE id = $1
	`, productID, string(surface))
	if err != nil {
		return fmt.Errorf("claim selling surface update: %w", err)
	}

	return nil
}

func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func mustMarshalJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
