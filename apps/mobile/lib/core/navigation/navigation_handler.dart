import 'package:flutter/material.dart';

/// Navigation abstraction interface untuk modular navigation
///
/// GUIDELINES compliance: Semua navigasi harus melalui interface ini,
/// tidak boleh ada direct context.go() di UI layer
abstract class NavigationHandler {
  // Core Navigation
  void navigateToHome();
  void navigateBack();
  void navigateToProfile();

  // Authentication Navigation
  void navigateToLogin();
  void navigateToSignIn(); // Alias for login
  void navigateToRegister();
  void navigateToSignUp(); // Alias for register
  void navigateToForgotPassword();

  // Onboarding Navigation
  void navigateToWelcome();

  // Profile & User Navigation
  void navigateToEditProfile();
  void navigateToUserProfile(String userId);
  void navigateToAddressPayment();
  void navigateToSecurity();

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

  // ============================================================================
  // PUBLIC PRODUCT CREATION - Use this for creating products/listings
  // This is the PRIMARY entry point for sellers to list items for sale
  // ============================================================================
  void navigateToCreateForSale();

  // ============================================================================
  // ⚠️ INTERNAL ONLY - AUCTION CREATION ⚠️
  // ============================================================================
  void navigateToCreateAuction();

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
  void navigateToPrivacySettings();
  void navigateToBlockedUsers();

  // Verification Navigation
  void navigateToKycVerification({String? userId});
  void navigateToBusinessDocuments();
  void navigateToSellerVerification();

  // Commerce Navigation
  void navigateToAuction(String auctionId);
  void navigateToCheckout();
  void
  navigateToSavedItems(); // Navigate to saved items (shortlist + auction watch) screen
  void navigateToOrders(); // Navigate to order list screen
  void navigateToOrderDetail(String orderId); // Navigate to specific order
  void navigateToOrderHistory(); // Alias for navigateToOrders

  // Payment Navigation
  void navigateToPayment(
    dynamic paymentRequest,
  ); // Navigate to payment screen with payment request

  // Seller Navigation
  void navigateToSellerDashboard();
  void navigateToSellerEarnings();
  void navigateToSellerForSales();
  void navigateToSellerRefundList();
  void navigateToSellerUpgrade();
  void navigateToExternalProductDetail(String productId);

  // Coin Navigation (loyalty points - NOT wallet/payment)
  void navigateToCoinBalance();
  void navigateToCoinHistory();

  // Modal Navigation
  void showBottomSheet<T>(Widget Function(BuildContext) builder);
  void showModalDialog<T>(Widget Function(BuildContext) builder);
  void showSnackBar(String message, {bool isError = false});
}
