import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/generated/app_localizations.dart';

/// Personal Information Section (Contact Info Only)
/// KYC/KTP is now managed separately via KYC Status Card
class PersonalInformationSection extends StatelessWidget {
  final DateTime? dateOfBirth;
  final VoidCallback onSelectDateOfBirth;
  final String email; // Login email from AuthUser (read-only)
  final bool emailVerified; // Email verification status
  final VoidCallback onVerifyEmail; // Send email verification
  final bool isLoadingEmailVerification;
  final TextEditingController phoneController;
  final bool phoneVerified;
  final DateTime? phoneVerifiedAt;
  final VoidCallback onVerifyPhone;
  final bool isDark;

  const PersonalInformationSection({
    super.key,
    this.dateOfBirth,
    required this.onSelectDateOfBirth,
    required this.email,
    required this.emailVerified,
    required this.onVerifyEmail,
    required this.isLoadingEmailVerification,
    required this.phoneController,
    required this.phoneVerified,
    this.phoneVerifiedAt,
    required this.onVerifyPhone,
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
          Row(
            children: [
              Icon(
                Icons.person_outline,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
                size: 20,
              ),
              const SizedBox(width: 8),
              Text(
                AppLocalizations.of(context)!.contactIdentityInformation,
                style: TextStyle(
                  color: isDark
                      ? AppColors.neutralGray200
                      : AppColors.neutralGray900,
                  fontSize: 16,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ],
          ),
          const SizedBox(height: 16),

          // Date of Birth Picker
          _buildDateOfBirthPicker(context),
          const SizedBox(height: 24),

          // Contact Information Header
          Row(
            children: [
              Icon(
                Icons.contact_mail_outlined,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
                size: 20,
              ),
              const SizedBox(width: 8),
              Text(
                'Contact Information',
                style: TextStyle(
                  color: isDark
                      ? AppColors.neutralGray200
                      : AppColors.neutralGray900,
                  fontSize: 16,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ],
          ),
          const SizedBox(height: 16),

          // Email (Read-only)
          _buildReadOnlyEmailField(context),
          const SizedBox(height: 16),

          // Phone Verification Section (includes input)
          _buildPhoneVerificationSection(context),
        ],
      ),
    );
  }

  Widget _buildPhoneVerificationSection(BuildContext context) {
    return Column(
      children: [
        Container(
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
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 4,
                    ),
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
                          color: phoneVerified
                              ? AppColors.successGreen
                              : AppColors.warning,
                          size: 12,
                        ),
                        const SizedBox(width: 4),
                        Text(
                          phoneVerified ? 'Verified' : 'Unverified',
                          style: TextStyle(
                            color: phoneVerified
                                ? AppColors.successGreen
                                : AppColors.warning,
                            fontSize: 10,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              TextField(
                controller: phoneController,
                keyboardType: TextInputType.phone,
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w500,
                  color: isDark
                      ? AppColors.neutralWhite
                      : AppColors.neutralGray900,
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
        ),
        if (!phoneVerified) ...[
          const SizedBox(height: 12),
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: AppColors.warning.withValues(alpha: 0.08),
              borderRadius: BorderRadius.circular(8),
              border: Border.all(
                color: AppColors.warning.withValues(alpha: 0.2),
              ),
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
          ),
        ],
      ],
    );
  }

  Widget _buildDateOfBirthPicker(BuildContext context) {
    return InkWell(
      onTap: onSelectDateOfBirth,
      borderRadius: BorderRadius.circular(8),
      child: Container(
        padding: const EdgeInsets.all(16),
        decoration: BoxDecoration(
          color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
          border: Border.all(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray300,
          ),
          borderRadius: BorderRadius.circular(8),
        ),
        child: Row(
          children: [
            Icon(
              Icons.cake_outlined,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
              size: 20,
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Date of Birth (Optional)',
                    style: TextStyle(
                      fontSize: 12,
                      color: isDark
                          ? AppColors.neutralGray500
                          : AppColors.neutralGray600,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    dateOfBirth == null
                        ? 'Select your date of birth'
                        : '${dateOfBirth!.day}/${dateOfBirth!.month}/${dateOfBirth!.year}',
                    style: TextStyle(
                      fontSize: 14,
                      fontWeight: dateOfBirth == null
                          ? FontWeight.normal
                          : FontWeight.w500,
                      color: dateOfBirth == null
                          ? (isDark
                                ? AppColors.neutralGray500
                                : AppColors.neutralGray600)
                          : (isDark
                                ? AppColors.neutralWhite
                                : AppColors.neutralGray900),
                    ),
                  ),
                ],
              ),
            ),
            Icon(
              Icons.calendar_today,
              size: 18,
              color: isDark
                  ? AppColors.neutralGray500
                  : AppColors.neutralGray600,
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildReadOnlyEmailField(BuildContext context) {
    return Column(
      children: [
        Container(
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
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
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
              Container(
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
                      color: emailVerified
                          ? AppColors.successGreen
                          : AppColors.warning,
                      size: 12,
                    ),
                    const SizedBox(width: 4),
                    Text(
                      emailVerified ? 'Verified' : 'Unverified',
                      style: TextStyle(
                        color: emailVerified
                            ? AppColors.successGreen
                            : AppColors.warning,
                        fontSize: 10,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
        if (!emailVerified) ...[
          const SizedBox(height: 12),
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: AppColors.warning.withValues(alpha: 0.08),
              borderRadius: BorderRadius.circular(8),
              border: Border.all(
                color: AppColors.warning.withValues(alpha: 0.2),
              ),
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
          ),
        ],
      ],
    );
  }
}
