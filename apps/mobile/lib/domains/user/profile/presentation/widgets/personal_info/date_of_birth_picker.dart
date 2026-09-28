import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Date of birth picker widget
class DateOfBirthPicker extends StatelessWidget {
  final DateTime? dateOfBirth;
  final VoidCallback onTap;

  const DateOfBirthPicker({
    super.key,
    this.dateOfBirth,
    required this.onTap,
    
  });

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
          border: Border.all(
            color: scheme.onSurfaceVariant,
          ),
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
                          : scheme.onSurfaceVariant,
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
}
