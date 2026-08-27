import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:labuda/core/src/theme/app_colors.dart';

/// Shipping Quote Creation Modal
///
/// Modal dialog for sellers to create shipping quotes for buyers.
/// Allows inputting shipping cost and optional notes.
class ShippingQuoteCreationModal extends StatefulWidget {
  final String listingName;
  final Function(int cost, String? note) onCreate;

  const ShippingQuoteCreationModal({
    super.key,
    required this.listingName,
    required this.onCreate,
  });

  @override
  State<ShippingQuoteCreationModal> createState() =>
      _ShippingQuoteCreationModalState();
}

class _ShippingQuoteCreationModalState
    extends State<ShippingQuoteCreationModal> {
  final _formKey = GlobalKey<FormState>();
  final _costController = TextEditingController();
  final _noteController = TextEditingController();
  bool _isSubmitting = false;

  @override
  void dispose() {
    _costController.dispose();
    _noteController.dispose();
    super.dispose();
  }

  Future<void> _handleSubmit() async {
    if (!_formKey.currentState!.validate()) return;

    setState(() {
      _isSubmitting = true;
    });

    try {
      final costText = _costController.text.replaceAll('.', '');
      final cost = int.tryParse(costText);
      if (cost == null) return;

      final note = _noteController.text.trim().isEmpty
          ? null
          : _noteController.text.trim();

      await widget.onCreate(cost, note);

      if (mounted) {
        Navigator.of(context).pop();
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Ongkir berhasil dikirim'),
            backgroundColor: AppColors.successGreen,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _isSubmitting = false;
        });
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: const Text('Gagal mengirim ongkir. Coba lagi.'),
            backgroundColor: AppColors.statusError,
          ),
        );
      }
    }
  }

  String _formatCurrency(String value) {
    final cleanValue = value.replaceAll('.', '');
    if (cleanValue.isEmpty) return '';

    final amount = int.tryParse(cleanValue);
    if (amount == null) return value;

    return amount.toString().replaceAllMapped(
      RegExp(r'(\d{1,3})(?=(\d{3})+(?!\d))'),
      (Match m) => '${m[1]}.',
    );
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Row(
        children: [
          Icon(
            Icons.local_shipping_outlined,
            color: AppColors.primaryRed,
            size: 24,
          ),
          const SizedBox(width: 8),
          const Text('Buat Ongkir'),
        ],
      ),
      content: Form(
        key: _formKey,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Listing info
            Container(
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: AppColors.primaryRed.withValues(alpha: 0.08),
                borderRadius: BorderRadius.circular(8),
                border: Border.all(
                  color: AppColors.primaryRed.withValues(alpha: 0.25),
                  width: 1,
                ),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.storefront_outlined,
                    size: 16,
                    color: AppColors.primaryRed,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      widget.listingName,
                      style: const TextStyle(
                        fontSize: 12,
                        fontWeight: FontWeight.w500,
                        color: AppColors.neutralGray900,
                      ),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 16),

            // Cost input
            Text(
              'Biaya Ongkir',
              style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: AppColors.neutralGray900,
              ),
            ),
            const SizedBox(height: 8),
            TextFormField(
              controller: _costController,
              keyboardType: TextInputType.number,
              inputFormatters: [FilteringTextInputFormatter.digitsOnly],
              onChanged: (value) {
                final formatted = _formatCurrency(value);
                if (formatted != value) {
                  _costController.value = TextEditingValue(
                    text: formatted,
                    selection: TextSelection.collapsed(
                      offset: formatted.length,
                    ),
                  );
                }
              },
              decoration: InputDecoration(
                labelText: 'Rp',
                prefixText: 'Rp ',
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(8),
                ),
                filled: true,
                fillColor: AppColors.neutralGray50,
              ),
              validator: (value) {
                if (value == null || value.trim().isEmpty) {
                  return 'Masukkan biaya ongkir';
                }
                final costText = value.replaceAll('.', '');
                final cost = int.tryParse(costText);
                if (cost == null) {
                  return 'Biaya tidak valid';
                }
                if (cost <= 0) {
                  return 'Biaya harus lebih dari 0';
                }
                if (cost > 100000000) {
                  return 'Biaya terlalu besar';
                }
                return null;
              },
            ),
            const SizedBox(height: 16),

            // Note input (optional)
            Text(
              'Catatan (opsional)',
              style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: AppColors.neutralGray900,
              ),
            ),
            const SizedBox(height: 8),
            TextFormField(
              controller: _noteController,
              maxLines: 3,
              decoration: InputDecoration(
                hintText: 'Contoh: Ongkir via JNE reguler',
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(8),
                ),
                filled: true,
                fillColor: AppColors.neutralGray50,
              ),
            ),

            // Info hint
            const SizedBox(height: 12),
            Container(
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: AppColors.coinPrimary.withValues(alpha: 0.08),
                borderRadius: BorderRadius.circular(8),
                border: Border.all(
                  color: AppColors.coinPrimary.withValues(alpha: 0.25),
                  width: 1,
                ),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.info_outline,
                    size: 14,
                    color: AppColors.coinSecondary,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      'Ongkir yang Anda berikan akan dikirim ke pembeli dan dapat langsung digunakan untuk checkout.',
                      style: TextStyle(
                        fontSize: 11,
                        color: AppColors.neutralGray700,
                        height: 1.4,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: _isSubmitting ? null : () => Navigator.of(context).pop(),
          child: const Text('Batal'),
        ),
        ElevatedButton(
          onPressed: _isSubmitting ? null : _handleSubmit,
          style: ElevatedButton.styleFrom(
            backgroundColor: AppColors.primaryRed,
            foregroundColor: Colors.white,
          ),
          child: _isSubmitting
              ? const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    valueColor: AlwaysStoppedAnimation<Color>(Colors.white),
                  ),
                )
              : const Text('Kirim Ongkir'),
        ),
      ],
    );
  }
}
