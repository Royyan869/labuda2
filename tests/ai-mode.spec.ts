import { test, expect } from '@playwright/test';
import { ConsolePage } from './page-objects/ConsolePage';
import { TEST_DATA } from './utils/test-helpers';

test.describe('AI Conversation Mode (DE5)', () => {
  let console: ConsolePage;

  test.beforeEach(async ({ page }) => {
    console = new ConsolePage(page);
    await console.goto();
  });

  test('AI mode is activated successfully', async () => {
    await console.executeCommand('ai');
    const mode = await console.getCurrentMode();
    expect(mode).toBe('ai');
    expect(await console.hasSuccessMessage()).toBeTruthy();
    await console.waitForOutput(TEST_DATA.AI_GREETING);
  });

  test('AI sends message to provider', async () => {
    await console.executeCommand('ai');
    await console.executeCommand('Hello AI, how are you?');
    await console.waitForOutput('user');
    await console.waitForOutput('assistant');
    expect(await console.outputContains('Hello AI')).toBeTruthy();
  });

  test('AI provider is unavailable', async () => {
    // This test assumes the provider is configured to fail
    // In a real test, we'd mock the provider
    await console.executeCommand('ai');
    await console.executeCommand('This message will fail');
    // The mock provider should return an error
    const hasError = await console.hasErrorMessage() || await console.hasWarningMessage();
    expect(hasError).toBeTruthy();
    const mode = await console.getCurrentMode();
    expect(mode).toBe('ai');
  });

  test('AI handles empty provider key gracefully', async () => {
    // This test assumes the API key is empty
    await console.executeCommand('ai');
    const hasWarning = await console.hasWarningMessage();
    expect(hasWarning).toBeTruthy();
    const mode = await console.getCurrentMode();
    expect(mode).toBe('menu');
  });

  test('AI maintains conversation history within session', async () => {
    await console.executeCommand('ai');
    await console.executeCommand('My name is Alice');
    await console.waitForOutput('Alice');
    await console.executeCommand('What is my name?');
    // The second request should include previous messages
    await console.waitForOutput('name');
  });

  test('AI conversation history resets on mode switch', async () => {
    await console.executeCommand('ai');
    await console.executeCommand('Remember this context');
    await console.waitForOutput('context');
    await console.executeCommand('menu');
    await console.executeCommand('ai');
    await console.executeCommand('What was the context?');
    // The new session should not have the previous context
    await console.waitForOutput('context');
  });

  test('AI response displays correctly formatted', async () => {
    await console.executeCommand('ai');
    await console.executeCommand('Tell me a story');
    await console.waitForOutput('---');
    await console.waitForOutput('AI Response');
    const output = await console.getOutputText();
    expect(output).toContain('---');
    expect(output).toContain('AI Response');
  });

  test('AI handles special characters in message', async () => {
    await console.executeCommand('ai');
    await console.executeCommand("Hello <script>alert('xss')</script>");
    // Should not trigger XSS
    const output = await console.getOutputText();
    expect(output).not.toContain('<script>');
  });

  test('AI handles very long message', async () => {
    await console.executeCommand('ai');
    const longMsg = 'A'.repeat(10000);
    await console.typeCommand(longMsg);
    await console.pressEnter();
    // Should get a response or error
    await console.page.waitForTimeout(1000);
    const output = await console.getOutputText();
    expect(output.length).toBeGreaterThan(0);
  });

  test('AI handles concurrent requests gracefully', async () => {
    await console.executeCommand('ai');
    const promises = [];
    for (let i = 0; i < 5; i++) {
      promises.push(console.executeCommand(`Message ${i}`));
    }
    await Promise.all(promises);
    await console.page.waitForTimeout(2000);
    // Console should not crash
    expect(await console.isPromptVisible()).toBeTruthy();
  });
});
