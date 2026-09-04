import 'package:flutter/material.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';
import 'package:labuda/core/api/api_error_codes.dart' as codes;

/// Canonical presentation for backend commerce restriction errors.
///
/// This is the ONLY place in the mobile app that decides how to show
/// `COMMERCE_RESTRICTED` to the user. All commerce screens MUST call
/// [handleCommerceRestriction] instead of implementing their own
/// restriction error UI.
///
/// Commerce restriction ≠ account suspension.
/// The user can still browse; only commerce actions are blocked.
class CommerceRestrictionPresenter {
  const CommerceRestrictionPresenter._();

  /// Returns `true` if [errorCode] is the canonical commerce restriction code.
  static bool isCommerceRestricted(String? errorCode) =>
      errorCode == codes.commerceRestricted;

  /// Shows the canonical commerce restriction UX.
  ///
  /// Call this from any commerce screen that receives `COMMERCE_RESTRICTED`
  /// from the backend. After calling this, the error is consumed — do NOT
  /// additionally show a generic error snackbar.
  ///
  /// [context] must be mounted. [actionDescription] is a short Indonesian
  /// verb phrase describing what the user was trying to do, e.g.
  /// `'melakukan checkout'`, `'menempatkan bid'`, `'membuat listing'`.
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
}
