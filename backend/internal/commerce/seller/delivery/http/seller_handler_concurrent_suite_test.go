//go:build integration

package http

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	sellerrepository "github.com/labuda/backend/internal/commerce/seller/infrastructure/repository"
	sellerrepoiface "github.com/labuda/backend/internal/commerce/seller/repository"
	subscriptionApp "github.com/labuda/backend/internal/commerce/subscription/application"
	"github.com/labuda/backend/internal/config"
	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
	addressrepository "github.com/labuda/backend/internal/identity/address/infrastructure/repository"
	userEntity "github.com/labuda/backend/internal/identity/user/domain/entity"
	userrepoimpl "github.com/labuda/backend/internal/identity/user/infrastructure/repository"
	userrepository "github.com/labuda/backend/internal/identity/user/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// SellerHandlerTestSuite provides a minimal integration harness for onboarding tests.
type SellerHandlerTestSuite struct {
	suite.Suite

	testDB      *testdb.TestDB
	db          *db.DB
	appDB       *db.DB
	userRepo    userrepository.UserRepository
	sellerRepo  sellerrepoiface.SellerRepository
	addressRepo *addressrepository.AddressRepositoryImpl

	handler *SellerHandler
	router  *gin.Engine

	currentUserID uuid.UUID
}

func TestSellerHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(SellerHandlerTestSuite))
}

func (s *SellerHandlerTestSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)

	testDB, _ := testdb.SetupDB(s.T())
	s.testDB = testDB
	s.T().Cleanup(func() {
		if s.testDB != nil {
			s.testDB.Pool().Close()
		}
		if s.appDB != nil {
			s.appDB.Close()
		}
	})

	cfg, err := config.Load()
	require.NoError(s.T(), err)

	s.appDB, err = db.New(context.Background(), db.Config{
		ConnString: cfg.Database.GetTestDSN(),
	})
	require.NoError(s.T(), err)

	userRepo := userrepoimpl.NewUserRepository(s.appDB)
	sellerRepo := sellerrepository.NewSellerRepository()
	addressRepo := addressrepository.NewAddressRepository()
	onboardingService := subscriptionApp.NewSellerOnboardingService(userRepo, sellerRepo, addressRepo)

	s.db = s.appDB
	s.userRepo = userRepo
	s.sellerRepo = sellerRepo
	s.addressRepo = addressRepo

	s.handler = &SellerHandler{
		db:                s.appDB,
		log:               zap.NewNop(),
		userRepo:          userRepo,
		sellerRepo:        sellerRepo,
		onboardingService: onboardingService,
	}

	s.router = gin.New()
	s.router.POST(
		"/api/v1/seller/onboarding",
		func(c *gin.Context) {
			c.Set("userID", s.currentUserID)
			s.handler.Onboarding(c)
		},
	)
}

func (s *SellerHandlerTestSuite) performRequest(method, path string, body io.Reader) *httptest.ResponseRecorder {
	req, err := http.NewRequest(method, path, body)
	require.NoError(s.T(), err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w
}

func (s *SellerHandlerTestSuite) seedCompleteOnboardingFixture(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	storeName string,
) error {
	now := time.Now()

	username := "seller-" + userID.String()[:8]
	bio := "Test seller bio"
	if _, err := s.userRepo.UpdateProfile(ctx, tx, userID, &userEntity.UpdateProfileInput{
		Username: &username,
		Bio:      &bio,
	}); err != nil {
		return err
	}

	address := &addressEntity.Address{
		ID:                     uuid.New(),
		UserID:                 userID,
		Purpose:                addressEntity.AddressPurposeSender,
		Nickname:               "Farm",
		RecipientName:          storeName,
		Phone:                  "+628123456789",
		ProvinceID:             "32",
		ProvinceName:           "Jawa Barat",
		CityID:                 "3204",
		CityName:               "Bandung",
		DistrictID:             "320401",
		DistrictName:           "Coblong",
		VillageID:              "3204011001",
		VillageName:            "Dago",
		StreetAddress:          "Jl. Test No. 1",
		PostalCode:             "40135",
		IsPrimary:              true,
		IsAvailableForCheckout: true,
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	if err := s.addressRepo.Create(ctx, tx, address); err != nil {
		return err
	}

	return nil
}


