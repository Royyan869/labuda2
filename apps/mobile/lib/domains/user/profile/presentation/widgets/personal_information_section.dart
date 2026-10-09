import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/utils/app_formatters.dart';

/// F5-local fit measure: whether a single-line title + badge pair fits the
/// incoming width. Same principle as the order F2 fit-measure; local copy
/// (no new shared authority) because this library cannot see it.
bool _fitsTitleBadgeSingleLine({
  required BuildContext context,
  required double maxWidth,
  required String title,
  required String badge,
  required TextStyle? titleStyle,
  required TextStyle? badgeStyle,
  required double fixedExtrasWidth,
}) {
  if (!maxWidth.isFinite) {
    return false;
  }
  final TextDirection direction = Directionality.of(context);
  final TextScaler scaler = MediaQuery.textScalerOf(context);

  double singleLineWidth(String text, TextStyle? style) {
    final TextPainter painter = TextPainter(
      text: TextSpan(text: text, style: style),
      textDirection: direction,
      textScaler: scaler,
      maxLines: 1,
    )..layout();
    return painter.width;
  }

  const double safetyMargin = 2;
  return singleLineWidth(title, titleStyle) +
          fixedExtrasWidth +
          singleLineWidth(badge, badgeStyle) +
          safetyMargin <=
      maxWidth;
}

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
      padding: const EdgeInsets.all(AppMetrics.p24),
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
                size: AppIconSize.action,
              ),
              const SizedBox(width: 8),
              Text(
                AppLocalizations.of(context)!.contactIdentityInformation,
                style: context.typeRoles.titleCompact.copyWith(
                  color: scheme.onSurface,
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
                size: AppIconSize.action,
              ),
              const SizedBox(width: 8),
              Text(
                'Contact Information',
                style: context.typeRoles.titleCompact.copyWith(
                  color: scheme.onSurface,
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

  /// Verification badge (Verified/Unverified). The label wraps instead of
  /// truncating when an ancestor bounds this badge, and hugs intrinsic
  /// width otherwise — business-meaningful copy is never ellipsized.
  Widget _buildPhoneBadge(BuildContext context) {
    return Container(
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
            size: AppIconSize.inlineGlyph,
          ),
          const SizedBox(width: 4),
          Flexible(
            child: Text(
              phoneVerified ? 'Verified' : 'Unverified',
              style: context.typeRoles.labelMicro.copyWith(
                color: phoneVerified
                    ? context.statusColors.success
                    : context.statusColors.warning,
                fontWeight: FontWeight.w600,
              ),
              softWrap: true,
            ),
          ),
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
              // F5 header (adaptive): title + verification badge share one
              // row when the single-line pair fits, else the title stacks
              // over the fully-readable badge.
              LayoutBuilder(
                builder: (BuildContext context, BoxConstraints constraints) {
                  final TextStyle titleStyle = context.typeRoles.labelMicro
                      .copyWith(color: scheme.onSurfaceVariant);
                  final TextStyle badgeStyle = context.typeRoles.labelMicro
                      .copyWith(fontWeight: FontWeight.w600);
                  final String badgeText = phoneVerified
                      ? 'Verified'
                      : 'Unverified';
                  final bool fits = _fitsTitleBadgeSingleLine(
                    context: context,
                    maxWidth: constraints.maxWidth,
                    title: 'Phone Number',
                    badge: badgeText,
                    titleStyle: titleStyle,
                    badgeStyle: badgeStyle,
                    fixedExtrasWidth:
                        AppIconSize.action +
                        8 +
                        AppIconSize.inlineGlyph +
                        4 +
                        AppMetrics.p8 * 2,
                  );
                  if (fits) {
                    return Row(
                      children: [
                        Icon(
                          Icons.phone_outlined,
                          color: scheme.onSurfaceVariant,
                          size: AppIconSize.action,
                        ),
                        const SizedBox(width: 8),
                        Text(
                          'Phone Number',
                          style: context.typeRoles.labelMicro.copyWith(
                            color: scheme.onSurfaceVariant,
                          ),
                        ),
                        const Spacer(),
                        _buildPhoneBadge(context),
                      ],
                    );
                  }
                  return Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Icon(
                            Icons.phone_outlined,
                            color: scheme.onSurfaceVariant,
                            size: AppIconSize.action,
                          ),
                          const SizedBox(width: 8),
                          Expanded(
                            child: Text(
                              'Phone Number',
                              style: context.typeRoles.labelMicro.copyWith(
                                color: scheme.onSurfaceVariant,
                              ),
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 4),
                      _buildPhoneBadge(context),
                    ],
                  );
                },
              ),
              const SizedBox(height: 12),
              TextField(
                controller: phoneController,
                keyboardType: TextInputType.phone,
                style: Theme.of(context).textTheme.bodyLarge?.copyWith(
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
                    vertical: AppMetrics.p12,
                  ),
                  // Border shapes come from AppTheme.inputDecorationTheme.
                ),
              ),
              if (phoneVerified && phoneVerifiedAt != null) ...[
                const SizedBox(height: 8),
                Text(
                  'Verified on ${AppFormatters.formatShortDate(phoneVerifiedAt!)}',
                  style: context.typeRoles.labelMicro.copyWith(
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
                Icon(
                  Icons.info_outline,
                  color: context.statusColors.warning,
                  size: AppIconSize.inlineGlyph,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    'Please verify your phone number',
                    style: context.typeRoles.labelMicro.copyWith(
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
              size: AppIconSize.action,
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Date of Birth (Optional)',
                    style: context.typeRoles.labelMicro.copyWith(
                      color: scheme.onSurfaceVariant,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    dateOfBirth == null
                        ? 'Select your date of birth'
                        : AppFormatters.formatShortDate(dateOfBirth!),
                    style: context.typeRoles.bodyDense.copyWith(
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
              size: AppIconSize.action,
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
                size: AppIconSize.action,
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'Login Email',
                      style: context.typeRoles.labelMicro.copyWith(
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      email,
                      style: context.typeRoles.bodyDense.copyWith(
                        fontWeight: FontWeight.w500,
                        color: scheme.onSurface,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      'Used for login and cannot be changed',
                      style: context.typeRoles.labelMicro.copyWith(
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                  ],
                ),
              ),
              Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: AppMetrics.p8,
                  vertical: AppMetrics.p4,
                ),
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
                      size: AppIconSize.inlineGlyph,
                    ),
                    const SizedBox(width: 4),
                    Text(
                      emailVerified ? 'Verified' : 'Unverified',
                      style: context.typeRoles.labelMicro.copyWith(
                        color: emailVerified
                            ? context.statusColors.success
                            : context.statusColors.warning,
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
