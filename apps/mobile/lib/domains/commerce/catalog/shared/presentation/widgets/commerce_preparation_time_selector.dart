import 'package:flutter/material.dart';
import 'package:hishumi/core/common/types/preparation_time.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

/// Preparation-time selector — ONE authority for choosing when the seller can
/// ship after checkout (owner vocabulary: 1–3 / 4–7 / 8–15 days, default 1–3).
///
/// Shared by create ForSale and create Auction. Private copies of this form
/// control are forbidden (duplicated design → zombie divergence).
class CommercePreparationTimeSelector extends StatelessWidget {
  final PreparationTime selected;
  final ValueChanged<PreparationTime> onChanged;

  const CommercePreparationTimeSelector({
    super.key,
    required this.selected,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Waktu Persiapan *',
            style: context.typeRoles.bodyDense.copyWith(
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 12),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: PreparationTime.values.map((time) {
              final isSelected = selected == time;
              return InkWell(
                onTap: () => onChanged(time),
                borderRadius: BorderRadius.circular(AppShape.r20),
                child: Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppMetrics.p16,
                    vertical: AppMetrics.p12,
                  ),
                  decoration: BoxDecoration(
                    color: isSelected ? scheme.primary : scheme.surface,
                    borderRadius: BorderRadius.circular(AppShape.r20),
                    border: Border.all(
                      color: isSelected
                          ? scheme.primary
                          : scheme.outlineVariant,
                    ),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      if (isSelected)
                        Icon(
                          Icons.check_circle,
                          size: AppIconSize.inlineGlyph,
                          color: scheme.onPrimary,
                        )
                      else
                        Icon(
                          Icons.radio_button_unchecked,
                          size: AppIconSize.inlineGlyph,
                          color: scheme.onSurfaceVariant,
                        ),
                      const SizedBox(width: 6),
                      Text(
                        time.displayName,
                        style: Theme.of(context).textTheme.labelLarge?.copyWith(
                          color: isSelected
                              ? scheme.onPrimary
                              : scheme.onSurface,
                          fontWeight: isSelected
                              ? FontWeight.w600
                              : FontWeight.normal,
                        ),
                      ),
                    ],
                  ),
                ),
              );
            }).toList(),
          ),
          const SizedBox(height: 8),
          Text(
            selected.description,
            style: context.typeRoles.bodyDense.copyWith(
              color: scheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }
}
