import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Dialog showing address information and rules
class AddressInfoDialog extends StatelessWidget {
  final ColorScheme scheme;

  const AddressInfoDialog({super.key, required this.scheme});

  /// Show the address info dialog
  static void show(BuildContext context, ColorScheme scheme) {
    showDialog(
      context: context,
      builder: (context) => AddressInfoDialog(scheme: scheme),
    );
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return AlertDialog(
      backgroundColor: scheme.surface,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppShape.r16)),
      title: Row(
        children: [
          Icon(Icons.info_outline, color: scheme.primary, size: 24),
          const SizedBox(width: 12),
          Text(
            'Address Information',
            style: TextStyle(
              fontSize: AppType.s18,
              fontWeight: FontWeight.bold,
              color: scheme.onSurface,
            ),
          ),
        ],
      ),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
_buildInfoRow(
             scheme,
             Icons.home,
            'Shipping Address',
            'Address for receiving packages/shipments',
          ),
          const SizedBox(height: 12),
_buildInfoRow(
             scheme,
             Icons.agriculture,
            'Sender Address',
            'Origin address for goods (for seller)',
          ),
          const SizedBox(height: 16),
          _buildRulesBox(scheme),
        ],
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: Text(
            'Got it',
            style: TextStyle(
              color: scheme.primary,
              fontWeight: FontWeight.w600,
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildInfoRow(ColorScheme scheme, IconData icon, String title, String desc) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(
          icon,
          size: 20,
          color: scheme.onSurfaceVariant,
        ),
        const SizedBox(width: 10),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                title,
                style: TextStyle(
                  fontSize: AppType.s14,
                  fontWeight: FontWeight.w600,
                  color: scheme.onSurfaceVariant,
                ),
              ),
              Text(
                desc,
                style: TextStyle(
                  fontSize: AppType.s12,
                  color: scheme.onSurfaceVariant,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildRulesBox(ColorScheme scheme) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: scheme.primary.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(AppShape.r8),
      ),
      child: Row(
        children: [
          Icon(Icons.rule, color: scheme.primary, size: 20),
          const SizedBox(width: 10),
          Expanded(
            child: Text(
              'Min. 1 address per category\nMax. 10 addresses per category',
              style: TextStyle(
                fontSize: AppType.s13,
                color: scheme.onSurfaceVariant,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
