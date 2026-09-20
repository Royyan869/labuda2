import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/api/api_error_codes.dart' as codes;
import 'package:labuda/core/navigation/navigation_handler.dart';
import 'package:labuda/core/navigation/navigation_provider.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';

/// Canonical presentation authority for backend commerce restriction errors.
///
/// This is the ONLY place in the mobile app that decides how to show
/// restriction errors (`COMMERCE_RESTRICTED`, `MARKET_AUTHORITY_REQUIRED`)
/// to the user. All commerce screens MUST call [CommerceRestrictionPresenter.handle]
/// instead of implementing their own restriction error UI.
///
/// Two canonical restriction semantics, one presenter:
///
/// - `COMMERCE_RESTRICTED` — backend governance restriction. The user can
///   still browse; only commerce actions are blocked. Resolution is manual
///   (contact support), so the UX is a non-navigating error snackbar.
///
/// - `MARKET_AUTHORITY_REQUIRED` — HTTP 403 from the active seller
///   subscription gate. NOT a governance restriction and NOT a ban: the
///   resolution is a payment-only self-service renewal, so the UX navigates
///   the seller to the canonical renewal screen via
///   [NavigationHandler.navigateToSellerRenewal] → `RoutePaths.sellerRenewal`.
///
/// Generic `FORBIDDEN` is neither of the above and MUST fall through as an
/// unknown error — it is never treated as a market-authority denial.
class CommerceRestrictionPresenter {
  const CommerceRestrictionPresenter._();

  /// Returns `true` if [errorCode] is the canonical commerce restriction code.
  static bool isCommerceRestricted(String? errorCode) =>
      errorCode == codes.commerceRestricted;

  /// Returns `true` if [errorCode] is the canonical market authority code.
  static bool isMarketAuthorityRequired(String? errorCode) =>
      errorCode == codes.marketAuthorityRequired;

  /// Returns `true` if [errorCode] is a restriction code this presenter owns.
  ///
  /// Use this (instead of the per-code predicates) at call sites that want a
  /// single canonical branch for any restriction-family error.
  static bool isRestrictionPresented(String? errorCode) =>
      isCommerceRestricted(errorCode) || isMarketAuthorityRequired(errorCode);

  /// Canonical dispatcher for any restriction-family error code.
  ///
  /// Call this from any commerce screen that receives a restriction error
  /// from the backend. After calling this, the error is consumed — do NOT
  /// additionally show a generic error snackbar.
  ///
  /// Returns `true` when the code was presented (consumed) by this presenter;
  /// `false` when the code is outside the restriction family and the caller
  /// must handle it through its existing generic error path.
  ///
  /// Canonical behavior per code:
  /// - `COMMERCE_RESTRICTED` → error snackbar (unchanged contract).
  /// - `MARKET_AUTHORITY_REQUIRED` → navigate via
  ///   [NavigationHandler.navigateToSellerRenewal] (canonical payment-only
  ///   renewal lifecycle).
  /// - anything else (including generic `FORBIDDEN`) → no-op, returns `false`.
  ///
  /// [context] must be mounted. [actionDescription] is a short Indonesian
  /// verb phrase describing what the user was trying to do, e.g.
  /// `'melakukan checkout'`, `'menempatkan bid'`, `'membuat forSale'`.
  static bool handle(
    BuildContext context, {
    required String? errorCode,
    required String actionDescription,
  }) {
    if (isMarketAuthorityRequired(errorCode)) {
      _navigateToSellerRenewal(context);
      return true;
    }
    if (isCommerceRestricted(errorCode)) {
      show(context, actionDescription: actionDescription);
      return true;
    }
    return false;
  }

  /// Shows the canonical `COMMERCE_RESTRICTED` UX.
  ///
  /// Call this from any commerce screen that receives `COMMERCE_RESTRICTED`
  /// from the backend. After calling this, the error is consumed — do NOT
  /// additionally show a generic error snackbar.
  ///
  /// [context] must be mounted. [actionDescription] is a short Indonesian
  /// verb phrase describing what the user was trying to do, e.g.
  /// `'melakukan checkout'`, `'menempatkan bid'`, `'membuat forSale'`.
  static void show(
    BuildContext context, {
    required String actionDescription,
  }) {
    AppSnackBar.showError(
      context,
      'Aktivitas commerce Anda saat ini dibatasi. '
      'Tidak dapat $actionDescription. '
      'Hubungi dukungan untuk informasi lebih lanjut.',
      duration: const Duration(seconds: 5),
    );
  }

  /// Routes the seller to the canonical payment-only renewal lifecycle.
  ///
  /// Uses the canonical navigation API (NavigationHandler
  /// → AppRouter → `RoutePaths.sellerRenewal` → SellerRenewalScreen) so no
  /// direct `context.push`/`context.go` bypasses the navigation doctrine.
  static void _navigateToSellerRenewal(BuildContext context) {
    // Presenter API is context-based but lives outside a WidgetRef tree;
    // resolve the canonical navigation provider from the nearest ProviderScope.
    ProviderScope.containerOf(context, listen: false)
        .read(navigationHandlerProvider)
        .navigateToSellerRenewal();
  }
}
