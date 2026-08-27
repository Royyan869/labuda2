import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Dialog showing address information and rules
class AddressInfoDialog extends StatelessWidget {
  final bool isDark;

  const AddressInfoDialog({super.key, required this.isDark});

  /// Show the address info dialog
  static void show(BuildContext context, bool isDark) {
    showDialog(
      context: context,
      builder: (context) => AddressInfoDialog(isDark: isDark),
    );
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      backgroundColor: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      title: Row(
        children: [
          Icon(Icons.info_outline, color: AppColors.primaryRed, size: 24),
          const SizedBox(width: 12),
          Text(
            'Address Information',
            style: TextStyle(
              fontSize: 18,
              fontWeight: FontWeight.bold,
              color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
            ),
          ),
        ],
      ),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _buildInfoRow(
            Icons.home,
            'Shipping Address',
            'Address for receiving packages/shipments',
          ),
          const SizedBox(height: 12),
          _buildInfoRow(
            Icons.agriculture,
            'Sender Address',
            'Origin address for goods (for seller)',
          ),
          const SizedBox(height: 16),
          _buildRulesBox(),
        ],
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: Text(
            'Got it',
            style: TextStyle(
              color: AppColors.primaryRed,
              fontWeight: FontWeight.w600,
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildInfoRow(IconData icon, String title, String desc) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(
          icon,
          size: 20,
          color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
        ),
        const SizedBox(width: 10),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                title,
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w600,
                  color: isDark
                      ? AppColors.neutralWhite
                      : AppColors.neutralGray900,
                ),
              ),
              Text(
                desc,
                style: TextStyle(
                  fontSize: 12,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildRulesBox() {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: AppColors.primaryRed.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          Icon(Icons.rule, color: AppColors.primaryRed, size: 20),
          const SizedBox(width: 10),
          Expanded(
            child: Text(
              'Min. 1 address per category\nMax. 10 addresses per category',
              style: TextStyle(
                fontSize: 13,
                color: isDark
                    ? AppColors.neutralGray200
                    : AppColors.neutralGray800,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
