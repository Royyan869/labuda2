/// Navigation abstraction interface untuk modular navigation
///
/// CANONICAL NAVIGATION POLICY:
/// - Widget/UI context: navigate directly through the canonical GoRouter
///   context (`context.go`, `context.push`, `context.pushNamed`, `context.pop`).
/// - Non-widget / global / service context (no BuildContext): use this
///   [NavigationHandler] (backed by [AppRouter] over the global navigator key).
///
/// This interface forwards to the ONE GoRouter — it is not a second router and
/// holds no route registry of its own.
abstract class NavigationHandler {
  // Core Navigation
  void navigateToHome();
  void navigateToProfile();

  // Authentication Navigation
  void navigateToSignIn();
  void navigateToSignUp();
  void navigateToForgotPassword();

  // Onboarding Navigation
  void navigateToWelcome();

  // Profile & User Navigation
  void navigateToUserProfile(String userId);

  // Content Navigation
  void navigateToContentDetail(
    String contentId,
  ); // Works for both post & request types
  void navigateToForSaleDetail(
    String forSaleId,
  ); // PUBLIC: Use this for product detail pages

  // Creation Navigation - removed hub/form, using dedicated screens only

  // Dedicated creation navigation
  void navigateToCreateContent();

  // Chat Navigation
  void navigateToChat();
  void navigateToChatConversation(String conversationId);

  // Notification Navigation
  void navigateToNotifications();

  // Search Navigation
  void navigateToSearch();
  void navigateToSearchResults(String query, {String? type});

  // Settings Navigation
  void navigateToSettings();
  void navigateToNotificationSettings();

  // Verification Navigation
  void navigateToSellerVerification();

  // Commerce Navigation
  void navigateToAuction(String auctionId);
  void
  navigateToSavedItems(); // Navigate to saved items (for_sale + auction) screen
  void navigateToMyBids();
  void navigateToOrders(); // Navigate to order list screen
  void navigateToOrderDetail(String orderId); // Navigate to specific order

  // Seller Navigation
  void navigateToSellerDashboard();
  void navigateToSellerEarnings();
  void navigateToSellerUpgrade();
  void navigateToSellerRenewal();
  void navigateToExternalProductDetail(String productId);

  // Coin Navigation (loyalty points - NOT wallet/payment)
  void navigateToCoinHistory();
}
