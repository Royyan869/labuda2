/// Payment Method Tile Widget
///
/// Displays a payment method option for selection.
library;

import 'package:flutter/material.dart';
import 'package:labuda/core/common/types/payment_types.dart'
    show PaymentChannel;
import '../../domain/entities/payment_method.dart';

/// Payment Method Tile
class PaymentMethodTile extends StatelessWidget {
  final PaymentMethod paymentMethod;
  final bool isSelected;
  final VoidCallback? onTap;
  final bool showFee;

  const PaymentMethodTile({
    super.key,
    required this.paymentMethod,
    this.isSelected = false,
    this.onTap,
    this.showFee = true,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Row(
            children: [
              // Payment Method Icon
              _buildIcon(context),
              const SizedBox(width: 12),
              // Payment Method Info
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      paymentMethod.displayName,
                      style: theme.textTheme.titleSmall?.copyWith(
                        fontWeight: isSelected
                            ? FontWeight.bold
                            : FontWeight.normal,
                      ),
                    ),
                    if (showFee) ...[
                      const SizedBox(height: 4),
                      Text(
                        'Biaya: ${_formatCurrency(paymentMethod.fee.flatFee)}'
                        '${paymentMethod.fee.percentageFee > 0 ? ' + ${paymentMethod.fee.percentageFee.toStringAsFixed(1)}%' : ''}',
                        style: theme.textTheme.bodySmall,
                      ),
                    ],
                  ],
                ),
              ),
              // Selection Indicator
              if (isSelected)
                Icon(Icons.check_circle, color: theme.colorScheme.primary),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildIcon(BuildContext context) {
    final channel = paymentMethod.channel;
    final iconData = _getIconForChannel(channel);
    final color = _getColorForChannel(channel);

    return Container(
      width: 48,
      height: 48,
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Icon(iconData, color: color, size: 24),
    );
  }

  IconData _getIconForChannel(PaymentChannel channel) {
    return switch (channel) {
      PaymentChannel.qris => Icons.qr_code_2,
      PaymentChannel.gopay => Icons.account_balance_wallet,
      PaymentChannel.dana => Icons.account_balance_wallet,
      PaymentChannel.ovo => Icons.account_balance_wallet,
      PaymentChannel.linkAja => Icons.account_balance_wallet,
      PaymentChannel.shopeepay => Icons.account_balance_wallet,
      PaymentChannel.bcaVa => Icons.receipt_long,
      PaymentChannel.mandiriVa => Icons.receipt_long,
      PaymentChannel.bniVa => Icons.receipt_long,
      PaymentChannel.briVa => Icons.receipt_long,
      PaymentChannel.permataVa => Icons.receipt_long,
      PaymentChannel.cimbVa => Icons.receipt_long,
      PaymentChannel.bsiVa => Icons.receipt_long,
      PaymentChannel.danamonVa => Icons.receipt_long,
      PaymentChannel.maybankVa => Icons.receipt_long,
      PaymentChannel.btnVa => Icons.receipt_long,
      PaymentChannel.indomaret => Icons.store,
      PaymentChannel.alfamart => Icons.store,
      PaymentChannel.kredivo => Icons.account_balance_wallet,
      PaymentChannel.akulaku => Icons.account_balance_wallet,
      PaymentChannel.creditCard => Icons.credit_card,
      PaymentChannel.debitCard => Icons.credit_card,
    };
  }

  Color _getColorForChannel(PaymentChannel channel) {
    return switch (channel) {
      PaymentChannel.qris => Colors.black,
      PaymentChannel.gopay => Colors.green,
      PaymentChannel.dana => Colors.blue,
      PaymentChannel.ovo => Colors.purple,
      PaymentChannel.linkAja => Colors.red,
      PaymentChannel.shopeepay => Colors.orange,
      PaymentChannel.bcaVa => Colors.blue,
      PaymentChannel.mandiriVa => Colors.yellow.shade700,
      PaymentChannel.bniVa => Colors.orange,
      PaymentChannel.briVa => Colors.blue.shade800,
      PaymentChannel.permataVa => Colors.purple,
      PaymentChannel.cimbVa => Colors.red.shade700,
      PaymentChannel.bsiVa => Colors.yellow.shade700,
      PaymentChannel.danamonVa => Colors.blue,
      PaymentChannel.maybankVa => Colors.blue.shade800,
      PaymentChannel.btnVa => Colors.blue,
      PaymentChannel.indomaret => Colors.blue.shade900,
      PaymentChannel.alfamart => Colors.red.shade800,
      PaymentChannel.kredivo => Colors.orange.shade700,
      PaymentChannel.akulaku => Colors.green,
      PaymentChannel.creditCard => Colors.blue.shade700,
      PaymentChannel.debitCard => Colors.green.shade700,
    };
  }

  String _formatCurrency(double amount) {
    return 'Rp ${amount.toStringAsFixed(0).replaceAllMapped(RegExp(r'(\d{1,3})(?=(\d{3})+(?!\d))'), (Match m) => '${m[1]}.')}';
  }
}
