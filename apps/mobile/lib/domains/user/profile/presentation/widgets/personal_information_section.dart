import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/generated/app_localizations.dart';

/// Personal Information Section (Contact Info Only)
/// KYC/KTP is now managed separately via KYC Status Card
class PersonalInformationSection extends StatelessWidget {
  final DateTime? dateOfBirth;
  final VoidCallback onSelectDateOfBirth;
  final String email; // Login email from AuthUser (read-only)
  final bool emailVerified; // Email verification status (display-only)
  final TextEditingController phoneController;
  final bool phoneVerified;
  final DateTime? phoneVerifiedAt;
  final VoidCallback onVerifyPhone;

  const PersonalInformationSection({
    super.key,
    this.dateOfBirth,
    required this.onSelectDateOfBirth,
    required this.email,
    required this.emailVerified,
    required this.phoneController,
    required this.phoneVerified,
    this.phoneVerifiedAt,
    required this.onVerifyPhone,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p20),
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r16),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.person_outline,
                color: scheme.onSurfaceVariant,
                size: 20,
              ),
              const SizedBox(width: 8),
              Text(
                AppLocalizations.of(context)!.contactIdentityInformation,
                style: TextStyle(
                  color: scheme.onSurface,
                  fontSize: AppType.s16,
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
                color: scheme.onSurfaceVariant,
                size: 20,
              ),
              const SizedBox(width: 8),
              Text(
                'Contact Information',
                style: TextStyle(
                  color: scheme.onSurface,
                  fontSize: AppType.s16,
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
    final scheme = Theme.of(context).colorScheme;
    return Column(
      children: [
        Container(
          padding: const EdgeInsets.all(AppMetrics.p16),
          decoration: BoxDecoration(
            color: scheme.surfaceContainerHighest,
            border: Border.all(color: scheme.outlineVariant),
            borderRadius: BorderRadius.circular(AppShape.r8),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Icon(
                    Icons.phone_outlined,
                    color: scheme.onSurfaceVariant,
                    size: 20,
                  ),
                  const SizedBox(width: 8),
                  Text(
                    'Phone Number',
                    style: TextStyle(
                      fontSize: AppType.s12,
                      color: scheme.onSurfaceVariant,
                    ),
                  ),
                  const Spacer(),
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: AppMetrics.p8,
                      vertical: AppMetrics.p4,
                    ),
                    decoration: BoxDecoration(
                      color: phoneVerified
                          ? context.statusColors.success.withValues(alpha: 0.1)
                          : context.statusColors.warning.withValues(alpha: 0.1),
                      borderRadius: BorderRadius.circular(AppShape.r6),
                      border: Border.all(
                        color: phoneVerified
                            ? context.statusColors.success.withValues(alpha: 0.3)
                            : context.statusColors.warning.withValues(alpha: 0.3),
                      ),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          phoneVerified ? Icons.verified : Icons.warning,
                          color: phoneVerified
                              ? context.statusColors.success
                              : context.statusColors.warning,
                          size: 12,
                        ),
                        const SizedBox(width: 4),
                        Text(
                          phoneVerified ? 'Verified' : 'Unverified',
                          style: TextStyle(
                            color: phoneVerified
                                ? context.statusColors.success
                                : context.statusColors.warning,
                            fontSize: AppType.s10,
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
                  fontSize: AppType.s14,
                  fontWeight: FontWeight.w500,
                  color: scheme.onSurface,
                ),
                decoration: InputDecoration(
                  hintText: '081234567890',
                  hintStyle: TextStyle(
                    color: scheme.onSurfaceVariant.withValues(alpha: 0.6),
                  ),
                  contentPadding: const EdgeInsets.symmetric(
                    horizontal: AppMetrics.p12,
                    vertical: AppMetrics.p10,
                  ),
                  // Border shapes come from AppTheme.inputDecorationTheme.
                ),
              ),
              if (phoneVerified && phoneVerifiedAt != null) ...[
                const SizedBox(height: 8),
                Text(
                  'Verified on ${phoneVerifiedAt!.day}/${phoneVerifiedAt!.month}/${phoneVerifiedAt!.year}',
                  style: TextStyle(
                    fontSize: AppType.s11,
                    color: scheme.onSurfaceVariant,
                  ),
                ),
              ],
            ],
          ),
        ),
        if (!phoneVerified) ...[
          const SizedBox(height: 12),
          Container(
            padding: const EdgeInsets.all(AppMetrics.p12),
            decoration: BoxDecoration(
              color: context.statusColors.warning.withValues(alpha: 0.08),
              borderRadius: BorderRadius.circular(AppShape.r8),
              border: Border.all(
                color: context.statusColors.warning.withValues(alpha: 0.2),
              ),
            ),
            child: Row(
              children: [
                Icon(Icons.info_outline, color: context.statusColors.warning, size: 16),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    'Please verify your phone number',
                    style: TextStyle(
                      fontSize: AppType.s11,
                      color: scheme.onSurfaceVariant,
                    ),
                  ),
                ),
                TextButton(
                  onPressed: onVerifyPhone,
                  child: const Text('Verify'),
                ),
              ],
            ),
          ),
        ],
      ],
    );
  }

  Widget _buildDateOfBirthPicker(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return InkWell(
      onTap: onSelectDateOfBirth,
      borderRadius: BorderRadius.circular(AppShape.r8),
      child: Container(
        padding: const EdgeInsets.all(AppMetrics.p16),
        decoration: BoxDecoration(
          color: scheme.surface,
          border: Border.all(color: scheme.outlineVariant),
          borderRadius: BorderRadius.circular(AppShape.r8),
        ),
        child: Row(
          children: [
            Icon(
              Icons.cake_outlined,
              color: scheme.onSurfaceVariant,
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
                      fontSize: AppType.s12,
                      color: scheme.onSurfaceVariant,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    dateOfBirth == null
                        ? 'Select your date of birth'
                        : '${dateOfBirth!.day}/${dateOfBirth!.month}/${dateOfBirth!.year}',
                    style: TextStyle(
                      fontSize: AppType.s14,
                      fontWeight: dateOfBirth == null
                          ? FontWeight.normal
                          : FontWeight.w500,
                      color: dateOfBirth == null
                          ? scheme.onSurfaceVariant
                          : scheme.onSurface,
                    ),
                  ),
                ],
              ),
            ),
            Icon(
              Icons.calendar_today,
              size: 18,
              color: scheme.onSurfaceVariant,
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildReadOnlyEmailField(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Column(
      children: [
        Container(
          padding: const EdgeInsets.all(AppMetrics.p16),
          decoration: BoxDecoration(
            color: scheme.surfaceContainerHighest,
            border: Border.all(color: scheme.outlineVariant),
            borderRadius: BorderRadius.circular(AppShape.r8),
          ),
          child: Row(
            children: [
              Icon(
                Icons.email_outlined,
                color: scheme.onSurfaceVariant,
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
                        fontSize: AppType.s12,
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      email,
                      style: TextStyle(
                        fontSize: AppType.s14,
                        fontWeight: FontWeight.w500,
                        color: scheme.onSurface,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      'Used for login and cannot be changed',
                      style: TextStyle(
                        fontSize: AppType.s11,
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                  ],
                ),
              ),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p8, vertical: AppMetrics.p4),
                decoration: BoxDecoration(
                  color: emailVerified
                      ? context.statusColors.success.withValues(alpha: 0.1)
                      : context.statusColors.warning.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(AppShape.r6),
                  border: Border.all(
                    color: emailVerified
                        ? context.statusColors.success.withValues(alpha: 0.3)
                        : context.statusColors.warning.withValues(alpha: 0.3),
                  ),
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(
                      emailVerified ? Icons.verified : Icons.warning,
                      color: emailVerified
                          ? context.statusColors.success
                          : context.statusColors.warning,
                      size: 12,
                    ),
                    const SizedBox(width: 4),
                    Text(
                      emailVerified ? 'Verified' : 'Unverified',
                      style: TextStyle(
                        color: emailVerified
                            ? context.statusColors.success
                            : context.statusColors.warning,
                        fontSize: AppType.s10,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}
