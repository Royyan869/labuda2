# Unhinged Console E2E Test Suite

## Overview

This test suite validates all MVP demo features of the Unhinged Console using Playwright for end-to-end testing. It covers all four demo requirements (DE3-DE6) with comprehensive BDD scenarios.

## Demo Features Covered

### DE3 - Console System Mode (DE6)
- Mode activation and deactivation
- System status display
- Screen clearing
- State reset functionality
- Command handling
- Error recovery

### DE4 - GUI Console Interaction (DE5)
- Welcome banner display
- Input handling and validation
- Keyboard shortcuts (Ctrl+C, Ctrl+L)
- Message coloring (success, error, info, warning)
- Window resize handling
- Performance and responsiveness

### DE5 - AI Conversation Mode (DE6)
- AI mode activation
- Message sending and receiving
- Provider error handling
- Conversation history management
- Response formatting
- Special character handling

### DE6 - Interactive Learn Mode (DE7)
- Tutorial activation and navigation
- Help system
- Keyboard shortcuts reference
- Key bindings display
- Tutorial consistency
- Beginner-friendly content

## Test Structure

```
tests/
├── page-objects/
│   ├── ConsolePage.ts      # Page Object for console interactions
│   └── index.ts            # Export barrel file
├── utils/
│   └── test-helpers.ts     # Test utilities and constants
├── ai-mode.spec.ts         # DE5 AI mode tests
├── console-mode.spec.ts    # DE3 Console mode tests
├── learn-mode.spec.ts      # DE6 Learn mode tests
├── gui-behavior.spec.ts    # DE4 GUI behavior tests
├── integration.spec.ts     # Cross-feature integration tests
└── README.md               # This file
```

## Running Tests

### Prerequisites
- Node.js 18+ installed
- Playwright browsers installed

### Installation
```bash
# Install dependencies
npm install

# Install Playwright browsers
npx playwright install
```

### Running All Tests
```bash
npm test
```

### Running Specific Test Suites
```bash
# AI mode tests
npm run test:ai

# Console mode tests
npm run test:console

# Learn mode tests
npm run test:learn

# GUI behavior tests
npm run test:gui

# Integration tests
npm run test:integration
```

### Debugging Tests
```bash
# Run with browser visible
npm run test:headed

# Run with Playwright debugger
npm run test:debug

# Run with Playwright UI mode
npm run test:ui
```

### Viewing Reports
```bash
# Generate and view HTML report
npm run report
```

## Test Configuration

### Browser Support
Tests run on:
- Chromium (Desktop Chrome)
- Firefox (Desktop Firefox)
- WebKit (Desktop Safari)

### Environment Variables
- `CI=true`: Enables retries and parallel worker limits
- `BASE_URL`: Override the application URL (default: http://localhost:3000)

### Web Server
The test suite automatically starts the development server before running tests. Configure the server command in `playwright.config.ts`.

## Writing New Tests

### Using the ConsolePage Object
```typescript
import { ConsolePage } from './page-objects/ConsolePage';

test('My new test', async ({ page }) => {
  const console = new ConsolePage(page);
  await console.goto();
  await console.executeCommand('ai');
  expect(await console.getCurrentMode()).toBe('ai');
});
```

### Available ConsolePage Methods
- `goto()` - Navigate to the console
- `waitForReady()` - Wait for console to load
- `executeCommand(cmd)` - Type and execute a command
- `typeCommand(text)` - Type without executing
- `pressEnter()` - Execute current input
- `getOutputText()` - Get all output text
- `outputContains(text)` - Check if output contains text
- `waitForOutput(text)` - Wait for specific text to appear
- `hasSuccessMessage()` - Check for green success message
- `hasErrorMessage()` - Check for red error message
- `hasInfoMessage()` - Check for blue info message
- `hasWarningMessage()` - Check for orange warning message
- `getCurrentMode()` - Get current console mode
- `pressCtrlC()` - Press Ctrl+C
- `pressCtrlL()` - Press Ctrl+L
- `isBannerVisible()` - Check if banner is visible
- `getVersion()` - Get version from banner
- `measureCommandResponseTime(cmd)` - Measure command response time

## BDD Feature Files

Feature files are located in the `features/` directory and follow Gherkin syntax:

```
features/
├── ai.feature           # DE5 AI conversation scenarios
├── console.feature      # DE3 Console system scenarios
├── learn.feature        # DE6 Interactive learn scenarios
├── gui-behavior.feature # DE4 GUI interaction scenarios
└── integration.feature  # Cross-feature integration scenarios
```

## CI/CD Integration

### GitHub Actions Example
```yaml
name: E2E Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-node@v3
        with:
          node-version: '18'
      - run: npm ci
      - run: npx playwright install --with-deps
      - run: npm test
      - uses: actions/upload-artifact@v3
        if: always()
        with:
          name: playwright-report
          path: playwright-report/
```

## Troubleshooting

### Tests fail to start
- Ensure the development server is running
- Check if port 3000 is available
- Verify Playwright browsers are installed

### Console not responding
- Check if the application is fully loaded
- Increase timeouts in playwright.config.ts
- Run tests with `--headed` to see what's happening

### Flaky tests
- Check for race conditions in async operations
- Use `waitForOutput()` instead of fixed timeouts
- Ensure proper test isolation with `beforeEach`

## Contributing

1. Write tests for new features
2. Follow the existing Page Object pattern
3. Add new scenarios to appropriate feature files
4. Update this README for significant changes
5. Run full test suite before submitting PRs
