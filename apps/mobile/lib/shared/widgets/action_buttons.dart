import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

/// Reusable Action Buttons untuk Cancel & Save/Submit actions
///
/// Features:
/// - Consistent button design dengan proper spacing
/// - Cancel button dengan neutral styling
/// - Save button dengan primary outline styling
/// - Loading state support untuk save action
/// - Responsive width dan customizable
/// - Theme adaptive (dark/light mode)
class ActionButtons extends StatelessWidget {
  final String? cancelText;
  final String saveText;
  final VoidCallback? onCancel;
  final VoidCallback? onSave;
  final bool isLoading;
  final bool isFullWidth;
  final EdgeInsets? padding;

  const ActionButtons({
    super.key,
    this.cancelText,
    required this.saveText,
    this.onCancel,
    this.onSave,
    this.isLoading = false,
    this.isFullWidth = true,
    this.padding,
  });

  /// Factory untuk screen forms dengan Save & Cancel
  factory ActionButtons.saveCancel({
    Key? key,
    VoidCallback? onCancel,
    VoidCallback? onSave,
    bool isLoading = false,
    String saveText = 'Save Changes',
    String cancelText = 'Cancel',
    bool isFullWidth = true,
    EdgeInsets? padding,
  }) {
    return ActionButtons(
      key: key,
      cancelText: cancelText,
      saveText: saveText,
      onCancel: onCancel,
      onSave: onSave,
      isLoading: isLoading,
      isFullWidth: isFullWidth,
      padding: padding,
    );
  }

  /// Factory untuk create/submit forms
  factory ActionButtons.submitCancel({
    Key? key,
    VoidCallback? onCancel,
    VoidCallback? onSubmit,
    bool isLoading = false,
    String submitText = 'Submit',
    String cancelText = 'Cancel',
    bool isFullWidth = true,
    EdgeInsets? padding,
  }) {
    return ActionButtons(
      key: key,
      cancelText: cancelText,
      saveText: submitText,
      onCancel: onCancel,
      onSave: onSubmit,
      isLoading: isLoading,
      isFullWidth: isFullWidth,
      padding: padding,
    );
  }

  /// Factory untuk confirmation dialogs
  factory ActionButtons.confirmCancel({
    Key? key,
    VoidCallback? onCancel,
    VoidCallback? onConfirm,
    bool isLoading = false,
    String confirmText = 'Confirm',
    String cancelText = 'Cancel',
    bool isFullWidth = true,
    EdgeInsets? padding,
  }) {
    return ActionButtons(
      key: key,
      cancelText: cancelText,
      saveText: confirmText,
      onCancel: onCancel,
      onSave: onConfirm,
      isLoading: isLoading,
      isFullWidth: isFullWidth,
      padding: padding,
    );
  }

  /// Factory untuk single action (hanya save/submit button)
  factory ActionButtons.singleAction({
    Key? key,
    required String actionText,
    VoidCallback? onAction,
    bool isLoading = false,
    bool isFullWidth = true,
    EdgeInsets? padding,
  }) {
    return ActionButtons(
      key: key,
      saveText: actionText,
      onSave: onAction,
      isLoading: isLoading,
      isFullWidth: isFullWidth,
      padding: padding,
    );
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    Widget buttonsWidget;

    if (cancelText != null && onCancel != null) {
      // Two buttons layout
      buttonsWidget = Row(
        children: [
          if (isFullWidth) ...[
            Expanded(child: _buildCancelButton(context, isDark)),
            const SizedBox(width: 16),
            Expanded(child: _buildSaveButton(context, isDark)),
          ] else ...[
            _buildCancelButton(context, isDark),
            const SizedBox(width: 16),
            _buildSaveButton(context, isDark),
          ],
        ],
      );
    } else {
      // Single button layout
      buttonsWidget = isFullWidth
          ? SizedBox(
              width: double.infinity,
              child: _buildSaveButton(context, isDark),
            )
          : _buildSaveButton(context, isDark);
    }

    return Padding(padding: padding ?? EdgeInsets.zero, child: buttonsWidget);
  }

  Widget _buildCancelButton(BuildContext context, bool isDark) {
    return SizedBox(
      height: 48,
      child: OutlinedButton(
        onPressed: isLoading ? null : onCancel,
        style: OutlinedButton.styleFrom(
          backgroundColor: Colors.transparent,
          side: BorderSide(
            color: isDark ? AppColors.neutralGray500 : AppColors.neutralGray400,
            width: 1.5,
          ),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(12),
          ),
        ),
        child: Text(
          cancelText!,
          style: TextStyle(
            color: isDark ? AppColors.neutralGray300 : AppColors.neutralGray700,
            fontSize: 16,
            fontWeight: FontWeight.w600,
          ),
        ),
      ),
    );
  }

  Widget _buildSaveButton(BuildContext context, bool isDark) {
    return SizedBox(
      height: 48,
      child: OutlinedButton(
        onPressed: (isLoading || onSave == null) ? null : onSave,
        style: OutlinedButton.styleFrom(
          backgroundColor: Colors.transparent,
          side: BorderSide(
            color: (isLoading || onSave == null)
                ? (isDark ? AppColors.neutralGray600 : AppColors.neutralGray300)
                : AppColors.primaryRed,
            width: 1.5,
          ),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(12),
          ),
        ),
        child: isLoading
            ? SizedBox(
                width: 20,
                height: 20,
                child: CircularProgressIndicator(
                  strokeWidth: 2,
                  valueColor: AlwaysStoppedAnimation<Color>(
                    AppColors.primaryRed,
                  ),
                ),
              )
            : Text(
                saveText,
                style: TextStyle(
                  color: (isLoading || onSave == null)
                      ? (isDark
                            ? AppColors.neutralGray500
                            : AppColors.neutralGray400)
                      : AppColors.primaryRed,
                  fontSize: 16,
                  fontWeight: FontWeight.w600,
                ),
              ),
      ),
    );
  }
}

/// Action Buttons dengan design alternative (filled save button)
/// Untuk kasus tertentu yang membutuhkan emphasis lebih pada save action
class ActionButtonsFilled extends StatelessWidget {
  final String? cancelText;
  final String saveText;
  final VoidCallback? onCancel;
  final VoidCallback? onSave;
  final bool isLoading;
  final bool isFullWidth;
  final EdgeInsets? padding;

  const ActionButtonsFilled({
    super.key,
    this.cancelText,
    required this.saveText,
    this.onCancel,
    this.onSave,
    this.isLoading = false,
    this.isFullWidth = true,
    this.padding,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    Widget buttonsWidget;

    if (cancelText != null && onCancel != null) {
      // Two buttons layout
      buttonsWidget = Row(
        children: [
          if (isFullWidth) ...[
            Expanded(child: _buildCancelButton(context, isDark)),
            const SizedBox(width: 16),
            Expanded(child: _buildSaveButton(context, isDark)),
          ] else ...[
            _buildCancelButton(context, isDark),
            const SizedBox(width: 16),
            _buildSaveButton(context, isDark),
          ],
        ],
      );
    } else {
      // Single button layout
      buttonsWidget = isFullWidth
          ? SizedBox(
              width: double.infinity,
              child: _buildSaveButton(context, isDark),
            )
          : _buildSaveButton(context, isDark);
    }

    return Padding(padding: padding ?? EdgeInsets.zero, child: buttonsWidget);
  }

  Widget _buildCancelButton(BuildContext context, bool isDark) {
    return AppButton.secondary(
      text: cancelText!,
      onPressed: isLoading ? null : onCancel,
    );
  }

  Widget _buildSaveButton(BuildContext context, bool isDark) {
    return AppButton.primary(
      text: saveText,
      onPressed: onSave,
      isLoading: isLoading,
    );
  }
}
