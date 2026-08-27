part of '../screens/checkout_screen_impl.dart';

/// Saved Address Picker Section — selects from user's saved shipping addresses
class _SavedAddressPickerSection extends ConsumerStatefulWidget {
  final String? selectedAddressId;
  final ValueChanged<AddressEntity> onAddressSelected;

  const _SavedAddressPickerSection({
    required this.selectedAddressId,
    required this.onAddressSelected,
  });

  @override
  ConsumerState<_SavedAddressPickerSection> createState() =>
      _SavedAddressPickerSectionState();
}

class _SavedAddressPickerSectionState
    extends ConsumerState<_SavedAddressPickerSection> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final authState = ref.read(authControllerProvider);
      if (authState is AuthStateAuthenticated) {
        ref
            .read(addressProvider.notifier)
            .loadAddressesByPurpose(authState.user.id, AddressPurpose.shipping);
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final addressesAsync = ref.watch(addressesListProvider);

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
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              const Text(
                'Alamat Pengiriman',
                style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
              ),
              TextButton.icon(
                onPressed: () => context.push(RoutePaths.addresses),
                icon: const Icon(Icons.add, size: 16),
                label: const Text('Kelola'),
                style: TextButton.styleFrom(
                  foregroundColor: AppColors.primaryRed,
                  padding: const EdgeInsets.symmetric(
                    horizontal: 8,
                    vertical: 4,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          addressesAsync.when(
            data: (addresses) {
              final shippingAddresses = addresses
                  .where((a) => a.purpose == AddressPurpose.shipping)
                  .toList();
              if (shippingAddresses.isEmpty) {
                return _EmptyAddressPrompt();
              }
              // Auto-select primary if nothing selected yet
              if (widget.selectedAddressId == null) {
                final primary = shippingAddresses
                    .where((a) => a.isPrimary)
                    .firstOrNull;
                if (primary != null) {
                  WidgetsBinding.instance.addPostFrameCallback((_) {
                    widget.onAddressSelected(primary);
                  });
                }
              }
              return Column(
                children: shippingAddresses.map((addr) {
                  final isSelected = addr.id == widget.selectedAddressId;
                  return _AddressCard(
                    address: addr,
                    isSelected: isSelected,
                    onTap: () => widget.onAddressSelected(addr),
                  );
                }).toList(),
              );
            },
            loading: () => const Center(
              child: Padding(
                padding: EdgeInsets.all(16),
                child: CircularProgressIndicator(),
              ),
            ),
            error: (e, _) => Padding(
              padding: const EdgeInsets.all(8),
              child: Text(
                'Gagal memuat alamat: $e',
                style: TextStyle(color: AppColors.statusError),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// Empty address prompt when no shipping addresses exist
class _EmptyAddressPrompt extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: AppColors.neutralGray100,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        children: [
          Icon(Icons.location_off, size: 32, color: AppColors.neutralGray400),
          const SizedBox(height: 8),
          const Text(
            'Belum ada alamat pengiriman',
            style: TextStyle(fontWeight: FontWeight.w500),
          ),
          const SizedBox(height: 4),
          Text(
            'Tambahkan alamat pengiriman terlebih dahulu',
            style: TextStyle(fontSize: 12, color: AppColors.neutralGray600),
          ),
          const SizedBox(height: 12),
          ElevatedButton.icon(
            onPressed: () => context.push(RoutePaths.addresses),
            icon: const Icon(Icons.add, size: 16),
            label: const Text('Tambah Alamat'),
            style: ElevatedButton.styleFrom(
              backgroundColor: AppColors.primaryRed,
              foregroundColor: Colors.white,
            ),
          ),
        ],
      ),
    );
  }
}

/// Single address card with selection indicator
class _AddressCard extends StatelessWidget {
  final AddressEntity address;
  final bool isSelected;
  final VoidCallback onTap;

  const _AddressCard({
    required this.address,
    required this.isSelected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        margin: const EdgeInsets.only(bottom: 8),
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(8),
          border: Border.all(
            color: isSelected ? AppColors.primaryRed : AppColors.neutralGray200,
            width: isSelected ? 2 : 1,
          ),
          color: isSelected
              ? AppColors.primaryRed.withValues(alpha: 0.05)
              : AppColors.neutralWhite,
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Icon(
              isSelected ? Icons.radio_button_checked : Icons.radio_button_off,
              color: isSelected
                  ? AppColors.primaryRed
                  : AppColors.neutralGray400,
              size: 20,
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Text(
                        address.recipientName,
                        style: const TextStyle(fontWeight: FontWeight.w600),
                      ),
                      if (address.isPrimary) ...[
                        const SizedBox(width: 8),
                        Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 6,
                            vertical: 2,
                          ),
                          decoration: BoxDecoration(
                            color: AppColors.primaryRed.withValues(alpha: 0.1),
                            borderRadius: BorderRadius.circular(4),
                          ),
                          child: Text(
                            'Utama',
                            style: TextStyle(
                              fontSize: 10,
                              color: AppColors.primaryRed,
                              fontWeight: FontWeight.w500,
                            ),
                          ),
                        ),
                      ],
                    ],
                  ),
                  const SizedBox(height: 2),
                  Text(
                    address.phone,
                    style: TextStyle(
                      fontSize: 12,
                      color: AppColors.neutralGray600,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    '${address.streetAddress}, ${address.village.name}, '
                    '${address.district.name}, ${address.city.name}, '
                    '${address.province.name} ${address.postalCode}',
                    style: TextStyle(
                      fontSize: 12,
                      color: AppColors.neutralGray700,
                    ),
                    maxLines: 3,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
