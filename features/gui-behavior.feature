Feature: GUI Console Behavior (DE4)
  As a user interacting with the Unhinged Console GUI
  I want proper display, input handling, and error display
  So that I can use the console effectively

  Background:
    Given the console application is running

  Scenario: Console displays welcome banner on startup
    When the application starts
    Then the ASCII art banner should be displayed
    And the banner should contain "UNHINGED CONSOLE"
    And the version number should be shown
    And the "Ready for Chaos" message should appear

  Scenario: Console prompt is visible and positioned correctly
    Given the console has loaded
    Then the prompt should be visible
    And the prompt should be at the bottom of the screen

  Scenario: Console accepts text input
    Given the console is ready
    When I type "hello world"
    Then the text should appear at the cursor position

  Scenario: Console processes Enter key
    Given the console is ready
    When I type "menu"
    And I press Enter
    Then the command should be processed
    And a new prompt should appear

  Scenario: Console handles Ctrl+C gracefully
    Given the console is ready
    When I press Ctrl+C
    Then a graceful shutdown message should appear
    And the application should exit cleanly

  Scenario: Console handles Ctrl+L for clear screen
    Given the console is ready
    When I press Ctrl+L
    Then the screen should be cleared
    And the banner should be redisplayed

  Scenario: Console displays error messages correctly
    Given the console is ready
    When I type "invalid_mode"
    And I press Enter
    Then an orange warning message should be displayed
    And the warning should indicate unknown command

  Scenario: Console displays success messages in green
    Given the console is ready
    When I type "ai"
    And I press Enter
    Then a green status message should be displayed

  Scenario: Console displays info messages in blue
    Given the console is ready
    When I type "help"
    And I press Enter
    Then a blue info message should be displayed

  Scenario: Console displays warning messages in orange
    Given the console is ready
    When I type an empty command
    And I press Enter
    Then an orange warning message should be displayed

  Scenario: Console handles window resize
    Given the console is ready
    When the window is resized
    Then the layout should adapt to new dimensions
    And the prompt should remain visible

  Scenario: Console input is cleared after command execution
    Given the console is ready
    When I type "test_input"
    And I press Enter
    Then the input line should be cleared

  Scenario: Console handles rapid input
    Given the console is ready
    When I rapidly type 100 characters
    Then the console should not crash
    And the text should be displayed correctly

  Scenario: Console handles special characters
    Given the console is ready
    When I type "!@#$%^&*()_+-={}[]|;':\",./<>?"
    And I press Enter
    Then the characters should be processed without error

  Scenario: Console handles empty input
    Given the console is ready
    When I press Enter without typing
    Then an orange warning message should be displayed

  Scenario: Console preserves input history context
    Given the console is ready
    When I type "mode1"
    And I press Enter
    And I type "mode2"
    And I press Enter
    Then both commands should be processed independently

  Scenario: Console handles focus and blur events
    Given the console is ready
    When the console loses focus
    Then the cursor should blink appropriately
    When the console regains focus
    Then the input should resume normally

  Scenario: Console displays colored text correctly
    Given the console is ready
    When I trigger a green message
    Then the text should be rendered in green
    When I trigger a red message
    Then the text should be rendered in red
    When I trigger a blue message
    Then the text should be rendered in blue

  Scenario: Console handles multiple rapid commands
    Given the console is ready
    When I rapidly execute 10 different commands
    Then all commands should be processed
    And the console should remain responsive
