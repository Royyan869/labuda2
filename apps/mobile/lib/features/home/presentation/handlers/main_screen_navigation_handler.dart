/// Main Screen Navigation Handler
///
/// Navigation handler khusus untuk Main Screen dan drawer actions
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

/// Navigation handler untuk Main Screen
///
/// Menangani navigasi terkait dengan main screen seperti:
/// - Tab switching
/// - Bottom navigation
/// - Drawer actions (sign in/out, settings, profile, etc.)
class MainScreenNavigationHandler {
  final WidgetRef ref;
  final BuildContext context;
  final NavigationHandler _appRouter;

  MainScreenNavigationHandler({
    required this.ref,
    required this.context,
    NavigationHandler? appRouter,
  }) : _appRouter = appRouter ?? AppRouter();

  /// Navigate to home tab
  void navigateToHome() {
    _appRouter.navigateToHome();
  }

  /// Navigate to explore/search tab
  void navigateToExplore() {
    _appRouter.navigateToSearch();
  }

  /// Navigate to create content
  void navigateToCreate() => _appRouter.navigateToCreateContent();

  /// Navigate to notifications
  VoidCallback get navigateToNotifications => () {
    _appRouter.navigateToNotifications();
  };

  /// Navigate to messages/chat
  VoidCallback get navigateToMessages => () {
    _appRouter.navigateToChat();
  };

  /// Handle sign in
  VoidCallback get handleSignIn => () {
    _appRouter.navigateToSignIn();
  };

  /// Handle sign up
  VoidCallback get handleSignUp => () {
    _appRouter.navigateToSignUp();
  };

  /// Handle sign out
  VoidCallback get handleSignOut => () {
    // Sign out logic
    final authController = ref.read(authControllerProvider.notifier);
    authController.signOut();
  };

  /// Handle settings navigation
  void handleSettings({bool closeDrawer = true}) {
    _appRouter.navigateToSettings();
  }

  /// Handle profile navigation
  VoidCallback get handleProfile => () {
    final authState = ref.read(authControllerProvider);
    if (authState is AuthStateAuthenticated) {
      _appRouter.navigateToProfile();
    }
  };

  /// Handle coming soon features
  void Function(BuildContext, String) get handleComingSoon => (ctx, feature) {
    Navigator.pop(ctx); // Close drawer
    AppSnackBar.showInfo(ctx, '$feature coming soon');
  };

  /// Navigate to profile
  void navigateToProfile() {
    _appRouter.navigateToProfile();
  }

  /// Navigate to chat
  void navigateToChat() {
    _appRouter.navigateToChat();
  }

  /// Navigate to specific content detail
  void navigateToContentDetail(String contentId) {
    _appRouter.navigateToContentDetail(contentId);
  }

  /// Navigate to For Sale detail (PUBLIC - use this for product browsing)
  void navigateToForSaleDetail(String fixedPriceSaleId) {
    _appRouter.navigateToForSaleDetail(fixedPriceSaleId);
  }

  /// Navigate to auction detail
  void navigateToAuction(String auctionId) {
    _appRouter.navigateToAuction(auctionId);
  }

  /// Navigate back
  void navigateBack() {
    _appRouter.navigateBack();
  }
}
