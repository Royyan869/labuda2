import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/widgets/app_bottom_sheet.dart';

/// Collection of modal dialogs used in create post screen
class ContentModals {
  /// Show input modal for adding hashtags or other text inputs
  static void showInputModal({
    required BuildContext context,
    required String title,
    required String hintText,
    required Function(String) onAdd,
  }) {
    String inputText = '';

    AppBottomSheet.show(
      context: context,
      title: title,
      showSaveButton: true,
      saveButtonText: 'Add',
      onSave: () {
        if (inputText.isNotEmpty) {
          onAdd(inputText);
          Navigator.of(context).pop();
        }
      },
      content: TextField(
        autofocus: true,
        onChanged: (value) => inputText = value,
        decoration: InputDecoration(
          hintText: hintText,
          border: OutlineInputBorder(borderRadius: BorderRadius.circular(12)),
          contentPadding: const EdgeInsets.symmetric(
            horizontal: 16,
            vertical: 12,
          ),
        ),
      ),
    );
  }

  /// Show exit confirmation dialog
  static void showExitDialog({
    required BuildContext context,
    required VoidCallback onDiscard,
  }) {
    showDialog<void>(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          title: const Text('Discard Changes?'),
          content: const Text('Filled data will be lost. Are you sure?'),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(dialogContext).pop(),
              child: const Text('Cancel'),
            ),
            TextButton(
              onPressed: () {
                Navigator.of(dialogContext).pop();
                onDiscard();
              },
              style: TextButton.styleFrom(
                foregroundColor: AppColors.primaryRed,
              ),
              child: const Text('Discard'),
            ),
          ],
        );
      },
    );
  }
}
