import { type Page } from '@playwright/test';
import { ConsolePage } from '../page-objects/ConsolePage';

/**
 * Common test data for console tests
 */
export const TEST_DATA = {
  VERSION: '0.1.0',
  BANNER_TEXT: 'UNHINGED CONSOLE',
  READY_MESSAGE: 'Ready for Chaos',
  AI_GREETING: 'Game show AI',
  CONSOLE_MODE: 'console',
  LEARN_MODE: 'learn',
  AI_MODE: 'ai',
  MENU_MODE: 'menu',
} as const;

/**
 * Color classes used in the console
 */
export const COLORS = {
  SUCCESS: ['text-green-500', 'text-green-400'],
  ERROR: ['text-red-500', 'text-red-400'],
  INFO: ['text-blue-500', 'text-blue-400'],
  WARNING: ['text-yellow-500', 'text-orange-500', 'text-amber-500'],
} as const;

/**
 * Wait for a specific color class to appear in the output
 */
export async function waitForColor(
  page: Page,
  colorClasses: readonly string[],
  timeout = 5000
): Promise<boolean> {
  const selector = colorClasses.map(c => `.${c}`).join(', ');
  try {
    await page.waitForSelector(selector, { timeout });
    return true;
  } catch {
    return false;
  }
}

/**
 * Get all elements with specific color classes
 */
export function getColoredElements(page: Page, colorClasses: readonly string[]) {
  const selector = colorClasses.map(c => `.${c}`).join(', ');
  return page.locator(selector);
}

/**
 * Generate a random string of specified length
 */
export function randomString(length: number): string {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*()';
  let result = '';
  for (let i = 0; i < length; i++) {
    result += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  return result;
}

/**
 * Generate a very long message for stress testing
 */
export function longMessage(length: number = 10000): string {
  return 'A'.repeat(length);
}

/**
 * Expected keyboard shortcuts
 */
export const KEYBOARD_SHORTCUTS = {
  CLEAR_SCREEN: 'Control+l',
  GRACEFUL_EXIT: 'Control+c',
} as const;

/**
 * Expected mode commands
 */
export const MODE_COMMANDS = {
  CONSOLE: 'console',
  LEARN: 'learn',
  AI: 'ai',
  MENU: 'menu',
  EXIT: 'exit',
} as const;

/**
 * Expected console commands
 */
export const CONSOLE_COMMANDS = {
  STATUS: 'status',
  CLEAR: 'clear',
  RESET: 'reset',
} as const;

/**
 * Expected learn commands
 */
export const LEARN_COMMANDS = {
  HELP: 'help',
  SHORTCUTS: 'shortcuts',
  BINDINGS: 'bindings',
  EXIT: 'exit',
} as const;

/**
 * Create a ConsolePage instance and navigate to the app
 */
export async function setupConsole(page: Page): Promise<ConsolePage> {
  const console = new ConsolePage(page);
  await console.goto();
  return console;
}

/**
 * Verify that the console is in a stable state
 */
export async function verifyConsoleStable(page: Page): Promise<boolean> {
  const console = new ConsolePage(page);
  const isInputVisible = await console.consoleInput.isVisible();
  const isBannerVisible = await console.isBannerVisible();
  return isInputVisible && isBannerVisible;
}
