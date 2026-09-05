import { test, expect } from '@playwright/test';
import { ConsolePage } from './page-objects/ConsolePage';
import { TEST_DATA } from './utils/test-helpers';

test.describe('Interactive Learn Mode (DE6)', () => {
  let console: ConsolePage;

  test.beforeEach(async ({ page }) => {
    console = new ConsolePage(page);
    await console.goto();
  });

  test('Learn mode is activated successfully', async () => {
    await console.executeCommand('learn');
    const mode = await console.getCurrentMode();
    expect(mode).toBe('learn');
    expect(await console.hasSuccessMessage()).toBeTruthy();
    await console.waitForOutput('tutorial');
  });

  test('Learn mode displays tutorial overview', async () => {
    await console.executeCommand('learn');
    await console.waitForOutput('Welcome');
    const output = await console.getOutputText();
    expect(output.toLowerCase()).toContain('mode');
    expect(output.toLowerCase()).toContain('shortcut');
  });

  test('Learn mode shows keyboard shortcuts help', async () => {
    await console.executeCommand('learn');
    await console.executeCommand('shortcuts');
    await console.waitForOutput('Ctrl+L');
    await console.waitForOutput('Ctrl+C');
    const output = await console.getOutputText();
    expect(output).toContain('Ctrl+L');
    expect(output).toContain('Ctrl+C');
  });

  test('Learn mode shows key bindings', async () => {
    await console.executeCommand('learn');
    await console.executeCommand('bindings');
    await console.waitForOutput('mode');
    const output = await console.getOutputText();
    expect(output.toLowerCase()).toContain('mode');
  });

  test('Learn mode shows tutorial content', async () => {
    await console.executeCommand('learn');
    await console.executeCommand('help');
    await console.waitForOutput('tutorial');
    const output = await console.getOutputText();
    expect(output.toLowerCase()).toContain('tutorial');
  });

  test('Learn mode returns to menu', async () => {
    await console.executeCommand('learn');
    await console.executeCommand('exit');
    const mode = await console.getCurrentMode();
    expect(mode).toBe('menu');
    expect(await console.hasSuccessMessage()).toBeTruthy();
  });

  test('Learn mode handles unknown command', async () => {
    await console.executeCommand('learn');
    await console.executeCommand('invalid_command');
    expect(await console.hasWarningMessage()).toBeTruthy();
    await console.waitForOutput('Unknown');
    const mode = await console.getCurrentMode();
    expect(mode).toBe('learn');
  });

  test('Learn mode provides consistent help', async () => {
    await console.executeCommand('learn');
    await console.executeCommand('help');
    const firstHelp = await console.getOutputText();
    await console.executeCommand('exit');
    await console.executeCommand('learn');
    await console.executeCommand('help');
    const secondHelp = await console.getOutputText();
    expect(firstHelp).toBe(secondHelp);
  });

  test('Learn mode tutorial is beginner friendly', async () => {
    await console.executeCommand('learn');
    const output = await console.getOutputText();
    // Should not contain overly technical jargon
    expect(output.toLowerCase()).not.toContain('implementation');
    expect(output.toLowerCase()).not.toContain('architecture');
    // Should contain helpful instructions
    expect(output.toLowerCase()).toContain('mode');
  });

  test('Learn mode does not affect other state', async () => {
    await console.executeCommand('learn');
    await console.executeCommand('help');
    await console.executeCommand('exit');
    const mode = await console.getCurrentMode();
    expect(mode).toBe('menu');
    // No settings should be modified
    expect(await console.isPromptVisible()).toBeTruthy();
  });
});
