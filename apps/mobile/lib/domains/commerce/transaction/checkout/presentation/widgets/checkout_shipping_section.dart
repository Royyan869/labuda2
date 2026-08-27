part of '../screens/checkout_screen_impl.dart';

/// Shipping Option Picker Section — selects standard shipping option for direct buy
class _ShippingOptionPickerSection extends StatelessWidget {
  final List<DeliveryOption> deliveryOptions;
  final String? selectedOptionId;
  final bool isLoading;
  final bool hasAddress;
  final ValueChanged<String> onSelected;

  const _ShippingOptionPickerSection({
    required this.deliveryOptions,
    required this.selectedOptionId,
    required this.isLoading,
    required this.hasAddress,
    required this.onSelected,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: AppColors.neutralGray200),
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
            const Text(
              'Pilih alamat pengiriman terlebih dahulu',
              style: TextStyle(color: AppColors.neutralGray500),
            )
          else if (isLoading)
            const Center(child: CircularProgressIndicator())
          else if (deliveryOptions.isEmpty)
            const Text(
              'Tidak ada opsi pengiriman tersedia',
              style: TextStyle(color: AppColors.neutralGray500),
            )
          else
            RadioGroup<String>(
              groupValue: selectedOptionId,
              onChanged: (v) => onSelected(v!),
              child: Column(
                children: deliveryOptions
                    .map(
                      (option) => RadioListTile<String>(
                        value: option.shippingOptionId,
                        title: Text(option.displayName),
                        subtitle: Text(
                          AppFormatters.formatCurrency(option.rate),
                        ),
                        activeColor: AppColors.primaryRed,
                        selected: option.shippingOptionId == selectedOptionId,
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
