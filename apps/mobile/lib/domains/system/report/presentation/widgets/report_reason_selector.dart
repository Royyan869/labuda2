import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/report/domain/entities/entities.dart';

/// Report Reason Selector Widget
///
/// Displays all available report reasons in a grid layout.
/// Each reason has an icon and description to help users choose appropriately.
class ReportReasonSelector extends StatelessWidget {
  final ReportReasonType selectedReason;
  final Function(ReportReasonType) onReasonSelected;
  final bool isEnabled;

  const ReportReasonSelector({
    super.key,
    required this.selectedReason,
    required this.onReasonSelected,
    this.isEnabled = true,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Why are you reporting this?',
          style: TextStyle(
            fontSize: 16,
            fontWeight: FontWeight.w600,
            color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
          ),
        ),
        const SizedBox(height: 16),
        GridView.count(
          crossAxisCount: 2,
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          mainAxisSpacing: 12,
          crossAxisSpacing: 12,
          childAspectRatio: 1.0,
          children: ReportReasonType.values.map((reason) {
            final isSelected = selectedReason == reason;
            return _ReasonCard(
              reason: reason,
              isSelected: isSelected,
              isDark: isDark,
              isEnabled: isEnabled,
              onTap: isEnabled ? () => onReasonSelected(reason) : null,
            );
          }).toList(),
        ),
      ],
    );
  }
}

class _ReasonCard extends StatelessWidget {
  final ReportReasonType reason;
  final bool isSelected;
  final bool isDark;
  final bool isEnabled;
  final VoidCallback? onTap;

  const _ReasonCard({
    required this.reason,
    required this.isSelected,
    required this.isDark,
    required this.isEnabled,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(12),
      child: Container(
        decoration: BoxDecoration(
          color: isSelected
              ? AppColors.primaryBlue.withValues(alpha: 0.1)
              : (isDark ? AppColors.darkGray700 : AppColors.neutralGray100),
          borderRadius: BorderRadius.circular(12),
          border: Border.all(
            color: isSelected
                ? AppColors.primaryBlue
                : (isDark ? AppColors.darkGray600 : AppColors.neutralGray300),
            width: isSelected ? 2 : 1,
          ),
        ),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                color: isSelected
                    ? AppColors.primaryBlue
                    : (isDark
                          ? AppColors.darkGray600
                          : AppColors.neutralGray200),
                shape: BoxShape.circle,
              ),
              child: Icon(
                _getIconForReason(reason),
                color: isSelected
                    ? AppColors.neutralWhite
                    : (isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray600),
                size: 20,
              ),
            ),
            const SizedBox(height: 8),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 8),
              child: Text(
                reason.displayName,
                style: TextStyle(
                  fontSize: 12,
                  fontWeight: isSelected ? FontWeight.w600 : FontWeight.w500,
                  color: isSelected
                      ? AppColors.primaryBlue
                      : (isDark
                            ? AppColors.neutralWhite
                            : AppColors.neutralGray900),
                ),
                textAlign: TextAlign.center,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ],
        ),
      ),
    );
  }

  IconData _getIconForReason(ReportReasonType reason) {
    switch (reason) {
      case ReportReasonType.spam:
        return Icons.mark_email_unread_outlined;
      case ReportReasonType.harassment:
        return Icons.person_off_outlined;
      case ReportReasonType.inappropriateContent:
        return Icons.block_outlined;
      case ReportReasonType.scam:
        return Icons.warning_outlined;
      case ReportReasonType.fakeProduct:
        return Icons.shopping_bag_outlined;
      case ReportReasonType.copyrightViolation:
        return Icons.copyright_outlined;
      case ReportReasonType.violence:
        return Icons.error_outline;
      case ReportReasonType.hateSpeech:
        return Icons.report_problem_outlined;
      case ReportReasonType.falseInformation:
        return Icons.help_outline;
      case ReportReasonType.other:
        return Icons.more_horiz;
    }
  }
}
