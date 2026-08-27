# Integration Tests

This directory contains integration tests for the Labuda backend.

## 🧪 Test Types

### Unit Tests
- Location: `internal/*/..._test.go`
- Run with: `go test ./internal/...`
- No database required
- Fast execution (<1s)

### Integration Tests
- Location: `tests/integration/..._test.go`
- Run with: `go test -tags=integration ./tests/integration`
- Requires database (SQLite for local, PostgreSQL for CI/CD)
- Slower execution (1-10s)

---

## 🚀 Running Tests

### Prerequisites

**For Windows (CGO required for SQLite):**

```bash
# Install GCC (MinGW-w64)
choco install mingw

# Or download from: https://winlibs.com/
```

**For Linux/macOS:**
```bash
# CGO is usually enabled by default
```

### Run All Unit Tests

```bash
cd backend
go test ./internal/...
```

### Run Integration Tests

```bash
cd backend

# With SQLite (requires CGO)
go test -tags=integration ./tests/integration

# With verbose output
go test -tags=integration -v ./tests/integration

# Run specific test
go test -tags=integration -v ./tests/integration -run TestGetReports_AdminUser

# With coverage
go test -tags=integration -cover ./tests/integration
```

### Run All Tests (Unit + Integration)

```bash
cd backend
go test -tags=integration ./... -v
```

---

## 📋 Integration Test Coverage

### Content Report Authorization (Security Fix)

| Test | Purpose | Expected Result |
|------|---------|-----------------|
| `TestGetReports_AdminUser_Success` | Admin can list reports | 200 OK |
| `TestGetReports_ModeratorUser_Success` | Moderator can list reports | 200 OK |
| `TestGetReports_RegularUser_Forbidden` | Regular user blocked | 403 Forbidden |
| `TestGetReports_Unauthenticated_Unauthorized` | No auth blocked | 401 Unauthorized |
| `TestReviewReport_AdminUser_Success` | Admin can review | 200 OK |
| `TestReviewReport_ModeratorUser_Success` | Moderator can review | 200 OK |
| `TestReviewReport_RegularUser_Forbidden` | Regular user blocked | 403 Forbidden |
| `TestReviewReport_Unauthenticated_Unauthorized` | No auth blocked | 401 Unauthorized |
| `TestReportContent_RegularUser_Success` | Regular user can report | 201 Created |
| `TestReportContent_Unauthenticated_Unauthorized` | No auth blocked | 401 Unauthorized |
| `TestAuthorizationFlow_CompleteScenario` | Full flow test | All scenarios work |

**Total:** 11 integration tests

---

## 🐳 Running with Docker (PostgreSQL)

For production-like testing with PostgreSQL:

```bash
# Start PostgreSQL
docker-compose -f docker-compose.test.yml up -d

# Run tests against PostgreSQL
DATABASE_URL="postgresql://postgres:password@localhost:5432/labuda_test" \
go test -tags=integration ./tests/integration

# Stop PostgreSQL
docker-compose -f docker-compose.test.yml down
```

---

## 🔍 CI/CD Integration

### GitHub Actions Example

```yaml
name: Integration Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_DB: labuda_test
          POSTGRES_PASSWORD: password
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Run unit tests
        run: go test ./internal/...

      - name: Run integration tests
        env:
          DATABASE_URL: postgresql://postgres:password@localhost:5432/labuda_test
        run: go test -tags=integration ./tests/integration
```

---

## 📝 Writing Integration Tests

### Test Structure

```go
// +build integration

package integration

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMyFeature_Success(t *testing.T) {
    // Arrange
    ctx := setupTestContext(t)

    // Act
    result := performAction(ctx)

    // Assert
    assert.Equal(t, expected, result)
}
```

### Best Practices

1. **Use build tags:** `// +build integration` at the top
2. **Clean up:** Defer cleanup in setup functions
3. **Isolated tests:** Each test should be independent
4. **Clear names:** `TestFeature_Scenario_ExpectedResult`
5. **Arrange-Act-Assert:** Follow AAA pattern
6. **Use helpers:** Create setup/teardown helpers
7. **Test data:** Use fixtures or factories

### Test Helpers

```go
// setupTestContext creates a complete test environment
func setupTestContext(t *testing.T) *TestContext {
    db := setupTestDB(t)
    router := setupTestRouter(db)
    users := createTestUsers(t, db)

    return &TestContext{
        DB:     db,
        Router: router,
        Users:  users,
    }
}

// createTestUser creates a user with specified roles
func createTestUser(t *testing.T, db *gorm.DB, roles []model.UserRole) *model.User {
    user := &model.User{
        ID:    uuid.New(),
        Email: fmt.Sprintf("user-%s@test.com", uuid.New().String()[:8]),
    }

    for _, role := range roles {
        user.AddRole(role)
    }

    require.NoError(t, db.Create(user).Error)
    return user
}
```

---

## 🐛 Troubleshooting

### CGO Error on Windows

**Error:** `Binary was compiled with 'CGO_ENABLED=0', go-sqlite3 requires cgo to work`

**Solution:**
```bash
# Install MinGW
choco install mingw

# Enable CGO
set CGO_ENABLED=1

# Run tests
go test -tags=integration ./tests/integration
```

### Database Connection Error

**Error:** `Failed to connect to test database`

**Solution:**
- Check PostgreSQL is running: `docker ps`
- Verify DATABASE_URL is correct
- Check firewall settings
- Use SQLite for local tests: (automatic in integration tests)

### Import Cycle Error

**Error:** `import cycle not allowed`

**Solution:**
- Move shared test helpers to `tests/testutil` package
- Use interfaces to break cycles
- Avoid importing handler in repository tests

---

## 📚 References

- **GUIDE.md:** Architecture and testing guidelines
- **User Model Tests:** `internal/domain/user/model/user_test.go`
- **Middleware Tests:** `internal/middleware/role_middleware_test.go`

---

## ✅ Test Checklist

Before committing:

- [ ] All unit tests passing
- [ ] All integration tests passing
- [ ] Code coverage > 70%
- [ ] No build errors
- [ ] Build tags added (`// +build integration`)
- [ ] Tests are isolated (no dependencies between tests)
- [ ] Cleanup code in place (defer teardown)
- [ ] Clear test names following convention

---

**Last Updated:** 2026-01-19
**Test Framework:** Go testing + testify + GORM (SQLite/PostgreSQL)
