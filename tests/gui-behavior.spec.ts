import { test, expect } from '@playwright/test';
import { ConsolePage } from './page-objects/ConsolePage';
import { TEST_DATA } from './utils/test-helpers';

test.describe('GUI Console Behavior (DE4)', () => {
  let console: ConsolePage;

  test.beforeEach(async ({ page }) => {
    console = new ConsolePage(page);
    await console.goto();
  });

  test('Console displays welcome banner on startup', async () => {
    expect(await console.isBannerVisible()).toBeTruthy();
    const output = await console.getOutputText();
    expect(output).toContain(TEST_DATA.BANNER_TEXT);
    expect(output).toContain(TEST_DATA.VERSION);
    expect(output).toContain(TEST_DATA.READY_MESSAGE);
  });

  test('Console prompt is visible and positioned correctly', async () => {
    expect(await console.isPromptVisible()).toBeTruthy();
    const promptBox = await console.prompt.boundingBox();
    const pageBox = await console.page.viewportSize();
    if (promptBox && pageBox) {
      // Prompt should be near the bottom
      expect(promptBox.y).toBeGreaterThan(pageBox.height * 0.5);
    }
  });

  test('Console accepts text input', async () => {
    await console.typeCommand('hello world');
    const value = await console.getInputValue();
    expect(value).toBe('hello world');
  });

  test('Console processes Enter key', async () => {
    await console.typeCommand('menu');
    await console.pressEnter();
    // Command should be processed
    await console.page.waitForTimeout(100);
    expect(await console.isPromptVisible()).toBeTruthy();
  });

  test('Console handles Ctrl+C gracefully', async () => {
    await console.pressCtrlC();
    await console.waitForOutput('Goodbye');
    const output = await console.getOutputText();
    expect(output).toContain('Goodbye');
  });

  test('Console handles Ctrl+L for clear screen', async () => {
    // First type something to have content
    await console.executeCommand('help');
    // Then clear
    await console.pressCtrlL();
    expect(await console.isBannerVisible()).toBeTruthy();
  });

  test('Console displays error messages correctly', async () => {
    await console.executeCommand('invalid_mode');
    expect(await console.hasWarningMessage()).toBeTruthy();
    await console.waitForOutput('Unknown');
  });

  test('Console displays success messages in green', async () => {
    await console.executeCommand('ai');
    expect(await console.hasSuccessMessage()).toBeTruthy();
  });

  test('Console displays info messages in blue', async () => {
    await console.executeCommand('help');
    expect(await console.hasInfoMessage()).toBeTruthy();
  });

  test('Console displays warning messages in orange', async () => {
    await console.executeCommand('');
    expect(await console.hasWarningMessage()).toBeTruthy();
  });

  test('Console handles window resize', async ({ page }) => {
    await page.setViewportSize({ width: 800, height: 600 });
    await console.waitForReady();
    expect(await console.isBannerVisible()).toBeTruthy();
    expect(await console.isPromptVisible()).toBeTruthy();

    await page.setViewportSize({ width: 1200, height: 800 });
    await console.waitForReady();
    expect(await console.isBannerVisible()).toBeTruthy();
    expect(await console.isPromptVisible()).toBeTruthy();
  });

  test('Console input is cleared after command execution', async () => {
    await console.typeCommand('test_input');
    await console.pressEnter();
    await console.page.waitForTimeout(100);
    const value = await console.getInputValue();
    expect(value).toBe('');
  });

  test('Console handles rapid input', async () => {
    const longText = 'A'.repeat(100);
    await console.typeCommand(longText);
    const value = await console.getInputValue();
    expect(value).toBe(longText);
    expect(await console.isPromptVisible()).toBeTruthy();
  });

  test('Console handles special characters', async () => {
    await console.typeCommand('!@#$%^&*()_+-={}[]|;:\'",./<>?');
    await console.pressEnter();
    // Should process without error
    await console.page.waitForTimeout(100);
    expect(await console.isPromptVisible()).toBeTruthy();
  });

  test('Console handles empty input', async () => {
    await console.pressEnter();
    expect(await console.hasWarningMessage()).toBeTruthy();
  });

  test('Console preserves input history context', async () => {
    await console.executeCommand('mode1');
    await console.executeCommand('mode2');
    // Both commands should be processed independently
    expect(await console.isPromptVisible()).toBeTruthy();
  });

  test('Console displays colored text correctly', async () => {
    // Test green message
    await console.executeCommand('ai');
    expect(await console.hasSuccessMessage()).toBeTruthy();
    await console.executeCommand('menu');

    // Test blue message
    await console.executeCommand('help');
    expect(await console.hasInfoMessage()).toBeTruthy();

    // Test orange message
    await console.executeCommand('');
    expect(await console.hasWarningMessage()).toBeTruthy();
  });

  test('Console handles multiple rapid commands', async () => {
    const commands = ['help', 'status', 'ai', 'menu', 'console', 'status', 'exit', 'learn', 'exit', 'help'];
    await console.rapidExecute(commands);
    // Console should remain responsive
    expect(await console.isPromptVisible()).toBeTruthy();
  });
});
