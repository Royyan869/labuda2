import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Stub widget for SettingsUpgradeCard
/// TODO: Implement seller upgrade flow
class SettingsUpgradeCard extends StatelessWidget {
  final VoidCallback onUpgrade;

  const SettingsUpgradeCard({super.key, required this.onUpgrade});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      margin: const EdgeInsets.fromLTRB(AppMetrics.p16, AppMetrics.p8, AppMetrics.p16, AppMetrics.p16),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [
            context.statusColors.success,
            context.statusColors.success,
          ],
        ),
        borderRadius: BorderRadius.circular(AppShape.r16),
        boxShadow: [
          BoxShadow(
            color: context.statusColors.success.withValues(alpha: 0.3),
            blurRadius: 12,
            offset: const Offset(0, 6),
          ),
        ],
      ),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: onUpgrade,
          borderRadius: BorderRadius.circular(AppShape.r16),
          child: Padding(
            padding: const EdgeInsets.all(AppMetrics.p24),
            child: Row(
              children: [
                Icon(
                  Icons.store_outlined,
                  color: scheme.onPrimary,
                  size: AppIconSize.emphasis,
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Become a Seller',
                        style: TextStyle(
                          color: scheme.onPrimary,
                          fontSize: AppType.s20,
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        'Start selling your koi products',
                        style: TextStyle(
                          color: scheme.onPrimary,
                          fontSize: AppType.s14,
                          fontWeight: FontWeight.w500,
                        ),
                      ),
                    ],
                  ),
                ),
                Icon(
                  Icons.arrow_forward_ios,
                  color: scheme.onPrimary,
                  size: AppIconSize.action,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
