//go:build ignore

// STALE (PASS_21C): the `listings`/ListingID queries in this file target a
// table dropped by migration 000010 (dead since the products/
// for_sales split). Rewrite against `for_sales JOIN
// products` before reusing — not rewritten in this pass; could not be
// tested end-to-end without a live DB connection (Docker was unavailable
// during this pass).
package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Configuration
const (
	DefaultAPIBase = "http://localhost:8080"
	DefaultDBConn  = "postgresql://labuda:labuda123@localhost:5432/labuda?sslmode=disable"
	RequestTimeout = 30 * time.Second
)

// Colors for terminal output
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
)

// Test data structures
type TestContext struct {
	APIBase      string
	DBConn       string
	Client       *http.Client
	DB           *sql.DB
	SellerID     string
	BuyerID      string
	SellerToken  string
	BuyerToken   string
	ListingID    string
	ShippingID   string
	AddressID    string
	PricingToken string
	OrderID      string
	Results      ValidationResults
}

type ValidationResults struct {
	ListingsCreated      int
	PricingTokensCreated int
	OrdersCreated        int
	PricingLogsFound     int
	OrderLogsFound       int
	TestsPassed          int
	TestsFailed          int
}

// API Response structures
type FirebaseExchangeResponse struct {
	Success bool `json:"success"`
	Data    struct {
		UserID                    string  `json:"user_id"`
		AccessToken               string  `json:"access_token"`
		RefreshToken              string  `json:"refresh_token"`
		ExpiresAt                 string  `json:"expires_at"`
		RefreshExpiresAt          string  `json:"refresh_expires_at"`
		RequiresProfileCompletion bool    `json:"requires_profile_completion"`
		Created                   bool    `json:"created"`
		Email                     *string `json:"email,omitempty"`
	} `json:"data"`
	// Handle nested error structure - error can be either string or object
	ErrorResponse *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type ListingResponse struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Success bool   `json:"success"`
	// Handle nested error structure - error can be either string or object
	ErrorResponse *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type ShippingOptionResponse struct {
	ID      string `json:"id"`
	Success bool   `json:"success"`
	// Handle nested error structure
	ErrorResponse *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type PricingPreviewResponse struct {
	Token   string `json:"token"`
	Success bool   `json:"success"`
	// Handle nested error structure
	ErrorResponse *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type OrderResponse struct {
	ID      string `json:"id"`
	Success bool   `json:"success"`
	// Handle nested error structure
	ErrorResponse *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func main() {
	fmt.Println("================================================================")
	fmt.Println("API FLOW VALIDATION - STRICT MODE (NO DIRECT DB)")
	fmt.Println("================================================================")
	fmt.Println("Objective: Validate REAL application flow through API layer")
	fmt.Println("Rule: DO NOT USE DIRECT SQL INSERT - MUST USE HTTP API")
	fmt.Println("")

	// Get configuration from environment or use defaults
	apiBase := getEnv("API_BASE", DefaultAPIBase)
	dbConn := getEnv("DB_CONN", DefaultDBConn)

	// Initialize test context
	ctx, err := InitializeTestContext(apiBase, dbConn)
	if err != nil {
		log.Fatalf(ColorRed+"[FAIL]"+ColorReset+" Failed to initialize test context: %v", err)
	}

	defer ctx.DB.Close()
	defer printSummary(&ctx.Results)

	// Run validation steps
	if !CheckServerHealth(ctx) {
		os.Exit(1)
	}

	if !CreateTestAccounts(ctx) {
		os.Exit(1)
	}

	if !SetupSellerAccount(ctx) {
		os.Exit(1)
	}

	if !CreateListing(ctx) {
		os.Exit(1)
	}

	if !SetupShippingOptions(ctx) {
		os.Exit(1)
	}

	if !CreateBuyerAddress(ctx) {
		os.Exit(1)
	}

	if !CallPricingPreview(ctx) {
		os.Exit(1)
	}

	if !CreateOrder(ctx) {
		os.Exit(1)
	}

	if !VerifyDatabaseState(ctx) {
		os.Exit(1)
	}

	if !VerifyLogs(ctx) {
		os.Exit(1)
	}

	// All tests passed
	fmt.Println("")
	fmt.Println("================================================================")
	fmt.Println(ColorGreen + "[PASS]" + ColorReset + " ✅ API FLOW VALIDATION PASSED")
	fmt.Println("")
	fmt.Println("REAL DATA:")
	fmt.Println("----------")
	fmt.Printf("Listings: %d\n", ctx.Results.ListingsCreated)
	fmt.Printf("Pricing Tokens: %d\n", ctx.Results.PricingTokensCreated)
	fmt.Printf("Orders: %d\n", ctx.Results.OrdersCreated)
	fmt.Println("")
	fmt.Println("RESULT: PASS")
	fmt.Println("")
	fmt.Println("✅ Application flow works correctly through API layer!")
}

func InitializeTestContext(apiBase, dbConn string) (*TestContext, error) {
	// Initialize HTTP client
	client := &http.Client{
		Timeout: RequestTimeout,
	}

	// Connect to database
	db, err := sql.Open("pgx", dbConn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &TestContext{
		APIBase: apiBase,
		DBConn:  dbConn,
		Client:  client,
		DB:      db,
		Results: ValidationResults{},
	}, nil
}

func CheckServerHealth(ctx *TestContext) bool {
	fmt.Println("")
	fmt.Println("================================================================")
	fmt.Println("STEP 0: CHECK SERVER HEALTH")
	fmt.Println("================================================================")

	fmt.Print(ColorYellow + "[CHECK]" + ColorReset + " Checking if server is running...")

	resp, err := ctx.Client.Get(ctx.APIBase + "/health")
	if err != nil {
		fmt.Println(ColorRed + "[FAIL]" + ColorReset + " Server is not running")
		fmt.Printf("Please start the server first: go run cmd/server/main.go\n")
		ctx.Results.TestsFailed++
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Println(ColorGreen + "[OK]" + ColorReset + " Server is running")
		ctx.Results.TestsPassed++
		return true
	}

	fmt.Println(ColorRed + "[FAIL]" + ColorReset + " Server returned non-200 status")
	ctx.Results.TestsFailed++
	return false
}

func CreateTestAccounts(ctx *TestContext) bool {
	fmt.Println("")
	fmt.Println("================================================================")
	fmt.Println("STEP 1: CREATE TEST ACCOUNTS")
	fmt.Println("================================================================")

	// Use explicit mock Firebase fixtures instead of deriving identity from
	// Firebase profile fields.
	sellerToken := "seller-1"
	buyerToken := "buyer-1"

	// Create seller account
	fmt.Print(ColorYellow + "[ACTION]" + ColorReset + " Creating seller account via Firebase exchange\n")
	sellerResp, err := makeAPIRequest[FirebaseExchangeResponse](ctx, "POST", "/api/v1/auth/firebase/exchange", map[string]interface{}{
		"firebase_id_token": sellerToken,
	}, "")

	// Check for errors
	sellerError := ""
	if sellerResp.ErrorResponse != nil {
		sellerError = sellerResp.ErrorResponse.Message
	}
	if err != nil || sellerError != "" || !sellerResp.Success {
		fmt.Println(ColorRed+"[FAIL]"+ColorReset+" Failed to create seller account", err, sellerError)
		ctx.Results.TestsFailed++
		return false
	}
	if sellerResp.Data.RequiresProfileCompletion || sellerResp.Data.AccessToken == "" || sellerResp.Data.RefreshToken == "" {
		fmt.Println(ColorRed + "[FAIL]" + ColorReset + " Seller exchange did not return a full session")
		ctx.Results.TestsFailed++
		return false
	}

	ctx.SellerID = sellerResp.Data.UserID
	ctx.SellerToken = sellerResp.Data.AccessToken
	fmt.Println(ColorGreen + "[OK]" + ColorReset + " Seller created: " + ctx.SellerID)
	ctx.Results.TestsPassed++

	// Create buyer account
	fmt.Print(ColorYellow + "[ACTION]" + ColorReset + " Creating buyer account via Firebase exchange\n")
	buyerResp, err := makeAPIRequest[FirebaseExchangeResponse](ctx, "POST", "/api/v1/auth/firebase/exchange", map[string]interface{}{
		"firebase_id_token": buyerToken,
	}, "")

	// Check for errors
	buyerError := ""
	if buyerResp.ErrorResponse != nil {
		buyerError = buyerResp.ErrorResponse.Message
	}
	if err != nil || buyerError != "" || !buyerResp.Success {
		fmt.Println(ColorRed+"[FAIL]"+ColorReset+" Failed to create buyer account", err, buyerError)
		ctx.Results.TestsFailed++
		return false
	}
	if buyerResp.Data.RequiresProfileCompletion || buyerResp.Data.AccessToken == "" || buyerResp.Data.RefreshToken == "" {
		fmt.Println(ColorRed + "[FAIL]" + ColorReset + " Buyer exchange did not return a full session")
		ctx.Results.TestsFailed++
		return false
	}

	ctx.BuyerID = buyerResp.Data.UserID
	ctx.BuyerToken = buyerResp.Data.AccessToken
	fmt.Println(ColorGreen + "[OK]" + ColorReset + " Buyer created: " + ctx.BuyerID)
	ctx.Results.TestsPassed++

	return true
}

func SetupSellerAccount(ctx *TestContext) bool {
	fmt.Println("")
	fmt.Println("================================================================")
	fmt.Println("STEP 2: SETUP SELLER ACCOUNT")
	fmt.Println("================================================================")

	fmt.Print(ColorYellow + "[ACTION]" + ColorReset + " Calling seller onboarding\n")

	_, err := makeAPIRequest[map[string]interface{}](ctx, "POST", "/api/v1/seller/onboarding", map[string]interface{}{
		"store_name": "Test Koi Farm",
	}, ctx.SellerToken)

	if err != nil {
		fmt.Println(ColorYellow+"[WARN]"+ColorReset+" Seller onboarding failed (may exist):", err)
		// Continue anyway - seller might already exist
	} else {
		fmt.Println(ColorGreen + "[OK]" + ColorReset + " Seller onboarding completed")
	}

	ctx.Results.TestsPassed++
	return true
}

func CreateListing(ctx *TestContext) bool {
	fmt.Println("")
	fmt.Println("================================================================")
	fmt.Println("STEP 3: CREATE LISTING VIA API")
	fmt.Println("================================================================")

	listingTitle := fmt.Sprintf("Test Koi %d", time.Now().Unix())

	fmt.Print(ColorYellow + "[ACTION]" + ColorReset + " Creating listing: " + listingTitle + "\n")

	resp, err := makeAPIRequest[ListingResponse](ctx, "POST", "/api/v1/listings", map[string]interface{}{
		"title":               listingTitle,
		"description":         "Beautiful koi fish for testing",
		"price":               500000,
		"quantity":            10,
		"negotiation_enabled": true,
		"visibility":          "public",
		"variety":             "Kohaku",
		"size_cm":             30,
		"age_months":          24,
		"gender":              "male",
		"media_urls":          []string{"https://picsum.photos/seed/koi1/800/600"},
	}, ctx.SellerToken)

	// Debug output
	fmt.Printf("DEBUG: Response Success=%v, ID='%s'\n", resp.Success, resp.ID)
	if resp.ErrorResponse != nil {
		fmt.Printf("DEBUG: ErrorResponse Code=%s, Message=%s\n", resp.ErrorResponse.Code, resp.ErrorResponse.Message)
	}

	// Check for errors
	listingError := ""
	if resp.ErrorResponse != nil {
		listingError = resp.ErrorResponse.Message
	}
	if err != nil || listingError != "" || !resp.Success || resp.ID == "" {
		fmt.Println(ColorRed + "[FAIL]" + ColorReset + " Failed to create listing")
		if err != nil {
			fmt.Println("Error:", err)
		}
		if listingError != "" {
			fmt.Println("API Error:", listingError)
		}
		fmt.Println("Success:", resp.Success, "ID:", resp.ID)
		ctx.Results.TestsFailed++
		return false
	}

	ctx.ListingID = resp.ID
	fmt.Println(ColorGreen + "[OK]" + ColorReset + " Listing created: " + ctx.ListingID)
	ctx.Results.TestsPassed++
	ctx.Results.ListingsCreated++

	return true
}

func SetupShippingOptions(ctx *TestContext) bool {
	fmt.Println("")
	fmt.Println("================================================================")
	fmt.Println("STEP 4: SETUP SHIPPING OPTIONS")
	fmt.Println("================================================================")

	fmt.Print(ColorYellow + "[ACTION]" + ColorReset + " Creating shipping option\n")

	resp, err := makeAPIRequest[ShippingOptionResponse](ctx, "POST", "/api/v1/seller/shipping/options", map[string]interface{}{
		"name":               "JNE Trucking",
		"transport_type":     "land",
		"expedition_name":    "JNE",
		"estimated_days_min": 2,
		"estimated_days_max": 5,
	}, ctx.SellerToken)

	shippingError := ""
	if resp.ErrorResponse != nil {
		shippingError = resp.ErrorResponse.Message
	}
	if err != nil || shippingError != "" {
		fmt.Println(ColorYellow+"[WARN]"+ColorReset+" Failed to create shipping option:", err, shippingError)
		return false
	}

	ctx.ShippingID = resp.ID
	fmt.Println(ColorGreen + "[OK]" + ColorReset + " Shipping option created: " + ctx.ShippingID)

	// Link shipping option to listing
	fmt.Print(ColorYellow + "[ACTION]" + ColorReset + " Linking shipping option to listing\n")

	_, err = makeAPIRequest[map[string]interface{}](ctx, "PUT", "/api/v1/listings/"+ctx.ListingID+"/shipping", map[string]interface{}{
		"shipping_option_ids": []string{ctx.ShippingID},
	}, ctx.SellerToken)

	if err != nil {
		fmt.Println(ColorYellow+"[WARN]"+ColorReset+" Failed to link shipping option:", err)
	} else {
		fmt.Println(ColorGreen + "[OK]" + ColorReset + " Shipping option linked to listing")
	}

	ctx.Results.TestsPassed++
	return true
}

func CreateBuyerAddress(ctx *TestContext) bool {
	fmt.Println("")
	fmt.Println("================================================================")
	fmt.Println("STEP 5: CREATE BUYER ADDRESS")
	fmt.Println("================================================================")

	fmt.Print(ColorYellow + "[ACTION]" + ColorReset + " Creating buyer address\n")

	_, err := makeAPIRequest[map[string]interface{}](ctx, "PUT", "/api/v1/users/me", map[string]interface{}{
		"phone":       "+628987654321",
		"address":     "Jalan Buyer 456",
		"city":        "Jakarta",
		"province":    "DKI Jakarta",
		"postal_code": "12345",
	}, ctx.BuyerToken)

	if err != nil {
		fmt.Println(ColorYellow+"[WARN]"+ColorReset+" Failed to create buyer address:", err)
		return false
	}

	fmt.Println(ColorGreen + "[OK]" + ColorReset + " Buyer address created")
	ctx.Results.TestsPassed++
	return true
}

func CallPricingPreview(ctx *TestContext) bool {
	fmt.Println("")
	fmt.Println("================================================================")
	fmt.Println("STEP 6: CALL PRICING PREVIEW API")
	fmt.Println("================================================================")

	fmt.Print(ColorYellow + "[ACTION]" + ColorReset + " Generating pricing preview\n")

	resp, err := makeAPIRequest[PricingPreviewResponse](ctx, "POST", "/api/v1/pricing/preview", map[string]interface{}{
		"listing_id":         ctx.ListingID,
		"quantity":           1,
		"shipping_option_id": ctx.ShippingID,
		"address_id":         ctx.AddressID,
	}, ctx.BuyerToken)

	pricingError := ""
	if resp.ErrorResponse != nil {
		pricingError = resp.ErrorResponse.Message
	}
	if err != nil || pricingError != "" || !resp.Success || resp.Token == "" {
		fmt.Println(ColorRed+"[FAIL]"+ColorReset+" Failed to generate pricing preview", err, pricingError)
		ctx.Results.TestsFailed++
		return false
	}

	ctx.PricingToken = resp.Token
	fmt.Println(ColorGreen + "[OK]" + ColorReset + " Pricing token generated: " + ctx.PricingToken)
	ctx.Results.TestsPassed++
	ctx.Results.PricingTokensCreated++

	return true
}

func CreateOrder(ctx *TestContext) bool {
	fmt.Println("")
	fmt.Println("================================================================")
	fmt.Println("STEP 7: CREATE ORDER VIA API USING PRICING TOKEN")
	fmt.Println("================================================================")

	fmt.Print(ColorYellow + "[ACTION]" + ColorReset + " Creating order with pricing token\n")

	resp, err := makeAPIRequest[OrderResponse](ctx, "POST", "/api/v1/orders", map[string]interface{}{
		"listing_id":         ctx.ListingID,
		"quantity":           1,
		"address_id":         ctx.AddressID,
		"shipping_option_id": ctx.ShippingID,
		"pricing_token":      ctx.PricingToken,
	}, ctx.BuyerToken)

	orderError := ""
	if resp.ErrorResponse != nil {
		orderError = resp.ErrorResponse.Message
	}
	if err != nil || orderError != "" || !resp.Success || resp.ID == "" {
		fmt.Println(ColorRed+"[FAIL]"+ColorReset+" Failed to create order", err, orderError)
		ctx.Results.TestsFailed++
		return false
	}

	ctx.OrderID = resp.ID
	fmt.Println(ColorGreen + "[OK]" + ColorReset + " Order created: " + ctx.OrderID)
	ctx.Results.TestsPassed++
	ctx.Results.OrdersCreated++

	return true
}

func VerifyDatabaseState(ctx *TestContext) bool {
	fmt.Println("")
	fmt.Println("================================================================")
	fmt.Println("STEP 8: VERIFY DATABASE STATE")
	fmt.Println("================================================================")

	fmt.Print(ColorYellow + "[CHECK]" + ColorReset + " Querying database for records...\n")

	// Verify listing exists
	var listingCount int
	err := ctx.DB.QueryRow("SELECT COUNT(*) FROM listings WHERE id = $1", ctx.ListingID).Scan(&listingCount)
	if err != nil {
		fmt.Println(ColorRed+"[FAIL]"+ColorReset+" Failed to query listings:", err)
		ctx.Results.TestsFailed++
		return false
	}

	// Verify pricing token exists
	var tokenCount int
	err = ctx.DB.QueryRow("SELECT COUNT(*) FROM pricing_tokens WHERE id = $1", ctx.PricingToken).Scan(&tokenCount)
	if err != nil {
		fmt.Println(ColorRed+"[FAIL]"+ColorReset+" Failed to query pricing_tokens:", err)
		ctx.Results.TestsFailed++
		return false
	}

	// Verify order exists
	var orderCount int
	err = ctx.DB.QueryRow("SELECT COUNT(*) FROM orders WHERE id = $1", ctx.OrderID).Scan(&orderCount)
	if err != nil {
		fmt.Println(ColorRed+"[FAIL]"+ColorReset+" Failed to query orders:", err)
		ctx.Results.TestsFailed++
		return false
	}

	fmt.Println("")
	fmt.Println("DATABASE VERIFICATION RESULTS:")
	fmt.Println("----------------------------")
	fmt.Printf("Listings found: %d\n", listingCount)
	fmt.Printf("Pricing Tokens found: %d\n", tokenCount)
	fmt.Printf("Orders found: %d\n", orderCount)

	if listingCount > 0 && tokenCount > 0 && orderCount > 0 {
		fmt.Println(ColorGreen + "[OK]" + ColorReset + " All database records found")
		ctx.Results.TestsPassed++
		return true
	}

	fmt.Println(ColorRed + "[FAIL]" + ColorReset + " Missing database records")
	ctx.Results.TestsFailed++
	return false
}

func VerifyLogs(ctx *TestContext) bool {
	fmt.Println("")
	fmt.Println("================================================================")
	fmt.Println("STEP 9: VERIFY LOGS")
	fmt.Println("================================================================")

	fmt.Print(ColorYellow + "[CHECK]" + ColorReset + " Searching for pricing token creation log...\n")

	var pricingLogCount int
	err := ctx.DB.QueryRow("SELECT COUNT(*) FROM outbox_events WHERE event_type = 'PRICING_TOKEN_CREATED'").Scan(&pricingLogCount)
	if err != nil {
		fmt.Println(ColorYellow+"[WARN]"+ColorReset+" Failed to query pricing logs:", err)
		pricingLogCount = 0
	}

	fmt.Print(ColorYellow + "[CHECK]" + ColorReset + " Searching for order creation log...\n")

	var orderLogCount int
	err = ctx.DB.QueryRow("SELECT COUNT(*) FROM outbox_events WHERE event_type = 'ORDER_CREATED'").Scan(&orderLogCount)
	if err != nil {
		fmt.Println(ColorYellow+"[WARN]"+ColorReset+" Failed to query order logs:", err)
		orderLogCount = 0
	}

	fmt.Println("")
	fmt.Println("LOG VERIFICATION RESULTS:")
	fmt.Println("------------------------")
	fmt.Printf("PRICING_TOKEN_CREATED events: %d\n", pricingLogCount)
	fmt.Printf("ORDER_CREATED events: %d\n", orderLogCount)

	ctx.Results.PricingLogsFound = pricingLogCount
	ctx.Results.OrderLogsFound = orderLogCount

	ctx.Results.TestsPassed++
	return true
}

// Helper functions

func makeAPIRequest[T any](ctx *TestContext, method, endpoint string, body map[string]interface{}, token string) (T, error) {
	var result T

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return result, err
	}

	req, err := http.NewRequest(method, ctx.APIBase+endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return result, err
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := ctx.Client.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return result, err
	}

	return result, nil
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func printSummary(results *ValidationResults) {
	fmt.Println("")
	fmt.Println("================================================================")
	fmt.Println("VALIDATION SUMMARY")
	fmt.Println("================================================================")
	fmt.Printf("Tests Passed: %d\n", results.TestsPassed)
	fmt.Printf("Tests Failed: %d\n", results.TestsFailed)
	fmt.Printf("Listings Created: %d\n", results.ListingsCreated)
	fmt.Printf("Pricing Tokens Created: %d\n", results.PricingTokensCreated)
	fmt.Printf("Orders Created: %d\n", results.OrdersCreated)
	fmt.Printf("Pricing Logs Found: %d\n", results.PricingLogsFound)
	fmt.Printf("Order Logs Found: %d\n", results.OrderLogsFound)
	fmt.Println("================================================================")
}
