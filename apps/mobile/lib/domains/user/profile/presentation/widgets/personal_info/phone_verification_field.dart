import 'package:hishumi/core/core.dart';
import 'package:flutter/material.dart';
import 'package:hishumi/shared/utils/app_formatters.dart';

/// F5-local fit measure: whether a single-line title + badge pair fits the
/// incoming width. Local copy (no new shared authority).
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

/// Phone verification field widget
class PhoneVerificationField extends StatelessWidget {
  final TextEditingController phoneController;
  final bool phoneVerified;
  final DateTime? phoneVerifiedAt;
  final VoidCallback onVerifyPhone;

  const PhoneVerificationField({
    super.key,
    required this.phoneController,
    required this.phoneVerified,
    this.phoneVerifiedAt,
    required this.onVerifyPhone,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Column(
      children: [
        _buildPhoneInput(context, scheme),
        if (!phoneVerified) ...[
          const SizedBox(height: 12),
          _buildVerifyPrompt(context, scheme),
        ],
      ],
    );
  }

  Widget _buildPhoneInput(BuildContext context, ColorScheme scheme) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        // Container roles, not ink: this block was a solid `onSurfaceVariant`
        // slab holding `onSurfaceVariant` labels and an `onSurface` field.
        color: scheme.surfaceContainerHighest,
        border: Border.all(color: scheme.outlineVariant),
        borderRadius: BorderRadius.circular(AppShape.r8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // F5 header (adaptive): title + verification badge share one row
          // when the single-line pair fits, else the title stacks over the
          // fully-readable badge.
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
                    _buildVerificationBadge(context, scheme),
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
                  _buildVerificationBadge(context, scheme),
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
            // Input chrome (radius, borders, padding, hint ink) is theme data
            // (`inputDecorationTheme`) — restating it here duplicated the
            // authority and put an ink role on a border.
            decoration: const InputDecoration(hintText: '081234567890'),
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
    );
  }

  Widget _buildVerificationBadge(BuildContext context, ColorScheme scheme) {
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
          // F5: wraps (never truncates) when an ancestor bounds this badge;
          // hugs intrinsic width otherwise.
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

  Widget _buildVerifyPrompt(BuildContext context, ColorScheme scheme) {
    return Container(
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
          TextButton(onPressed: onVerifyPhone, child: const Text('Verify')),
        ],
      ),
    );
  }
}
