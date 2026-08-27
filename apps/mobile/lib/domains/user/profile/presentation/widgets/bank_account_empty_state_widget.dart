import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

class BankAccountEmptyStateWidget extends StatelessWidget {
  final VoidCallback onAddAccount;
  final bool isDark;

  const BankAccountEmptyStateWidget({
    super.key,
    required this.onAddAccount,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(32),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
        ),
      ),
      child: Column(
        children: [
          // Icon
          Container(
            padding: const EdgeInsets.all(20),
            decoration: BoxDecoration(
              color: isDark ? AppColors.darkGray600 : AppColors.neutralGray100,
              shape: BoxShape.circle,
            ),
            child: Icon(
              Icons.account_balance,
              size: 48,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
            ),
          ),
          const SizedBox(height: 24),

          // Title
          Text(
            'No Bank Account Yet',
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray200
                  : AppColors.neutralGray900,
              fontSize: 20,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 8),

          // Subtitle
          Text(
            'Add your bank account to receive payments from your sales.',
            textAlign: TextAlign.center,
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
              fontSize: 14,
            ),
          ),
          const SizedBox(height: 24),

          // Add account button
          SizedBox(
            width: double.infinity,
            child: AppButton.primary(
              text: 'Add Bank Account',
              onPressed: onAddAccount,
            ),
          ),

          const SizedBox(height: 16),

          // Info note
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: AppColors.statusInfo.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(8),
              border: Border.all(
                color: AppColors.statusInfo.withValues(alpha: 0.3),
              ),
            ),
            child: Row(
              children: [
                Icon(Icons.info_outline, color: AppColors.statusInfo, size: 16),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    'Your bank account information is encrypted and secure.',
                    style: TextStyle(
                      color: isDark
                          ? AppColors.neutralGray300
                          : AppColors.neutralGray700,
                      fontSize: 12,
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
