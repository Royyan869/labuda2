import 'package:flutter/material.dart';
import 'package:hishumi/core/core.dart';

class BankAccountEmptyStateWidget extends StatelessWidget {
  final VoidCallback onAddAccount;

  const BankAccountEmptyStateWidget({super.key, required this.onAddAccount});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p32),
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r16),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Column(
        children: [
          Container(
            padding: const EdgeInsets.all(AppMetrics.p24),
            decoration: BoxDecoration(
              color: scheme.surfaceContainerHighest,
              shape: BoxShape.circle,
            ),
            child: Icon(
              Icons.account_balance,
              size: AppIconSize.display,
              color: scheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 24),
          Text(
            'No Bank Account Yet',
            style: context.typeRoles.titleSection.copyWith(
              color: scheme.onSurface,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Add your bank account to receive payments from your sales.',
            textAlign: TextAlign.center,
            style: context.typeRoles.bodyDense.copyWith(
              color: scheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 24),
          SizedBox(
            width: double.infinity,
            child: ElevatedButton(
              onPressed: onAddAccount,
              child: const Text('Add Bank Account'),
            ),
          ),
          const SizedBox(height: 16),
          Container(
            padding: const EdgeInsets.all(AppMetrics.p12),
            decoration: BoxDecoration(
              color: context.statusColors.info.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(AppShape.r8),
              border: Border.all(
                color: context.statusColors.info.withValues(alpha: 0.3),
              ),
            ),
            child: Row(
              children: [
                Icon(
                  Icons.info_outline,
                  color: context.statusColors.info,
                  size: AppIconSize.inlineGlyph,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    'Your bank account information is encrypted and secure.',
                    style: context.typeRoles.labelMicro.copyWith(
                      color: scheme.onSurface,
                    ),
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
