import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Reusable tooltip widget explaining discount management features
/// Shows info about Edit, Nonaktif, and Hapus
class DiscountManagementInfoTooltip extends StatelessWidget {
  const DiscountManagementInfoTooltip({super.key});

  void _showInfoDialog(BuildContext context) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Icon(
              Icons.info_outline,
              color: Theme.of(context).colorScheme.primary,
            ),
            const SizedBox(width: 8),
            const Expanded(
              child: Text('Discount Management Guide', softWrap: true),
            ),
          ],
        ),
        content: SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              _buildFeatureSection(
                context: context,
                icon: Icons.edit_outlined,
                iconColor: Theme.of(context).colorScheme.onSurfaceVariant,
                title: 'Edit',
                description:
                    'Change discount information that has been created.',
                rules: [
                  '• Discount never used: All fields can be changed',
                  '• Discount used: Only some fields can be changed (status, description, extend period, add limit)',
                ],
              ),
              const SizedBox(height: 16),
              _buildFeatureSection(
                context: context,
                icon: Icons.visibility_off_outlined,
                iconColor: Theme.of(context).colorScheme.onSurfaceVariant,
                title: 'Deactivate / Activate',
                description: 'Change active/inactive status of discount.',
                rules: [
                  '• Activate: Discount can be used by buyers',
                  '• Deactivate: Buyers cannot use this discount',
                  '• If discount has been used, warning will appear when deactivating',
                ],
              ),
              const SizedBox(height: 16),
              _buildFeatureSection(
                context: context,
                icon: Icons.delete_outline,
                iconColor: AppColors.error,
                title: 'Hapus',
                description: 'Permanently delete discount from system.',
                rules: [
                  '• Can only delete discounts that have never been used',
                  '• Used discounts cannot be deleted (for audit trail)',
                  '• This action cannot be undone',
                ],
              ),
              const SizedBox(height: 16),
              Container(
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: AppColors.warning.withValues(alpha: 0.1),
                  border: Border.all(color: AppColors.warning),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Icon(
                      Icons.lightbulb_outline,
                      color: AppColors.warning,
                      size: 20,
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        'Tip: Use "Deactivate" to stop discount temporarily, and "Delete" to clean up incorrectly input or testing discounts.',
                        style: TextStyle(
                          fontSize: 12,
                          color: AppColors.warning,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('Got It'),
          ),
        ],
      ),
    );
  }

  Widget _buildFeatureSection({
    required BuildContext context,
    required IconData icon,
    required Color iconColor,
    required String title,
    required String description,
    required List<String> rules,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Icon(icon, color: iconColor, size: 20),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                title,
                style: const TextStyle(
                  fontWeight: FontWeight.bold,
                  fontSize: 16,
                ),
                softWrap: true,
              ),
            ),
          ],
        ),
        const SizedBox(height: 8),
        Text(
          description,
          style: TextStyle(
            fontSize: 14,
            color: Theme.of(context).colorScheme.onSurfaceVariant,
          ),
          softWrap: true,
        ),
        const SizedBox(height: 8),
        ...rules.map(
          (rule) => Padding(
            padding: const EdgeInsets.only(left: 8, top: 4),
            child: Text(
              rule,
              style: TextStyle(
                fontSize: 13,
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
              softWrap: true,
            ),
          ),
        ),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    return IconButton(
      icon: const Icon(Icons.help_outline),
      tooltip: 'Panduan Kelola Diskon',
      onPressed: () => _showInfoDialog(context),
    );
  }
}
