import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

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

    final result = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Cancel Registration?'),
        content: const Text(
          'You have unsaved changes. Are you sure you want to exit?',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: const Text('Continue Filling'),
          ),
          ElevatedButton(
            onPressed: () => Navigator.of(context).pop(true),
            style: ElevatedButton.styleFrom(backgroundColor: AppColors.error),
            child: const Text('Exit'),
          ),
        ],
      ),
    );

    return result ?? false;
  }

  /// Check if Step 1 (Account Prerequisites) is valid
  static bool isAccountStepValid({
    required bool emailVerified,
    required String username,
    required String bio,
    required String phoneNumber,
    required String senderAddress,
  }) {
    return emailVerified &&
        username.isNotEmpty &&
        bio.isNotEmpty &&
        phoneNumber.isNotEmpty &&
        senderAddress.isNotEmpty;
  }

  /// Check if Step 2 (Store Info) is valid
  static bool isStoreStepValid({required String storeName}) {
    return storeName.isNotEmpty;
  }

  /// Check if any form field has been filled
  static bool hasAnyChanges({
    required String username,
    required String bio,
    required String phoneNumber,
    required String senderAddress,
    required String farmName,
    required String? farmPhotoUrl,
    required String? selectedStorePhotoPath,
    required bool agreeToTerms,
  }) {
    return username.isNotEmpty ||
        bio.isNotEmpty ||
        phoneNumber.isNotEmpty ||
        senderAddress.isNotEmpty ||
        farmName.isNotEmpty ||
        farmPhotoUrl != null ||
        selectedStorePhotoPath != null ||
        agreeToTerms;
  }
}
