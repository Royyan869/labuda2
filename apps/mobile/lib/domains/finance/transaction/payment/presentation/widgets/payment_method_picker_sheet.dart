/// Payment Method Picker Sheet (PASS_18V)
///
/// Shows the canonical payment methods for a specific order, each already
/// carrying the backend-calculated buyer payment fee and total. The buyer
/// picks one; the selected method_code is what gets sent to CreatePayment.
///
/// Backend is the sole fee authority — this widget only renders numbers the
/// backend already computed. It never calculates a fee itself.
library;

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import '../../domain/entities/payment.dart';

/// Shows the payment method picker and returns the selected method_code, or
/// null if the buyer dismissed the sheet.
class PaymentMethodPickerSheet extends StatelessWidget {
  final List<PaymentMethodOption> methods;

  const PaymentMethodPickerSheet({super.key, required this.methods});

  static Future<String?> show(
    BuildContext context, {
    required List<PaymentMethodOption> methods,
  }) {
    return showModalBottomSheet<String>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (_) => PaymentMethodPickerSheet(methods: methods),
    );
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
            Padding(
              padding: const EdgeInsets.all(16),
              child: Text(
                'Pilih Metode Pembayaran',
                style: TextStyle(
                  fontSize: 18,
                  fontWeight: FontWeight.bold,
                  color: isDark
                      ? AppColors.neutralWhite
                      : AppColors.neutralGray900,
                ),
              ),
            ),
            const Divider(height: 1),
            Flexible(
              child: methods.isEmpty
                  ? const Padding(
                      padding: EdgeInsets.all(24),
                      child: Text('Tidak ada metode pembayaran tersedia'),
                    )
                  : ListView.separated(
                      padding: const EdgeInsets.symmetric(vertical: 8),
                      itemCount: methods.length,
                      separatorBuilder: (_, _) => const Divider(height: 1),
                      itemBuilder: (context, index) {
                        final m = methods[index];
                        return ListTile(
                          title: Text(m.displayName),
                          subtitle: Text(
                            m.buyerPaymentFeeAmount > 0
                                ? 'Biaya layanan: Rp ${AppFormatters.formatCurrency(m.buyerPaymentFeeAmount.toDouble())}'
                                : 'Tanpa biaya layanan',
                          ),
                          trailing: Text(
                            'Rp ${AppFormatters.formatCurrency(m.totalPayableAmount.toDouble())}',
                            style: const TextStyle(fontWeight: FontWeight.w600),
                          ),
                          onTap: () => Navigator.of(context).pop(m.methodCode),
                        );
                      },
                    ),
            ),
          ],
        ),
      ),
    );
  }
}
