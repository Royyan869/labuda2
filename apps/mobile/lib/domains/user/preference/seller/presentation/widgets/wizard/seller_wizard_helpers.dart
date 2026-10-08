import 'package:flutter/material.dart';
import 'package:labuda/shared/widgets/app_dialog.dart';

/// Helper class for Seller Wizard validation and dialogs
/// Extracted from SellerUpgradeWizardScreen to reduce complexity
class SellerWizardHelpers {
  /// Show confirmation dialog before closing wizard
  static Future<bool> showExitConfirmation(
    BuildContext context,
    bool hasAnyChanges,
  ) async {
    if (!hasAnyChanges) {
      return true; // No changes, allow pop
    }

    return AppDialog.confirm(
      context: context,
      title: 'Cancel Registration?',
      message: 'You have unsaved changes. Are you sure you want to exit?',
      confirmLabel: 'Exit',
      cancelLabel: 'Continue Filling',
      intent: AppDialogIntent.destructive,
    );
  }

  /// Check if Step 1 (Account Prerequisites) is valid.
  ///
  /// D2 HARD GATE (design scope v2): emailVerified is no longer part of this
  /// client-side gate — every authenticated user has already proven a
  /// verified email before the exchange, so the check is provably dead. The
  /// backend stays authoritative (EMAIL_VERIFICATION_REQUIRED handler on the
  /// submission path remains as defense-in-depth).
  static bool isAccountStepValid({
    required String username,
    required String phoneNumber,
    required String primaryAddress,
  }) {
    return username.isNotEmpty &&
        phoneNumber.isNotEmpty &&
        primaryAddress.isNotEmpty;
  }

  /// Check if Step 2 (Store Info) is valid
  static bool isStoreStepValid({required String storeName}) {
    return storeName.isNotEmpty;
  }
}
