part of '../screens/checkout_screen_impl.dart';

/// Shipping Option Picker Section — selects standard shipping option for direct buy
class _ShippingSetupPickerSection extends StatelessWidget {
  final List<DeliveryOption> deliveryOptions;
  final String? selectedOptionId;
  final bool isLoading;
  final bool hasAddress;
  final ValueChanged<String> onSelected;

  /// Canonical "contact seller" channel — the SAME `_openChatWithSeller`
  /// helper the uncovered-area dialog uses. The picker owns no chat logic of
  /// its own; null hides the CTA.
  final VoidCallback? onContactSeller;

  const _ShippingSetupPickerSection({
    required this.deliveryOptions,
    required this.selectedOptionId,
    required this.isLoading,
    required this.hasAddress,
    required this.onSelected,
    this.onContactSeller,
  });

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: colorScheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Opsi Pengiriman',
            style: context.typeRoles.titleSection.copyWith(
              fontWeight: FontWeight.bold,
            ),
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
            // Empty state is a DEAD END without an exit: the primary action
            // stays disabled (no shipping selection → no preview), so the
            // buyer must be able to reach the seller from here — not only
            // after a failed order creation.
            Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Tidak ada opsi pengiriman tersedia',
                  style: TextStyle(color: colorScheme.onSurfaceVariant),
                ),
                if (onContactSeller != null) ...[
                  const SizedBox(height: 8),
                  Text(
                    'Hubungi penjual untuk meminta opsi pengiriman.',
                    style: TextStyle(color: colorScheme.onSurfaceVariant),
                  ),
                  const SizedBox(height: 12),
                  ElevatedButton.icon(
                    onPressed: onContactSeller,
                    icon: const Icon(Icons.local_shipping_outlined),
                    label: const Text('Hubungi Penjual'),
                  ),
                ],
              ],
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
