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
import 'package:hishumi/shared/shared.dart';
import '../../domain/entities/payment.dart';

/// Payment method selection.
///
/// Presentation authority is the canonical selection family — this file owns
/// NO bottom-sheet renderer, no surface, no shape, no handle. The business
/// data (backend-computed fee + total) and the returned `methodCode` contract
/// are unchanged.
class PaymentMethodPickerSheet {
  PaymentMethodPickerSheet._();

  /// Opens the canonical payment-method selection sheet and returns the chosen
  /// `methodCode`, or null if the buyer dismissed it.
  static Future<String?> show(
    BuildContext context, {
    required List<PaymentMethodOption> methods,
    String? selectedMethodCode,
  }) {
    return AppBottomSheetListSelection.showListSelection<String>(
      context: context,
      title: 'Pilih Metode Pembayaran',
      selectedValue: selectedMethodCode,
      items: [
        for (final m in methods)
          ListSelectionItem<String>(
            title: m.displayName,
            subtitle: _feeLine(m),
            leading: PaymentMethodLogo(
              visual: PaymentMethodVisuals.visual(m.methodCode),
              size: 24,
            ),
            trailingText: AppFormatters.formatCurrency(
              m.totalPayableAmount.toDouble(),
            ),
            value: m.methodCode,
          ),
      ],
    );
  }

  /// The fee line for one method. The canonical currency formatter already
  /// carries the 'Rp ' symbol — never prefix it a second time.
  static String _feeLine(PaymentMethodOption m) {
    return m.buyerPaymentFeeAmount > 0
        ? 'Biaya layanan: '
              '${AppFormatters.formatCurrency(m.buyerPaymentFeeAmount.toDouble())}'
        : 'Tanpa biaya layanan';
  }
}
