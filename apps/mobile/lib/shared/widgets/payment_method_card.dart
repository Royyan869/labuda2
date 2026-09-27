import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/widgets/base_card.dart';

/// Payment Method Selection Card
///
/// Reusable card for selecting payment methods.
class PaymentMethodCard extends StatelessWidget {
  final String label;
  final String? badge;
  final IconData icon;
  final VoidCallback? onTap;
  final bool isSelected;
  final String? subtitle;

  const PaymentMethodCard({
    super.key,
    required this.label,
    this.badge,
    required this.icon,
    this.onTap,
    this.isSelected = false,
    this.subtitle,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return BaseCard(
      onTap: onTap,
      padding: const EdgeInsets.all(16),
      showBorder: isSelected,
      borderColor: isSelected ? scheme.primary : null,
      child: Row(
        children: [
          // Icon
          Container(
            width: 48,
            height: 48,
            decoration: BoxDecoration(
              color: isSelected
                  ? scheme.primary.withValues(alpha: 0.1)
                  : scheme.surfaceContainerHighest,
              borderRadius: BorderRadius.circular(12),
            ),
            child: Icon(
              icon,
              size: 24,
              color: isSelected
                  ? scheme.primary
                  : scheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(width: 12),

          // Label, badge, and subtitle
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        label,
                        style: AppTypography.labelMedium.copyWith(
                          color: isSelected
                              ? scheme.primary
                              : scheme.onSurface,
                        ),
                      ),
                    ),
                    if (badge != null) ...[
                      const SizedBox(width: 8),
                      Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: 8,
                          vertical: 4,
                        ),
                        decoration: BoxDecoration(
                          color: scheme.primary.withValues(alpha: 0.1),
                          borderRadius: BorderRadius.circular(999),
                        ),
                        child: Text(
                          badge!,
                          style: AppTypography.labelSmall.copyWith(
                            color: scheme.primary,
                          ),
                        ),
                      ),
                    ],
                  ],
                ),
                if (subtitle != null)
                  Text(
                    subtitle!,
                    style: AppTypography.caption.copyWith(
                      color: scheme.onSurfaceVariant,
                    ),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
