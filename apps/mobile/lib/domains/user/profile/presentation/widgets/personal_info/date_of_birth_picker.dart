import 'package:flutter/material.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';
import 'package:hishumi/shared/utils/app_formatters.dart';

/// Date of birth picker widget
class DateOfBirthPicker extends StatelessWidget {
  final DateTime? dateOfBirth;
  final VoidCallback onTap;

  const DateOfBirthPicker({super.key, this.dateOfBirth, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(AppShape.r8),
      child: Container(
        padding: const EdgeInsets.all(AppMetrics.p16),
        decoration: BoxDecoration(
          color: scheme.surface,
          // Border role, not ink.
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
                          : scheme.onSurfaceVariant,
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
}
