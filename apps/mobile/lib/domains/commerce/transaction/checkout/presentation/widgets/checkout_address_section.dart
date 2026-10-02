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
            .loadAddressesByTag(authState.user.id, AddressTag.shipping);
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    final addressesAsync = ref.watch(addressesListProvider);

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
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                'Alamat Pengiriman',
                // Section-header role (canonical map: section → titleMedium).
                // This was s18 here while the auction claim sheet said the
                // same line at s16.
                style: Theme.of(context).textTheme.titleMedium,
              ),
              TextButton.icon(
                onPressed: () => context.push(RoutePaths.addresses),
                icon: const Icon(Icons.add, size: AppIconSize.inlineGlyph),
                label: const Text('Kelola'),
                style: TextButton.styleFrom(
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppMetrics.p8,
                    vertical: AppMetrics.p4,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          addressesAsync.when(
            data: (addresses) {
              // Server already narrows by tag (tag miss → falls back to every
              // active address). Never re-filter here — it would undo the
              // fallback and empty the checkout for no reason.
              final shippingAddresses = addresses;
              if (shippingAddresses.isEmpty) {
                return ShippingAddressEmptyState(
                  onAdd: _openAddressForm,
                );
              }
              // Auto-select primary if nothing selected yet
              if (widget.selectedAddressId == null) {
                final primary = shippingAddresses
                        .where((a) => a.isPrimary)
                        .firstOrNull ??
                    shippingAddresses.firstOrNull;
                if (primary != null) {
                  WidgetsBinding.instance.addPostFrameCallback((_) {
                    widget.onAddressSelected(primary);
                  });
                }
              }
              return Column(
                children: shippingAddresses.map((addr) {
                  final isSelected = addr.id == widget.selectedAddressId;
                  return ShippingAddressCard(
                    address: addr,
                    isSelected: isSelected,
                    onTap: () => widget.onAddressSelected(addr),
                  );
                }).toList(),
              );
            },
            loading: () => const Center(
              child: Padding(
                padding: EdgeInsets.all(AppMetrics.p16),
                child: CircularProgressIndicator(),
              ),
            ),
            error: (e, _) => Padding(
              padding: const EdgeInsets.all(AppMetrics.p8),
              child: Text(
                'Gagal memuat alamat: $e',
                style: TextStyle(color: colorScheme.error),
              ),
            ),
          ),
        ],
      ),
    );
  }

  /// Empty-state CTA: open the one address form directly, pre-locked to the
  /// shipping role this flow needs. Jumping to the address list instead made
  /// the buyer do the routing the app should have done.
  Future<void> _openAddressForm() async {
    final saved = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (context) => const AddressFormDialog(
        presetTags: [AddressTag.shipping],
      ),
    );

    if (saved != true || !mounted) return;

    final authState = ref.read(authControllerProvider);
    if (authState is AuthStateAuthenticated) {
      await ref
          .read(addressProvider.notifier)
          .loadAddressesByTag(authState.user.id, AddressTag.shipping);
    }
  }
}
