import 'package:flutter/material.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/generated/app_localizations.dart';

/// Security email change section widget
/// Extracted from security_screen.dart untuk better organization
class SecurityEmailSectionWidget extends StatelessWidget {
  final TextEditingController newEmailController;
  final TextEditingController currentPasswordForEmailController;
  final bool isLoadingEmail;
  final VoidCallback onChangeEmail;
  final AuthUser user;
  final bool isDark;

  const SecurityEmailSectionWidget({
    super.key,
    required this.newEmailController,
    required this.currentPasswordForEmailController,
    required this.isLoadingEmail,
    required this.onChangeEmail,
    required this.user,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            AppLocalizations.of(context)!.changeEmailAddress,
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray200
                  : AppColors.neutralGray900,
              fontSize: 16,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 16),
          AppTextField.email(
            controller: newEmailController,
            labelText: AppLocalizations.of(context)!.newEmailAddress,
            hintText: AppLocalizations.of(context)!.enterNewEmailAddress,
          ),
          const SizedBox(height: 16),
          AppTextField.password(
            controller: currentPasswordForEmailController,
            labelText: AppLocalizations.of(context)!.currentPassword,
            hintText: AppLocalizations.of(
              context,
            )!.enterCurrentPasswordToConfirm,
          ),
          const SizedBox(height: 20),
          _buildOutlineButton(
            context: context,
            text: AppLocalizations.of(context)!.updateEmail,
            onPressed: isLoadingEmail ? null : onChangeEmail,
            isLoading: isLoadingEmail,
          ),
          const SizedBox(height: 12),
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: AppColors.statusInfo.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(8),
              border: Border.all(
                color: AppColors.statusInfo.withValues(alpha: 0.3),
              ),
            ),
            child: Row(
              children: [
                Icon(Icons.info_outline, color: AppColors.statusInfo, size: 16),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    AppLocalizations.of(context)!.verifyNewEmailMessage,
                    style: TextStyle(
                      color: AppColors.neutralGray700,
                      fontSize: 12,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildOutlineButton({
    required BuildContext context,
    required String text,
    required VoidCallback? onPressed,
    bool isLoading = false,
  }) {
    return SizedBox(
      width: double.infinity,
      child: OutlinedButton(
        onPressed: onPressed,
        style: OutlinedButton.styleFrom(
          padding: const EdgeInsets.symmetric(vertical: 16),
          side: const BorderSide(color: AppColors.primaryRed),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(12),
          ),
        ),
        child: isLoading
            ? const SizedBox(
                height: 20,
                width: 20,
                child: CircularProgressIndicator(
                  strokeWidth: 2,
                  valueColor: AlwaysStoppedAnimation<Color>(
                    AppColors.primaryRed,
                  ),
                ),
              )
            : Text(
                text,
                style: const TextStyle(
                  color: AppColors.primaryRed,
                  fontSize: 16,
                  fontWeight: FontWeight.w600,
                ),
              ),
      ),
    );
  }
}
