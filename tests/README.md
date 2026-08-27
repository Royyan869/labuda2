# Tests

This directory contains integration and end-to-end tests for Labuda.

## Structure

```
tests/
├── integration/    # Integration tests (API, database, services)
└── e2e/            # End-to-end tests (full user flows)
```

## Integration Tests

Integration tests verify that different components work together correctly:
- API endpoint integration
- Database operations
- Service layer integration
- External service mocking

### Running Integration Tests
```bash
cd tests/integration
# TODO: Add test commands when implemented
```

## E2E Tests

End-to-end tests verify complete user workflows:
- User registration and login flows
- Product listing and purchase flows
- Auction participation flows
- Admin operations

### Running E2E Tests
```bash
cd tests/e2e
# TODO: Add test commands when implemented
```

## Best Practices

1. **Keep tests isolated**: Each test should be independent
2. **Use fixtures**: Create reusable test data
3. **Mock external services**: Don't rely on external APIs in tests
4. **Clean up after tests**: Remove test data after test completion
5. **Use descriptive names**: Test names should describe what they test

## Future Improvements

- [ ] Setup integration test framework (Go for backend, Jest for frontend)
- [ ] Setup E2E test framework (Playwright/Cypress)
- [ ] Add CI/CD integration
- [ ] Add test coverage reporting
- [ ] Create shared test utilities
