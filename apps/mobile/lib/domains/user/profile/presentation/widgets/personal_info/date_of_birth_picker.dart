import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Date of birth picker widget
class DateOfBirthPicker extends StatelessWidget {
  final DateTime? dateOfBirth;
  final VoidCallback onTap;
  final bool isDark;

  const DateOfBirthPicker({
    super.key,
    this.dateOfBirth,
    required this.onTap,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
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
}
