import { test, expect } from '@playwright/test';
import { ConsolePage } from './page-objects/ConsolePage';
import { TEST_DATA } from './utils/test-helpers';

test.describe('Console System Mode (DE3)', () => {
  let console: ConsolePage;

  test.beforeEach(async ({ page }) => {
    console = new ConsolePage(page);
    await console.goto();
  });

  test('Console mode is activated successfully', async () => {
    await console.executeCommand('console');
    const mode = await console.getCurrentMode();
    expect(mode).toBe('console');
    expect(await console.hasSuccessMessage()).toBeTruthy();
    expect(await console.isBannerVisible()).toBeTruthy();
  });

  test('Console status command', async () => {
    await console.executeCommand('console');
    await console.executeCommand('status');
    await console.waitForOutput('Mode:');
    await console.waitForOutput('Uptime:');
    const output = await console.getOutputText();
    expect(output).toContain('Mode:');
    expect(output).toContain('Uptime:');
  });

  test('Console clear command', async () => {
    await console.executeCommand('console');
    await console.executeCommand('clear');
    expect(await console.isBannerVisible()).toBeTruthy();
  });

  test('Console reset command - confirm', async () => {
    await console.executeCommand('console');
    await console.executeCommand('reset');
    await console.waitForOutput('Are you sure');
    await console.executeCommand('y');
    await console.waitForOutput('reset');
    const mode = await console.getCurrentMode();
    expect(mode).toBe('menu');
  });

  test('Console reset command - cancel', async () => {
    await console.executeCommand('console');
    await console.executeCommand('reset');
    await console.waitForOutput('Are you sure');
    await console.executeCommand('n');
    await console.waitForOutput('cancelled');
    const mode = await console.getCurrentMode();
    expect(mode).toBe('console');
  });

  test('Console returns unknown command', async () => {
    await console.executeCommand('console');
    await console.executeCommand('unknown_command');
    expect(await console.hasWarningMessage()).toBeTruthy();
    await console.waitForOutput('Unknown');
  });

  test('Console shows banner on entry', async () => {
    await console.executeCommand('console');
    expect(await console.isBannerVisible()).toBeTruthy();
    const version = await console.getVersion();
    expect(version).toBe(TEST_DATA.VERSION);
  });

  test('Console handles concurrent commands', async () => {
    await console.executeCommand('console');
    const commands = Array(5).fill('status');
    await console.rapidExecute(commands);
    // Console should not crash
    expect(await console.isPromptVisible()).toBeTruthy();
  });

  test('Console status shows all system components', async () => {
    await console.executeCommand('console');
    await console.executeCommand('status');
    await console.waitForOutput('GUI');
    await console.waitForOutput('AI');
    const output = await console.getOutputText();
    expect(output).toContain('GUI');
    expect(output).toContain('AI');
  });

  test('Console displays correct version', async () => {
    await console.executeCommand('console');
    await console.executeCommand('status');
    await console.waitForOutput(TEST_DATA.VERSION);
    const output = await console.getOutputText();
    expect(output).toContain(TEST_DATA.VERSION);
  });
});
