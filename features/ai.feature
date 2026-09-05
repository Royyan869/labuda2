Feature: AI Conversation Mode (DE5)
  As a user working with Unhinged Console
  I want to chat with an AI model in a game show environment
  So that I can get intelligent responses while being entertained

  Background:
    Given the console has loaded with the ASCII art banner
    And the AI provider is configured

  Scenario: AI mode is activated successfully
    When I type "ai" in the console
    Then the mode should switch to "ai"
    And a green status message should be displayed
    And a mock AI greeting response should appear

  Scenario: AI sends message to provider
    When I type "ai" to enter AI mode
    And I type "Hello AI, how are you?"
    Then the AI provider should receive a chat request
    And a green AI response should be displayed
    And the response should contain user message echo
    And the response should contain a model response

  Scenario: AI provider is unavailable
    Given the AI provider is configured to fail
    When I type "ai" to enter AI mode
    And I type "This message will fail"
    Then an orange error message should be displayed
    And the error should indicate provider failure
    And the mode should remain "ai"

  Scenario: AI handles empty provider key gracefully
    Given the AI provider API key is empty
    When I type "ai" in the console
    Then a yellow warning message should be displayed
    And the warning should indicate missing API key
    And the mode should remain "menu"

  Scenario: AI provider returns HTTP error
    Given the AI provider is configured to return status 401
    When I type "ai" to enter AI mode
    And I type "Test unauthorized"
    Then an orange error message should be displayed
    And the error should indicate HTTP error 401

  Scenario: AI provider returns HTTP 429 rate limit
    Given the AI provider is configured to return status 429
    When I type "ai" to enter AI mode
    And I type "Test rate limit"
    Then an orange error message should be displayed
    And the error should indicate rate limit

  Scenario: AI provider returns HTTP 500 server error
    Given the AI provider is configured to return status 500
    When I type "ai" to enter AI mode
    And I type "Test server error"
    Then an orange error message should be displayed
    And the error should indicate server error

  Scenario: AI provider has network timeout
    Given the AI provider is configured to timeout
    When I type "ai" to enter AI mode
    And I type "Test timeout"
    Then an orange error message should be displayed
    And the error should indicate timeout or connection failure

  Scenario: AI maintains conversation history within session
    When I type "ai" to enter AI mode
    And I type "My name is Alice"
    And I type "What is my name?"
    Then the second AI request should include previous messages
    And the conversation history should contain both exchanges

  Scenario: AI conversation history resets on mode switch
    When I type "ai" to enter AI mode
    And I type "Remember this context"
    When I type "menu" to exit AI mode
    And I type "ai" to re-enter AI mode
    And I type "What was the context?"
    Then the AI request should not include previous session messages

  Scenario: AI response displays correctly formatted
    When I type "ai" to enter AI mode
    And I type "Tell me a story"
    Then the response should use the AI display format
    And the response should show a separator line
    And the response should show a timestamp
    And the response should show the model name

  Scenario: AI handles special characters in message
    When I type "ai" to enter AI mode
    And I type "Hello <script>alert('xss')</script>"
    Then the AI provider should receive the message
    And no XSS vulnerability should be triggered

  Scenario: AI handles very long message
    When I type "ai" to enter AI mode
    And I type a message with 10000 characters
    Then the AI provider should receive the message
    And a response or error should be displayed

  Scenario: AI handles concurrent requests gracefully
    When I type "ai" to enter AI mode
    And I rapidly send 5 messages
    Then each message should get a response or error
    And the console should not crash
