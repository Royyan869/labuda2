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
import 'package:labuda/shared/shared.dart';
import 'package:labuda/core/src/theme/app_theme.dart';
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
    final colorScheme = Theme.of(context).colorScheme;

    return SafeArea(
      child: Container(
        constraints: BoxConstraints(
          maxHeight: MediaQuery.of(context).size.height * 0.75,
        ),
        decoration: BoxDecoration(
          color: colorScheme.surface,
          borderRadius: const BorderRadius.vertical(top: Radius.circular(AppShape.r20)),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const AppDragHandle(padding: EdgeInsets.only(top: AppMetrics.p12)),
            Padding(
              padding: const EdgeInsets.all(AppMetrics.p16),
              child: Text(
                'Pilih Metode Pembayaran',
                style: TextStyle(
                  fontSize: AppType.s20,
                  fontWeight: FontWeight.bold,
                  color: colorScheme.onSurface,
                ),
              ),
            ),
            const Divider(height: 1),
            Flexible(
              child: methods.isEmpty
                  ? const Padding(
                      padding: EdgeInsets.all(AppMetrics.p24),
                      child: Text('Tidak ada metode pembayaran tersedia'),
                    )
                  : ListView.separated(
                      padding: const EdgeInsets.symmetric(vertical: AppMetrics.p8),
                      itemCount: methods.length,
                      separatorBuilder: (_, _) => const Divider(height: 1),
                      itemBuilder: (context, index) {
                        final m = methods[index];
                        return ListTile(
                          title: Text(m.displayName),
                          // The canonical currency formatter already carries the
                          // 'Rp ' symbol — never prefix it a second time.
                          subtitle: Text(
                            m.buyerPaymentFeeAmount > 0
                                ? 'Biaya layanan: '
                                      '${AppFormatters.formatCurrency(m.buyerPaymentFeeAmount.toDouble())}'
                                : 'Tanpa biaya layanan',
                          ),
                          trailing: Text(
                            AppFormatters.formatCurrency(
                              m.totalPayableAmount.toDouble(),
                            ),
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
