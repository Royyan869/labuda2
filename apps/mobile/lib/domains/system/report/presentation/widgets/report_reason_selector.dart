import 'package:flutter/material.dart';
import 'package:hishumi/domains/system/report/domain/entities/entities.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

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
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Why are you reporting this?',
          style: context.typeRoles.titleSection.copyWith(
            fontWeight: FontWeight.w600,
            color: Theme.of(context).colorScheme.onSurface,
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
  final bool isEnabled;
  final VoidCallback? onTap;

  const _ReasonCard({
    required this.reason,
    required this.isSelected,
    required this.isEnabled,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(AppShape.r12),
      child: Container(
        decoration: BoxDecoration(
          color: isSelected
              ? Theme.of(context).colorScheme.secondary.withValues(alpha: 0.1)
              : (Theme.of(context).colorScheme.surfaceContainer),
          borderRadius: BorderRadius.circular(AppShape.r12),
          border: Border.all(
            color: isSelected
                ? Theme.of(context).colorScheme.secondary
                : (Theme.of(context).colorScheme.outlineVariant),
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
                    ? Theme.of(context).colorScheme.secondary
                    : (Theme.of(context).colorScheme.outlineVariant),
                shape: BoxShape.circle,
              ),
              child: Icon(
                _getIconForReason(reason),
                color: isSelected
                    ? Theme.of(context).colorScheme.onPrimary
                    : (Theme.of(context).colorScheme.onSurfaceVariant),
                size: AppIconSize.action,
              ),
            ),
            const SizedBox(height: 8),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p8),
              child: Text(
                reason.displayName,
                style: context.typeRoles.labelMicro.copyWith(
                  fontWeight: isSelected ? FontWeight.w600 : FontWeight.w500,
                  color: isSelected
                      ? Theme.of(context).colorScheme.secondary
                      : (Theme.of(context).colorScheme.onSurface),
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
      case ReportReasonType.scamOrFraud:
        return Icons.warning_outlined;
      case ReportReasonType.prohibitedContent:
        return Icons.block_outlined;
      case ReportReasonType.harassmentOrAbuse:
        return Icons.person_off_outlined;
      case ReportReasonType.impersonation:
        return Icons.badge_outlined;
      case ReportReasonType.misleadingInformation:
        return Icons.help_outline;
      case ReportReasonType.commerceViolation:
        return Icons.shopping_bag_outlined;
      case ReportReasonType.other:
        return Icons.more_horiz;
    }
  }
}
