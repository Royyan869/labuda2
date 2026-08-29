import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Implementasi konkret NavigationHandler menggunakan AppRouter
///
/// Menggunakan AppRouter langsung untuk menghindari context timing issues
class AppNavigationHandler implements NavigationHandler {
  final AppRouter _appRouter;

  AppNavigationHandler() : _appRouter = AppRouter();

  // Core Navigation
  @override
  void navigateToHome() {
    _appRouter.navigateToHome();
  }

  @override
  void navigateBack() {
    _appRouter.navigateBack();
  }

  @override
  void navigateToProfile() {
    _appRouter.navigateToProfile();
  }

  // Authentication Navigation
  @override
  void navigateToLogin() {
    _appRouter.navigateToSignIn();
  }

  @override
  void navigateToSignIn() {
    _appRouter.navigateToSignIn();
  }

  @override
  void navigateToRegister() {
    _appRouter.navigateToSignUp();
  }

  @override
  void navigateToSignUp() {
    _appRouter.navigateToSignUp();
  }

  @override
  void navigateToForgotPassword() {
    _appRouter.navigateToForgotPassword();
  }

  // Onboarding Navigation
  @override
  void navigateToWelcome() {
    _appRouter.navigateToWelcome();
  }

  // All other navigation methods delegate to AppRouter
  @override
  void navigateToEditProfile() => _appRouter.navigateToEditProfile();

  @override
  void navigateToUserProfile(String userId) =>
      _appRouter.navigateToUserProfile(userId);

  @override
  void navigateToAddressPayment() => _appRouter.navigateToAddressPayment();

  @override
  void navigateToSecurity() => _appRouter.navigateToSecurity();

  @override
  void navigateToContentDetail(String contentId) =>
      _appRouter.navigateToContentDetail(contentId);

  @override
  void navigateToForSaleDetail(String forSaleId) =>
      _appRouter.navigateToForSaleDetail(forSaleId);

  // Creation Navigation - removed hub/form, using dedicated screens only

  // Dedicated creation navigation implementations
  @override
  void navigateToCreateContent() => _appRouter.navigateToCreateContent();

  @override
  void navigateToCreateForSale() => _appRouter.navigateToCreateForSale();

  @override
  void navigateToCreateAuction() => _appRouter.navigateToCreateAuction();

  // Chat Navigation
  @override
  void navigateToChat() => _appRouter.navigateToChat();

  @override
  void navigateToChatConversation(String conversationId) =>
      _appRouter.navigateToChatConversation(conversationId);

  @override
  void navigateToNotifications() => _appRouter.navigateToNotifications();

  @override
  void navigateToSearch() => _appRouter.navigateToSearch();

  @override
  void navigateToSearchResults(String query, {String? type}) =>
      _appRouter.navigateToSearchResults(query, type: type);

  @override
  void navigateToSettings() => _appRouter.navigateToSettings();

  @override
  void navigateToNotificationSettings() =>
      _appRouter.navigateToNotificationSettings();

  @override
  void navigateToPrivacySettings() => _appRouter.navigateToPrivacySettings();

  @override
  void navigateToBlockedUsers() => _appRouter.navigateToBlockedUsers();

  // Verification Navigation
  @override
  void navigateToKycVerification({String? userId}) =>
      _appRouter.navigateToKycVerification(userId: userId);

  @override
  void navigateToBusinessDocuments() =>
      _appRouter.navigateToBusinessDocuments();

  @override
  void navigateToSellerVerification() =>
      _appRouter.navigateToSellerVerification();

  @override
  void navigateToAuction(String auctionId) =>
      _appRouter.navigateToAuction(auctionId);

  @override
  void navigateToCheckout() => _appRouter.navigateToCheckout();

  @override
  void navigateToSavedItems() => _appRouter.navigateToSavedItems();

  @override
  void navigateToOrders() => _appRouter.navigateToOrders();

  @override
  void navigateToOrderDetail(String orderId) =>
      _appRouter.navigateToOrderDetail(orderId);

  @override
  void navigateToOrderHistory() => _appRouter.navigateToOrderHistory();

  @override
  void navigateToPayment(dynamic paymentRequest) =>
      _appRouter.navigateToPayment(paymentRequest);

  @override
  void navigateToSellerDashboard() => _appRouter.navigateToSellerDashboard();

  @override
  void navigateToSellerEarnings() => _appRouter.navigateToSellerEarnings();

  @override
  void navigateToSellerForSales() => _appRouter.navigateToSellerForSales();

  @override
  void navigateToSellerRefundList() => _appRouter.navigateToSellerRefundList();

  @override
  void navigateToSellerUpgrade() => _appRouter.navigateToSellerUpgrade();

  @override
  void navigateToExternalProductDetail(String productId) =>
      _appRouter.navigateToExternalProductDetail(productId);

  @override
  void navigateToCoinBalance() => _appRouter.navigateToCoinBalance();

  @override
  void navigateToCoinHistory() => _appRouter.navigateToCoinHistory();

  // Modal Navigation - These still need context, will be addressed later if needed
  @override
  void showBottomSheet<T>(Widget Function(BuildContext) builder) {
    // TODO: Implement modal navigation through AppRouter if needed
  }

  @override
  void showModalDialog<T>(Widget Function(BuildContext) builder) {
    // TODO: Implement modal navigation through AppRouter if needed
  }

  @override
  void showSnackBar(String message, {bool isError = false}) {
    // TODO: Implement snackbar through AppRouter if needed
  }
}
