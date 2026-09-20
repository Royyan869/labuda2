part of '../screens/checkout_screen_impl.dart';

/// Shipping Option Picker Section — selects standard shipping option for direct buy
class _ShippingSetupPickerSection extends StatelessWidget {
  final List<DeliveryOption> deliveryOptions;
  final String? selectedOptionId;
  final bool isLoading;
  final bool hasAddress;
  final ValueChanged<String> onSelected;

  const _ShippingSetupPickerSection({
    required this.deliveryOptions,
    required this.selectedOptionId,
    required this.isLoading,
    required this.hasAddress,
    required this.onSelected,
  });

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: colorScheme.surface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'Opsi Pengiriman',
            style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 12),
          if (!hasAddress)
            Text(
              'Pilih alamat pengiriman terlebih dahulu',
              style: TextStyle(color: colorScheme.onSurfaceVariant),
            )
          else if (isLoading)
            const Center(child: CircularProgressIndicator())
          else if (deliveryOptions.isEmpty)
            Text(
              'Tidak ada opsi pengiriman tersedia',
              style: TextStyle(color: colorScheme.onSurfaceVariant),
            )
          else
            RadioGroup<String>(
              groupValue: selectedOptionId,
              onChanged: (v) => onSelected(v!),
              child: Column(
                children: deliveryOptions
                    .map(
                      (option) => RadioListTile<String>(
                        value: option.shippingSetupId,
                        title: Text(option.displayName),
                        subtitle: Text(
                          AppFormatters.formatCurrency(option.rate),
                        ),
                        // Selection colour is the canonical theme default
                        // (`colorScheme.primary`); no local override.
                        selected: option.shippingSetupId == selectedOptionId,
                        contentPadding: EdgeInsets.zero,
                      ),
                    )
                    .toList(),
              ),
            ),
        ],
      ),
    );
  }
}
