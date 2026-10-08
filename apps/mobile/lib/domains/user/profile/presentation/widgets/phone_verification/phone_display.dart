import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Displays the phone number being verified
class PhoneDisplay extends StatelessWidget {
  final String phoneNumber;

  const PhoneDisplay({super.key, required this.phoneNumber});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p16,
        vertical: AppMetrics.p12,
      ),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHigh,
        borderRadius: BorderRadius.circular(AppShape.r10),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.phone,
            size: AppIconSize.inlineGlyph,
            color: scheme.onSurfaceVariant,
          ),
          const SizedBox(width: 8),
          Text(
            phoneNumber,
            style: context.typeRoles.titleCompact.copyWith(
              fontWeight: FontWeight.w600,
              color: scheme.onSurface,
            ),
          ),
        ],
      ),
    );
  }
}
