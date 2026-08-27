/// Payment Method Selector Bottom Sheet
///
/// Simplified version for seller upgrade wizard.
/// Uses PaymentChannel enum directly for compatibility.
library;

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

/// Popular payment channels for seller subscription
/// Ordered by priority (most popular first)
const _popularPaymentChannels = [
  // E-Wallets (instant)
  PaymentChannel.qris,
  PaymentChannel.gopay,
  PaymentChannel.shopeepay,
  PaymentChannel.dana,
  PaymentChannel.ovo,
  // Bank Transfer (VA)
  PaymentChannel.bcaVa,
  PaymentChannel.mandiriVa,
  PaymentChannel.bniVa,
  PaymentChannel.briVa,
  // Convenience stores
  PaymentChannel.alfamart,
  PaymentChannel.indomaret,
];

/// Bottom sheet for selecting payment method
class PaymentMethodSelectorSheet extends StatefulWidget {
  final double totalAmount;
  final PaymentChannel? initialMethod;

  const PaymentMethodSelectorSheet({
    super.key,
    required this.totalAmount,
    this.initialMethod,
  });

  /// Show the payment method selector and return selected method
  static Future<PaymentChannel?> show(
    BuildContext context, {
    required double totalAmount,
    PaymentChannel? initialMethod,
  }) {
    return showModalBottomSheet<PaymentChannel>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (ctx) => PaymentMethodSelectorSheet(
        totalAmount: totalAmount,
        initialMethod: initialMethod,
      ),
    );
  }

  @override
  State<PaymentMethodSelectorSheet> createState() =>
      _PaymentMethodSelectorSheetState();
}

class _PaymentMethodSelectorSheetState
    extends State<PaymentMethodSelectorSheet> {
  PaymentChannel? _selectedMethod;

  @override
  void initState() {
    super.initState();
    _selectedMethod = widget.initialMethod;
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return SafeArea(
      child: Container(
        constraints: BoxConstraints(
          maxHeight: MediaQuery.of(context).size.height * 0.75,
        ),
        decoration: BoxDecoration(
          color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
          borderRadius: const BorderRadius.vertical(top: Radius.circular(20)),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Handle bar
            Container(
              margin: const EdgeInsets.only(top: 12),
              width: 40,
              height: 4,
              decoration: BoxDecoration(
                color: isDark
                    ? AppColors.neutralGray600
                    : AppColors.neutralGray300,
                borderRadius: BorderRadius.circular(2),
              ),
            ),

            // Header
            Padding(
              padding: const EdgeInsets.all(16),
              child: Row(
                children: [
                  Text(
                    'Pilih Metode Pembayaran',
                    style: TextStyle(
                      fontSize: 18,
                      fontWeight: FontWeight.bold,
                      color: isDark
                          ? AppColors.neutralWhite
                          : AppColors.neutralGray900,
                    ),
                  ),
                  const Spacer(),
                  IconButton(
                    icon: const Icon(Icons.close),
                    onPressed: () => Navigator.of(context).pop(),
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray600,
                  ),
                ],
              ),
            ),

            const Divider(height: 1),

            // Amount display
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(16),
              color: isDark ? AppColors.darkGray700 : AppColors.neutralGray50,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Total Pembayaran',
                    style: TextStyle(
                      fontSize: 12,
                      color: isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray600,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    'Rp ${AppFormatters.formatCurrency(widget.totalAmount)}',
                    style: TextStyle(
                      fontSize: 20,
                      fontWeight: FontWeight.bold,
                      color: isDark
                          ? AppColors.neutralWhite
                          : AppColors.neutralGray900,
                    ),
                  ),
                ],
              ),
            ),

            const Divider(height: 1),

            // Payment methods list
            Flexible(
              child: ListView.separated(
                padding: const EdgeInsets.symmetric(vertical: 8),
                itemCount: _popularPaymentChannels.length,
                separatorBuilder: (_, _) => const SizedBox(height: 4),
                itemBuilder: (context, index) {
                  final channel = _popularPaymentChannels[index];
                  final isSelected = _selectedMethod == channel;

                  return _PaymentMethodTile(
                    channel: channel,
                    isSelected: isSelected,
                    onTap: () => setState(() => _selectedMethod = channel),
                  );
                },
              ),
            ),

            // Bottom button
            Container(
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
                border: Border(
                  top: BorderSide(
                    color: isDark
                        ? AppColors.neutralGray700
                        : AppColors.neutralGray200,
                  ),
                ),
              ),
              child: SizedBox(
                width: double.infinity,
                child: ElevatedButton(
                  onPressed: _selectedMethod != null
                      ? () => Navigator.of(context).pop(_selectedMethod)
                      : null,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppColors.primaryRed,
                    foregroundColor: AppColors.neutralWhite,
                    disabledBackgroundColor: isDark
                        ? AppColors.neutralGray700
                        : AppColors.neutralGray300,
                    padding: const EdgeInsets.symmetric(vertical: 14),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(8),
                    ),
                  ),
                  child: const Text(
                    'Pilih Metode Pembayaran',
                    style: TextStyle(fontSize: 15, fontWeight: FontWeight.w600),
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Payment method tile widget
class _PaymentMethodTile extends StatelessWidget {
  final PaymentChannel channel;
  final bool isSelected;
  final VoidCallback onTap;

  const _PaymentMethodTile({
    required this.channel,
    required this.isSelected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return InkWell(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        color: isSelected
            ? (isDark
                  ? AppColors.primaryRed.withValues(alpha: 0.1)
                  : AppColors.primaryRed.withValues(alpha: 0.08))
            : Colors.transparent,
        child: Row(
          children: [
            // Icon
            Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                color: _getChannelColor(channel).withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(8),
              ),
              child: Icon(
                _getChannelIcon(channel),
                color: _getChannelColor(channel),
                size: 20,
              ),
            ),
            const SizedBox(width: 12),

            // Name
            Expanded(
              child: Text(
                channel.displayName,
                style: TextStyle(
                  fontSize: 15,
                  fontWeight: isSelected ? FontWeight.w600 : FontWeight.normal,
                  color: isDark
                      ? AppColors.neutralWhite
                      : AppColors.neutralGray900,
                ),
              ),
            ),

            // Selection indicator
            if (isSelected)
              Icon(Icons.check_circle, color: AppColors.primaryRed, size: 20)
            else
              Icon(
                Icons.radio_button_unchecked,
                color: isDark
                    ? AppColors.neutralGray600
                    : AppColors.neutralGray400,
                size: 20,
              ),
          ],
        ),
      ),
    );
  }

  IconData _getChannelIcon(PaymentChannel channel) {
    return switch (channel.type) {
      PaymentMethodType.eWallet => Icons.account_balance_wallet,
      PaymentMethodType.qris => Icons.qr_code_2,
      PaymentMethodType.bankTransfer => Icons.receipt_long,
      PaymentMethodType.manualTransfer => Icons.store,
      _ => Icons.payment,
    };
  }

  Color _getChannelColor(PaymentChannel channel) {
    return switch (channel) {
      PaymentChannel.qris => Colors.black,
      PaymentChannel.gopay => Colors.green,
      PaymentChannel.dana => Colors.blue,
      PaymentChannel.ovo => Colors.purple,
      PaymentChannel.linkAja => Colors.red,
      PaymentChannel.shopeepay => Colors.orange,
      PaymentChannel.bcaVa => Colors.blue,
      PaymentChannel.mandiriVa => const Color(0xFFffc107),
      PaymentChannel.bniVa => Colors.orange,
      PaymentChannel.briVa => Colors.blue.shade800,
      PaymentChannel.permataVa => Colors.purple,
      PaymentChannel.cimbVa => Colors.red.shade700,
      PaymentChannel.bsiVa => const Color(0xFFffc107),
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
}
