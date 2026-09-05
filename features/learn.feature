Feature: Interactive Learn Mode (DE6)
  As a new user of Unhinged Console
  I want an interactive tutorial to learn about the system
  So that I can understand the available modes and how to use them

  Background:
    Given the console has loaded with the ASCII art banner

  Scenario: Learn mode is activated successfully
    When I type "learn" in the console
    Then the mode should switch to "learn"
    And a green status message should be displayed
    And the welcome tutorial message should be displayed

  Scenario: Learn mode displays tutorial overview
    When I type "learn" in the console
    Then the tutorial overview should be shown
    And it should mention the available modes
    And it should mention keyboard shortcuts
    And it should mention key bindings

  Scenario: Learn mode shows keyboard shortcuts help
    When I type "learn" in the console
    And I type "shortcuts"
    Then the keyboard shortcuts should be displayed
    And Ctrl+L should be mentioned for clear screen
    And Ctrl+C should be mentioned for graceful exit

  Scenario: Learn mode shows key bindings
    When I type "learn" in the console
    And I type "bindings"
    Then the key bindings should be displayed
    And mode switching bindings should be listed

  Scenario: Learn mode shows tutorial content
    When I type "learn" in the console
    And I type "help"
    Then the tutorial content should be displayed
    And the tutorial should include usage instructions

  Scenario: Learn mode returns to menu
    When I type "learn" in the console
    And I type "exit"
    Then the mode should switch to "menu"
    And a green success message should be displayed

  Scenario: Learn mode handles unknown command
    When I type "learn" in the console
    And I type "invalid_command"
    Then an orange warning message should be displayed
    And the warning should indicate unknown tutorial command
    And the mode should remain "learn"

  Scenario: Learn mode provides consistent help
    When I type "learn" in the console
    And I type "help"
    And I type "exit"
    And I type "learn" to re-enter
    And I type "help" again
    Then both help responses should be identical

  Scenario: Learn mode tutorial is beginner friendly
    When I type "learn" in the console
    Then the tutorial language should be simple and clear
    And the tutorial should avoid technical jargon
    And the tutorial should provide actionable instructions

  Scenario: Learn mode does not affect other state
    When I type "learn" in the console
    And I type "help"
    And I type "exit"
    Then the console state should be unchanged
    And no settings should be modified
    And the mode should be "menu"
