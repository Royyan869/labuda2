import { type Page, type Locator, expect } from '@playwright/test';

/**
 * Page Object for the Unhinged Console
 * Encapsulates all interactions with the TUI console interface
 */
export class ConsolePage {
  readonly page: Page;
  readonly consoleInput: Locator;
  readonly consoleOutput: Locator;
  readonly banner: Locator;
  readonly prompt: Locator;

  constructor(page: Page) {
    this.page = page;
    this.consoleInput = page.locator('[data-testid="console-input"]');
    this.consoleOutput = page.locator('[data-testid="console-output"]');
    this.banner = page.locator('[data-testid="ascii-banner"]');
    this.prompt = page.locator('[data-testid="console-prompt"]');
  }

  /**
   * Navigate to the console application
   */
  async goto() {
    await this.page.goto('/');
    await this.waitForReady();
  }

  /**
   * Wait for the console to be fully loaded and ready
   */
  async waitForReady() {
    await this.consoleInput.waitFor({ state: 'visible', timeout: 10000 });
    await this.banner.waitFor({ state: 'visible', timeout: 5000 });
  }

  /**
   * Type a command into the console
   */
  async typeCommand(command: string) {
    await this.consoleInput.fill(command);
  }

  /**
   * Press Enter to execute a command
   */
  async pressEnter() {
    await this.consoleInput.press('Enter');
  }

  /**
   * Type a command and execute it
   */
  async executeCommand(command: string) {
    await this.typeCommand(command);
    await this.pressEnter();
  }

  /**
   * Get all visible text from the console output
   */
  async getOutputText(): Promise<string> {
    return await this.consoleOutput.innerText();
  }

  /**
   * Check if a specific text appears in the output
   */
  async outputContains(text: string): Promise<boolean> {
    const output = await this.getOutputText();
    return output.includes(text);
  }

  /**
   * Wait for specific text to appear in output
   */
  async waitForOutput(text: string, timeout = 5000) {
    await this.page.waitForFunction(
      (searchText) => {
        const output = document.querySelector('[data-testid="console-output"]');
        return output?.textContent?.includes(searchText) ?? false;
      },
      text,
      { timeout }
    );
  }

  /**
   * Check if a green success message is displayed
   */
  async hasSuccessMessage(): Promise<boolean> {
    return await this.page.locator('.text-green-500, .text-green-400').count() > 0;
  }

  /**
   * Check if a red error message is displayed
   */
  async hasErrorMessage(): Promise<boolean> {
    return await this.page.locator('.text-red-500, .text-red-400').count() > 0;
  }

  /**
   * Check if a blue info message is displayed
   */
  async hasInfoMessage(): Promise<boolean> {
    return await this.page.locator('.text-blue-500, .text-blue-400').count() > 0;
  }

  /**
   * Check if a yellow/orange warning message is displayed
   */
  async hasWarningMessage(): Promise<boolean> {
    return await this.page.locator('.text-yellow-500, .text-orange-500, .text-amber-500').count() > 0;
  }

  /**
   * Get the current mode from status
   */
  async getCurrentMode(): Promise<string> {
    const output = await this.getOutputText();
    const modeMatch = output.match(/Mode:\s*(\w+)/i);
    return modeMatch?.[1]?.toLowerCase() ?? 'menu';
  }

  /**
   * Press Ctrl+C for graceful exit
   */
  async pressCtrlC() {
    await this.consoleInput.press('Control+c');
  }

  /**
   * Press Ctrl+L for clear screen
   */
  async pressCtrlL() {
    await this.consoleInput.press('Control+l');
  }

  /**
   * Check if the banner is visible
   */
  async isBannerVisible(): Promise<boolean> {
    return await this.banner.isVisible();
  }

  /**
   * Get the version number from the banner
   */
  async getVersion(): Promise<string | null> {
    const bannerText = await this.banner.innerText();
    const versionMatch = bannerText.match(/v(\d+\.\d+\.\d+)/);
    return versionMatch?.[1] ?? null;
  }

  /**
   * Measure the time it takes for a command to respond
   */
  async measureCommandResponseTime(command: string): Promise<number> {
    const start = Date.now();
    await this.executeCommand(command);
    // Wait for output to appear
    await this.page.waitForTimeout(100);
    return Date.now() - start;
  }

  /**
   * Rapidly execute multiple commands
   */
  async rapidExecute(commands: string[]) {
    for (const cmd of commands) {
      await this.executeCommand(cmd);
    }
  }

  /**
   * Check if the prompt is visible and positioned correctly
   */
  async isPromptVisible(): Promise<boolean> {
    return await this.prompt.isVisible();
  }

  /**
   * Get the input value
   */
  async getInputValue(): Promise<string> {
    return await this.consoleInput.inputValue();
  }

  /**
   * Clear the input
   */
  async clearInput() {
    await this.consoleInput.fill('');
  }
}
