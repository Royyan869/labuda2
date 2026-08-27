import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

/// Email verification field widget
class EmailVerificationField extends StatelessWidget {
  final String email;
  final bool emailVerified;
  final VoidCallback onVerifyEmail;
  final bool isLoadingEmailVerification;
  final bool isDark;

  const EmailVerificationField({
    super.key,
    required this.email,
    required this.emailVerified,
    required this.onVerifyEmail,
    required this.isLoadingEmailVerification,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        _buildEmailInfo(),
        if (!emailVerified) ...[
          const SizedBox(height: 12),
          _buildVerifyPrompt(),
        ],
      ],
    );
  }

  Widget _buildEmailInfo() {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralGray50,
        border: Border.all(
          color: isDark ? AppColors.darkGray700 : AppColors.neutralGray300,
        ),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          Icon(
            Icons.email_outlined,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
            size: 20,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Login Email',
                  style: TextStyle(
                    fontSize: 12,
                    color: isDark
                        ? AppColors.neutralGray500
                        : AppColors.neutralGray600,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  email,
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w500,
                    color: isDark
                        ? AppColors.neutralWhite
                        : AppColors.neutralGray900,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  'Used for login and cannot be changed',
                  style: TextStyle(
                    fontSize: 11,
                    color: isDark
                        ? AppColors.neutralGray500
                        : AppColors.neutralGray600,
                  ),
                ),
              ],
            ),
          ),
          _buildVerificationBadge(),
        ],
      ),
    );
  }

  Widget _buildVerificationBadge() {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: emailVerified
            ? AppColors.successGreen.withValues(alpha: 0.1)
            : AppColors.warning.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(6),
        border: Border.all(
          color: emailVerified
              ? AppColors.successGreen.withValues(alpha: 0.3)
              : AppColors.warning.withValues(alpha: 0.3),
        ),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            emailVerified ? Icons.verified : Icons.warning,
            color: emailVerified ? AppColors.successGreen : AppColors.warning,
            size: 12,
          ),
          const SizedBox(width: 4),
          Text(
            emailVerified ? 'Verified' : 'Unverified',
            style: TextStyle(
              color: emailVerified ? AppColors.successGreen : AppColors.warning,
              fontSize: 10,
              fontWeight: FontWeight.w600,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildVerifyPrompt() {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: AppColors.warning.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: AppColors.warning.withValues(alpha: 0.2)),
      ),
      child: Row(
        children: [
          Icon(Icons.info_outline, color: AppColors.warning, size: 16),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              'Please verify your email to access all features',
              style: TextStyle(
                fontSize: 11,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray700,
              ),
            ),
          ),
          AppButton.text(
            text: 'Resend',
            onPressed: isLoadingEmailVerification ? null : onVerifyEmail,
            isLoading: isLoadingEmailVerification,
          ),
        ],
      ),
    );
  }
}
