/// Shipping quote input sheet — the single form authority for the seller's
/// manual shipping quote (ongkir + packing, all-in).
///
/// DOMAIN AUTHORITY: Shipping (commerce). Chat never renders or owns this
/// form — it forwards the seller's intent here, exactly like checkout is
/// forwarded through openForSaleCheckout. The wire contract is
/// `POST /api/v1/chat/:chat_id/shipping-quote` (backend authoritative:
/// expiry defaults to 24h, max 7 days; one ACTIVE quote per context, prior
/// revisions are superseded server-side).
library;

import 'package:flutter/material.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/models/wilayah_models.dart';
import 'package:hishumi/shared/utils/money_input_formatter.dart';
import 'package:hishumi/shared/widgets/app_bottom_sheet_base.dart';
import 'package:hishumi/shared/widgets/app_text_field.dart';
import 'package:hishumi/shared/widgets/wilayah/city_dropdown.dart';
import 'package:hishumi/shared/widgets/wilayah/province_dropdown.dart';

/// What the seller submitted: all-in cost (smallest currency unit), the
/// MANDATORY kota/kabupaten destination lock, and an optional note shown to
/// the buyer on the quote card.
typedef ShippingQuoteFormResult = ({
  int cost,
  String? note,
  String destinationCityId,
  String destinationProvinceId,
});

class ShippingQuoteFormSheet extends StatefulWidget {
  const ShippingQuoteFormSheet({super.key, required this.productTitle});

  final String productTitle;

  /// Opens the sheet. Returns the submitted value, or `null` when dismissed.
  static Future<ShippingQuoteFormResult?> show({
    required BuildContext context,
    required String productTitle,
  }) {
    return AppBottomSheetBase.show<ShippingQuoteFormResult>(
      context: context,
      title: 'Kirim Penawaran Ongkir',
      content: ShippingQuoteFormSheet(productTitle: productTitle),
    );
  }

  @override
  State<ShippingQuoteFormSheet> createState() => _ShippingQuoteFormSheetState();
}

class _ShippingQuoteFormSheetState extends State<ShippingQuoteFormSheet> {
  final _formKey = GlobalKey<FormState>();
  final _costController = TextEditingController();
  final _noteController = TextEditingController();

  Province? _selectedProvince;
  City? _selectedCity;

  @override
  void dispose() {
    _costController.dispose();
    _noteController.dispose();
    super.dispose();
  }

  void _submit() {
    // Field-level validation is INLINE (the cost field, ProvinceDropdown and
    // CityDropdown all carry validators); a failed validate() paints those
    // errors, so no Snackbar is needed here.
    if (!(_formKey.currentState?.validate() ?? false)) return;
    final cost = MoneyInputFormatter.parseAmount(_costController.text) ?? -1;
    if (cost <= 0) return;
    // DESTINATION LOCK is mandatory: without a locked kota/kabupaten the
    // quote would be consumable by ANY buyer address (backend rejects it).
    final province = _selectedProvince;
    final city = _selectedCity;
    if (province == null || city == null) return;
    final note = _noteController.text.trim();
    Navigator.of(context).pop((
      cost: cost,
      note: note.isEmpty ? null : note,
      destinationCityId: city.id,
      destinationProvinceId: province.id,
    ));
  }

  @override
  Widget build(BuildContext context) {
    return Form(
      key: _formKey,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            widget.productTitle,
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
            const SizedBox(height: AppMetrics.p12),
            AppTextField(
              controller: _costController,
              keyboardType: TextInputType.number,
              inputFormatters: const [MoneyInputFormatter()],
              labelText: 'Ongkir + Packing (Rp) *',
              hintText: 'Contoh: 25000',
              validator: (value) {
                final parsed = MoneyInputFormatter.parseAmount(value ?? '');
                if (parsed == null || parsed <= 0) {
                  return 'Masukkan nominal ongkir yang valid.';
                }
                return null;
              },
            ),
            const SizedBox(height: AppMetrics.p12),
            // DESTINATION LOCK (kota/kabupaten level only — never kecamatan
            // or desa): the quote only ever applies to a buyer whose address
            // city matches this selection.
            ProvinceDropdown(
              selectedProvince: _selectedProvince,
              onChanged: (province) {
                setState(() {
                  _selectedProvince = province;
                  _selectedCity = null; // cascade: cities depend on province
                });
              },
              labelText: 'Provinsi Tujuan *',
              validator: (value) =>
                  value == null ? 'Pilih provinsi tujuan.' : null,
            ),
            const SizedBox(height: AppMetrics.p12),
            CityDropdown(
              selectedCity: _selectedCity,
              selectedProvince: _selectedProvince,
              onChanged: (city) => setState(() => _selectedCity = city),
              labelText: 'Kota/Kabupaten Tujuan *',
              validator: (value) =>
                  value == null ? 'Pilih kota/kabupaten tujuan.' : null,
            ),
            const SizedBox(height: AppMetrics.p12),
            AppTextField(
              controller: _noteController,
              maxLines: 2,
              labelText: 'Catatan (opsional)',
              hintText: 'Contoh: termasuk kantong + oksigen',
            ),
            const SizedBox(height: AppMetrics.p12),
            // Backend truth surfaced to the seller: the quote lives 24h by
            // default (max 7 days) and replaces any previous active quote.
            Text(
              'Penawaran berlaku 24 jam dan otomatis menggantikan '
              'penawaran ongkir sebelumnya untuk pembeli ini. '
              'Ongkir hanya berlaku untuk alamat tujuan di '
              'kota/kabupaten yang dipilih.',
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    fontStyle: FontStyle.italic,
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
            ),
            const SizedBox(height: AppMetrics.p12),
            FilledButton.icon(
              onPressed: _submit,
              icon: const Icon(Icons.local_shipping_outlined),
              label: const Text('Kirim ke Pembeli'),
            ),
          ],
        ),
    );
  }
}
