Feature: Console System Mode (DE3)
  As a user working with Unhinged Console
  I want to use the console mode for system administration
  So that I can inspect status, clear screen, and reset state

  Background:
    Given the console has loaded with the ASCII art banner

  Scenario: Console mode is activated successfully
    When I type "console" in the console
    Then the mode should switch to "console"
    And a green status message should be displayed
    And the banner should be displayed

  Scenario: Console status command
    When I type "console" in the console
    And I type "status"
    Then the system status should be displayed
    And the status should include the active mode
    And the status should include uptime

  Scenario: Console clear command
    When I type "console" in the console
    And I type "clear"
    Then the screen should be cleared
    And the banner should be redisplayed

  Scenario: Console reset command
    When I type "console" in the console
    And I type "reset"
    Then a yellow warning message should be displayed
    And the warning should indicate reset confirmation
    When I type "y" to confirm
    Then a green success message should be displayed
    And the console should be in menu mode

  Scenario: Console reset cancelled
    When I type "console" in the console
    And I type "reset"
    And I type "n" to cancel
    Then a blue info message should be displayed
    And the info should indicate reset cancelled
    And the console should remain in console mode

  Scenario: Console returns unknown command
    When I type "console" in the console
    And I type "unknown_command"
    Then an orange warning message should be displayed
    And the warning should indicate unknown command

  Scenario: Console shows banner on entry
    When I type "console" in the console
    Then the ASCII art banner should be displayed
    And the version number should be shown

  Scenario: Console handles concurrent commands
    When I type "console" in the console
    And I rapidly execute 5 status commands
    Then each command should complete successfully
    And the console should not crash

  Scenario: Console status shows all system components
    When I type "console" in the console
    And I type "status"
    Then the status should show GUI subsystem
    And the status should show AI provider
    And the status should show test subsystem

  Scenario: Console reset clears all state
    Given I am in AI mode with conversation history
    When I type "console" in the console
    And I type "reset"
    And I type "y" to confirm
    Then the mode should be reset to "menu"
    And the AI conversation history should be cleared

  Scenario: Console displays correct version
    When I type "console" in the console
    And I type "status"
    Then the version should match the current version
