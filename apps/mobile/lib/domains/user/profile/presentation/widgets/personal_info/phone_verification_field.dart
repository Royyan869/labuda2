import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

/// Phone verification field widget
class PhoneVerificationField extends StatelessWidget {
  final TextEditingController phoneController;
  final bool phoneVerified;
  final DateTime? phoneVerifiedAt;
  final VoidCallback onVerifyPhone;
  final bool isDark;

  const PhoneVerificationField({
    super.key,
    required this.phoneController,
    required this.phoneVerified,
    this.phoneVerifiedAt,
    required this.onVerifyPhone,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        _buildPhoneInput(),
        if (!phoneVerified) ...[
          const SizedBox(height: 12),
          _buildVerifyPrompt(),
        ],
      ],
    );
  }

  Widget _buildPhoneInput() {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralGray50,
        border: Border.all(
          color: isDark ? AppColors.darkGray700 : AppColors.neutralGray300,
        ),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.phone_outlined,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
                size: 20,
              ),
              const SizedBox(width: 8),
              Text(
                'Phone Number',
                style: TextStyle(
                  fontSize: 12,
                  color: isDark
                      ? AppColors.neutralGray500
                      : AppColors.neutralGray600,
                ),
              ),
              const Spacer(),
              _buildVerificationBadge(),
            ],
          ),
          const SizedBox(height: 12),
          TextField(
            controller: phoneController,
            keyboardType: TextInputType.phone,
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w500,
              color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
            ),
            decoration: InputDecoration(
              hintText: '081234567890',
              hintStyle: TextStyle(
                color: isDark
                    ? AppColors.neutralGray600.withValues(alpha: 0.5)
                    : AppColors.neutralGray400.withValues(alpha: 0.6),
              ),
              contentPadding: const EdgeInsets.symmetric(
                horizontal: 12,
                vertical: 10,
              ),
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(8),
                borderSide: BorderSide(
                  color: isDark
                      ? AppColors.darkGray600
                      : AppColors.neutralGray300,
                ),
              ),
              enabledBorder: OutlineInputBorder(
                borderRadius: BorderRadius.circular(8),
                borderSide: BorderSide(
                  color: isDark
                      ? AppColors.darkGray600
                      : AppColors.neutralGray300,
                ),
              ),
              focusedBorder: OutlineInputBorder(
                borderRadius: BorderRadius.circular(8),
                borderSide: const BorderSide(
                  color: AppColors.primaryRed,
                  width: 1.5,
                ),
              ),
            ),
          ),
          if (phoneVerified && phoneVerifiedAt != null) ...[
            const SizedBox(height: 8),
            Text(
              'Verified on ${phoneVerifiedAt!.day}/${phoneVerifiedAt!.month}/${phoneVerifiedAt!.year}',
              style: TextStyle(
                fontSize: 11,
                color: isDark
                    ? AppColors.neutralGray500
                    : AppColors.neutralGray600,
              ),
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildVerificationBadge() {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: phoneVerified
            ? AppColors.successGreen.withValues(alpha: 0.1)
            : AppColors.warning.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(6),
        border: Border.all(
          color: phoneVerified
              ? AppColors.successGreen.withValues(alpha: 0.3)
              : AppColors.warning.withValues(alpha: 0.3),
        ),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            phoneVerified ? Icons.verified : Icons.warning,
            color: phoneVerified ? AppColors.successGreen : AppColors.warning,
            size: 12,
          ),
          const SizedBox(width: 4),
          Text(
            phoneVerified ? 'Verified' : 'Unverified',
            style: TextStyle(
              color: phoneVerified ? AppColors.successGreen : AppColors.warning,
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
              'Please verify your phone number',
              style: TextStyle(
                fontSize: 11,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray700,
              ),
            ),
          ),
          AppButton.text(text: 'Verify', onPressed: onVerifyPhone),
        ],
      ),
    );
  }
}
