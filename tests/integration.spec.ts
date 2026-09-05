import { test, expect } from '@playwright/test';
import { ConsolePage } from './page-objects/ConsolePage';
import { TEST_DATA } from './utils/test-helpers';

test.describe('Integration - All Demo Features', () => {
  let console: ConsolePage;

  test.beforeEach(async ({ page }) => {
    console = new ConsolePage(page);
    await console.goto();
  });

  test('Full user journey through all modes', async () => {
    // Console mode
    await console.executeCommand('console');
    expect(await console.getCurrentMode()).toBe('console');
    await console.executeCommand('status');
    await console.waitForOutput('Mode:');
    await console.executeCommand('exit');
    expect(await console.getCurrentMode()).toBe('menu');

    // Learn mode
    await console.executeCommand('learn');
    expect(await console.getCurrentMode()).toBe('learn');
    await console.executeCommand('help');
    await console.waitForOutput('tutorial');
    await console.executeCommand('shortcuts');
    await console.waitForOutput('Ctrl+L');
    await console.executeCommand('exit');
    expect(await console.getCurrentMode()).toBe('menu');

    // AI mode
    await console.executeCommand('ai');
    expect(await console.getCurrentMode()).toBe('ai');
    await console.executeCommand('Hello from integration test');
    await console.waitForOutput('user');
    await console.executeCommand('exit');
    expect(await console.getCurrentMode()).toBe('menu');

    // Verify console is still stable
    expect(await console.isPromptVisible()).toBeTruthy();
  });

  test('Mode switching preserves console stability', async () => {
    const modes = ['ai', 'menu', 'console', 'menu', 'learn', 'menu'];
    for (const mode of modes) {
      await console.executeCommand(mode);
      const currentMode = await console.getCurrentMode();
      expect(currentMode).toBe(mode === 'menu' ? 'menu' : mode);
    }
    expect(await console.isPromptVisible()).toBeTruthy();
  });

  test('Error recovery across modes', async () => {
    // Invalid command
    await console.executeCommand('nonexistent_mode');
    expect(await console.hasWarningMessage()).toBeTruthy();

    // AI mode with potential failure
    await console.executeCommand('ai');
    await console.executeCommand('This will fail');
    await console.page.waitForTimeout(500);

    // Return to menu
    await console.executeCommand('menu');
    expect(await console.getCurrentMode()).toBe('menu');

    // Console mode
    await console.executeCommand('console');
    await console.executeCommand('status');
    await console.waitForOutput('Mode:');

    // Verify stability
    expect(await console.isPromptVisible()).toBeTruthy();
  });

  test('GUI displays all message types correctly', async () => {
    // Green success message
    await console.executeCommand('ai');
    expect(await console.hasSuccessMessage()).toBeTruthy();
    await console.executeCommand('menu');

    // Blue info message
    await console.executeCommand('help');
    expect(await console.hasInfoMessage()).toBeTruthy();

    // Orange warning message
    await console.executeCommand('');
    expect(await console.hasWarningMessage()).toBeTruthy();

    // Verify prompt remains visible
    expect(await console.isPromptVisible()).toBeTruthy();
  });

  test('Console maintains state across interactions', async () => {
    // Enter AI mode
    await console.executeCommand('ai');
    expect(await console.getCurrentMode()).toBe('ai');

    // Exit to menu
    await console.executeCommand('menu');
    expect(await console.getCurrentMode()).toBe('menu');

    // Enter console mode
    await console.executeCommand('console');
    expect(await console.getCurrentMode()).toBe('console');

    // Check status
    await console.executeCommand('status');
    await console.waitForOutput('Mode:');

    // Verify state is maintained
    expect(await console.isPromptVisible()).toBeTruthy();
  });

  test('Console startup performance', async () => {
    const startTime = Date.now();
    await console.goto();
    const loadTime = Date.now() - startTime;
    expect(loadTime).toBeLessThan(3000);
    expect(await console.isBannerVisible()).toBeTruthy();
  });

  test('Console input responsiveness', async () => {
    const commands = ['help', 'status', 'ai', 'menu', 'console', 'status', 'exit', 'learn', 'exit', 'help'];
    const responseTimes: number[] = [];

    for (const cmd of commands) {
      const time = await console.measureCommandResponseTime(cmd);
      responseTimes.push(time);
    }

    // All commands should respond within 100ms
    for (const time of responseTimes) {
      expect(time).toBeLessThan(100);
    }
  });
});
