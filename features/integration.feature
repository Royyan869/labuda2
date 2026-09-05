Feature: Integration - All Demo Features
  As a stakeholder evaluating the Unhinged Console MVP
  I want to verify all demo features work together seamlessly
  So that I can confirm the product is ready for showcase

  Background:
    Given the console application is running
    And the console has loaded with the ASCII art banner

  Scenario: Full user journey through all modes
    When I type "console" to enter console mode
    And I type "status" to check system status
    And I type "exit" to return to menu
    When I type "learn" to enter learn mode
    And I type "help" to view tutorial
    And I type "shortcuts" to view shortcuts
    And I type "exit" to return to menu
    When I type "ai" to enter AI mode
    And I type "Hello from integration test"
    And I type "exit" to return to menu
    Then all modes should have responded correctly
    And no errors should have occurred

  Scenario: Mode switching preserves console stability
    When I rapidly switch between all modes 5 times
    Then the console should remain responsive
    And all mode switches should succeed
    And the final mode should be "menu"

  Scenario: Error recovery across modes
    When I type "nonexistent_mode"
    And I type "ai" to enter AI mode
    Given the AI provider is configured to fail
    And I type "This will fail"
    And I type "exit" to return to menu
    When I type "console" to enter console mode
    And I type "status"
    Then the system should be stable
    And all errors should be displayed correctly

  Scenario: GUI displays all message types correctly
    Given the console is ready
    When I trigger a green success message
    And I trigger a blue info message
    And I trigger an orange warning message
    And I trigger a red error message
    Then all messages should be displayed in their correct colors
    And the messages should not overlap
    And the prompt should remain visible

  Scenario: Console maintains state across interactions
    When I type "ai" to enter AI mode
    And I type "Set some context"
    And I type "exit" to return to menu
    And I type "console" to enter console mode
    And I type "status"
    Then the status should reflect current state
    And the mode history should be tracked

  Scenario: All demo scenarios can be executed in sequence
    When I execute all DE3 console scenarios
    And I execute all DE4 GUI scenarios
    And I execute all DE5 AI scenarios
    And I execute all DE6 learn scenarios
    Then all scenarios should pass
    And no system crashes should occur

  Scenario: Console startup performance
    When I measure console startup time
    Then the startup should complete within 3 seconds
    And the banner should be fully rendered

  Scenario: Console input responsiveness
    Given the console is ready
    When I measure input response time for 10 commands
    Then each command should respond within 100ms
    And the console should feel responsive
