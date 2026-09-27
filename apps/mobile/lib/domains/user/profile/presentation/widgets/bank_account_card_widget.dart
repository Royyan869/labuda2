import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/user/profile/domain/entities/bank_account_entity.dart';

class BankAccountCardWidget extends StatelessWidget {
  final BankAccountEntity account;
  final VoidCallback onEdit;
  final VoidCallback onDelete;
  final VoidCallback onSetPrimary;

  const BankAccountCardWidget({
    super.key,
    required this.account,
    required this.onEdit,
    required this.onDelete,
    required this.onSetPrimary,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      margin: const EdgeInsets.only(bottom: 16),
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
         color: scheme.surface,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(
          color: account.isDefault
              ? AppColors.primaryRed.withValues(alpha: 0.3)
              : scheme.outlineVariant,
        ),
        gradient: account.isDefault
            ? LinearGradient(
                colors: [
                  AppColors.primaryRed.withValues(alpha: 0.05),
                  AppColors.primaryRed.withValues(alpha: 0.02),
                ],
              )
            : null,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header dengan bank info dan status
          Row(
            children: [
              // Bank icon
              Container(
                padding: const EdgeInsets.all(8),
                decoration: BoxDecoration(
                  color: AppColors.primaryRed.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Icon(
                  Icons.account_balance,
                  color: AppColors.primaryRed,
                  size: 20,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      account.bankName,
                      style: TextStyle(
                        color: scheme.onSurface,
                        fontSize: 16,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      account.isDefault ? 'Rekening Utama' : account.bankCode,
                      style: TextStyle(
                        color: scheme.onSurfaceVariant,
                        fontSize: 12,
                      ),
                    ),
                  ],
                ),
              ),
              _buildStatusBadge(context),
            ],
          ),
          const SizedBox(height: 16),

          // Account details
          _buildAccountDetails(context),
          const SizedBox(height: 16),

          // Action buttons
          _buildActionButtons(),
        ],
      ),
    );
  }

  Widget _buildStatusBadge(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    Color badgeColor;
    Color textColor;
    String statusText;
    IconData icon;

    switch (account.status) {
      case BankAccountStatus.active:
        badgeColor = AppColors.success;
        textColor = scheme.onPrimary;
        statusText = 'Aktif';
        icon = Icons.check_circle_outline;
        break;
      case BankAccountStatus.deleted:
        badgeColor = AppColors.error;
        textColor = scheme.onPrimary;
        statusText = 'Dihapus';
        icon = Icons.remove_circle_outline;
        break;
    }

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: badgeColor,
        borderRadius: BorderRadius.circular(6),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 12, color: textColor),
          const SizedBox(width: 4),
          Text(
            statusText,
            style: TextStyle(
              color: textColor,
              fontSize: 10,
              fontWeight: FontWeight.w600,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildAccountDetails(BuildContext context) {
    return Column(
      children: [
        _buildDetailRow(context, 'Account Number', account.accountNumber),
        const SizedBox(height: 8),
        _buildDetailRow(context, 'Account Holder', account.accountHolderName),
      ],
    );
  }

  Widget _buildDetailRow(BuildContext context, String label, String value) {
    final scheme = Theme.of(context).colorScheme;
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 120,
          child: Text(
            label,
            style: TextStyle(
              color: scheme.onSurfaceVariant,
              fontSize: 12,
              fontWeight: FontWeight.w500,
            ),
          ),
        ),
        const Text(': ', style: TextStyle(fontSize: 12)),
        Expanded(
          child: Text(
            value,
            style: TextStyle(
              color: scheme.onSurface,
              fontSize: 12,
              fontWeight: FontWeight.w500,
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildActionButtons() {
    return Row(
      children: [
        if (!account.isDefault)
          Expanded(
            child: AppButton.secondary(
              text: 'Set as Primary',
              onPressed: onSetPrimary,
            ),
          ),
        if (!account.isDefault) const SizedBox(width: 8),
        Expanded(
          child: AppButton.secondary(text: 'Edit', onPressed: onEdit),
        ),
        const SizedBox(width: 8),
        Expanded(
          child: AppButton.secondary(text: 'Delete', onPressed: onDelete),
        ),
      ],
    );
  }
}
