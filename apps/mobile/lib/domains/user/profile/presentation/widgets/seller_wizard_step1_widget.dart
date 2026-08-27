import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/helpers/canonical_email_validator.dart';
import 'package:labuda/shared/helpers/canonical_phone_validator.dart';

/// Step 1: Email & Phone Verification Widget
///
/// Simplified component for seller upgrade wizard
/// Only handles email and phone verification with editable fields
class SellerWizardStep1Widget extends StatelessWidget {
  final GlobalKey<FormState> formKey;
  final TextEditingController emailController;
  final TextEditingController phoneController;
  final bool isEmailVerified;
  final bool isPhoneVerified;
  final VoidCallback onEmailVerify;
  final VoidCallback onPhoneVerify;
  final bool isDark;

  const SellerWizardStep1Widget({
    super.key,
    required this.formKey,
    required this.emailController,
    required this.phoneController,
    required this.isEmailVerified,
    required this.isPhoneVerified,
    required this.onEmailVerify,
    required this.onPhoneVerify,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return Form(
      key: formKey,
      child: ListView(
        padding: const EdgeInsets.all(24),
        children: [
          _buildSectionHeader(
            'Email & Phone Verification',
            'Email and phone number must be unique and verified',
          ),
          const SizedBox(height: 24),

          // Email Verification Card
          _buildEditableVerificationField(
            context,
            label: 'Email',
            controller: emailController,
            isVerified: isEmailVerified,
            onVerify: onEmailVerify,
            keyboardType: TextInputType.emailAddress,
            validator: (value) =>
                CanonicalEmailValidator.validationMessage(value),
          ),
          const SizedBox(height: 16),

          // Phone Verification Card
          _buildEditableVerificationField(
            context,
            label: 'Phone Number',
            controller: phoneController,
            isVerified: isPhoneVerified,
            onVerify: onPhoneVerify,
            keyboardType: TextInputType.phone,
            validator: (value) {
              if (value == null || value.isEmpty) {
                return 'Phone number is required';
              }
              if (!CanonicalPhoneValidator.isValid(
                value.replaceAll(RegExp(r'[\s-]'), ''),
              )) {
                return 'Invalid phone number format';
              }
              return null;
            },
          ),
          const SizedBox(height: 24),

          // Info Box
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
              borderRadius: BorderRadius.circular(12),
              border: Border.all(
                color: isDark
                    ? AppColors.darkGray600
                    : AppColors.neutralGray200,
                width: 1,
              ),
            ),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Icon(
                  Icons.info_outline,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                  size: 20,
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    'Make sure the email and phone number you enter are active and can receive verification codes.',
                    style: TextStyle(
                      fontSize: 14,
                      color: isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray600,
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

  Widget _buildSectionHeader(String title, String? subtitle) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          title,
          style: TextStyle(
            fontSize: 20,
            fontWeight: FontWeight.bold,
            color: isDark ? AppColors.neutralGray200 : AppColors.neutralGray900,
          ),
        ),
        if (subtitle != null) ...[
          const SizedBox(height: 8),
          Text(
            subtitle,
            style: TextStyle(
              fontSize: 14,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
          ),
        ],
      ],
    );
  }

  Widget _buildEditableVerificationField(
    BuildContext context, {
    required String label,
    required TextEditingController controller,
    required bool isVerified,
    required VoidCallback onVerify,
    required TextInputType keyboardType,
    required String? Function(String?) validator,
  }) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isVerified
              ? AppColors.successGreen
              : (isDark ? AppColors.darkGray600 : AppColors.neutralGray200),
          width: 2,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Label
          Text(
            label,
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w600,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
          ),
          const SizedBox(height: 12),

          // Text Field with Verify Button
          Row(
            children: [
              Expanded(
                child: TextFormField(
                  controller: controller,
                  keyboardType: keyboardType,
                  enabled: !isVerified, // Disable if already verified
                  style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.w500,
                    color: isDark
                        ? AppColors.neutralGray200
                        : AppColors.neutralGray900,
                  ),
                  decoration: InputDecoration(
                    hintText: 'Enter $label',
                    hintStyle: TextStyle(
                      color: isDark
                          ? AppColors.neutralGray500
                          : AppColors.neutralGray400,
                    ),
                    border: InputBorder.none,
                    contentPadding: EdgeInsets.zero,
                    isDense: true,
                  ),
                  validator: validator,
                ),
              ),
              const SizedBox(width: 12),

              // Verified Status or Verify Button
              if (isVerified)
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 12,
                    vertical: 6,
                  ),
                  decoration: BoxDecoration(
                    color: AppColors.successGreen.withValues(alpha: 0.1),
                    borderRadius: BorderRadius.circular(20),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(
                        Icons.check_circle,
                        color: AppColors.successGreen,
                        size: 16,
                      ),
                      const SizedBox(width: 6),
                      Text(
                        'Verified',
                        style: TextStyle(
                          color: AppColors.successGreen,
                          fontWeight: FontWeight.w600,
                          fontSize: 13,
                        ),
                      ),
                    ],
                  ),
                )
              else
                ElevatedButton(
                  onPressed: onVerify,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppColors.primaryRed,
                    foregroundColor: Colors.white,
                    padding: const EdgeInsets.symmetric(
                      horizontal: 16,
                      vertical: 8,
                    ),
                    minimumSize: Size.zero,
                    tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                  ),
                  child: const Text(
                    'Verify',
                    style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600),
                  ),
                ),
            ],
          ),
        ],
      ),
    );
  }
}
