/// Canonical Address Picker Sheet
///
/// THE one address-SELECTION surface that returns a chosen [AddressEntity]
/// to its caller. Interaction mirrors the canonical payment-method picker:
///
///     AddressPickerSheet.show(context, selectedAddressId: ...)
///         ↓
///     render the account's saved addresses (ShippingAddressCard rows)
///         ↓
///     user chooses one
///         ↓
///     return AddressEntity?  (null on dismissal)
///
/// SPLIT OF RESPONSIBILITY:
/// - LAYOUT of each row lives in [ShippingAddressCard] (the canonical
///   shipping-address tile, including the primary "Utama" badge).
/// - DATA comes from the canonical address state (`addressProvider` /
///   `addressesListProvider`) — never a second repository load path.
/// - SELECTION stays caller-owned: this surface never mutates primary
///   (`setPrimaryAddress` is never called here), never creates orders, and
///   knows nothing about CheckoutRequest, delivery options or payment.
///
/// This is NOT a checkout-specific component: any flow that lets a viewer
/// choose where goods ship from (checkout today, the auction claim flow
/// tomorrow) reuses THIS surface. Address MANAGEMENT (add / edit / delete /
/// set primary) remains the separate responsibility of AddressListScreen.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/user/profile/domain/entities/address_entity.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/address_providers.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/notifiers/address_notifier.dart';
import 'package:hishumi/domains/user/profile/presentation/widgets/address_form_dialog.dart';
import 'package:hishumi/shared/shared.dart';

/// Opens the canonical address picker and returns the chosen address, or null
/// if the viewer dismissed it.
class AddressPickerSheet {
  AddressPickerSheet._();

  static Future<AddressEntity?> show(
    BuildContext context, {
    String? selectedAddressId,
  }) {
    return AppBottomSheetBase.show<AddressEntity>(
      context: context,
      title: 'Pilih Alamat Pengiriman',
      padding: const EdgeInsets.fromLTRB(
        AppMetrics.p16,
        AppMetrics.p8,
        AppMetrics.p16,
        AppMetrics.p16,
      ),
      content: _AddressPickerContent(selectedAddressId: selectedAddressId),
    );
  }
}

class _AddressPickerContent extends ConsumerStatefulWidget {
  const _AddressPickerContent({required this.selectedAddressId});

  /// The caller's current selection, marked on the matching row.
  final String? selectedAddressId;

  @override
  ConsumerState<_AddressPickerContent> createState() =>
      _AddressPickerContentState();
}

class _AddressPickerContentState extends ConsumerState<_AddressPickerContent> {
  /// Guards the single initial-load trigger for this surface.
  bool _loadScheduled = false;

  @override
  void initState() {
    super.initState();
    // The canonical authority does not fetch on its own; each consuming
    // surface schedules the one canonical load on first mount (same contract
    // as Address List and the checkout summary). This is a refresh of the
    // shared collection, never a second load path.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || _loadScheduled) return;
      _loadScheduled = true;
      final user = ref.read(authenticatedUserProvider);
      if (user != null) {
        ref.read(addressProvider.notifier).loadAddresses(user.id);
      }
    });
  }

  Future<void> _retry() async {
    final user = ref.read(authenticatedUserProvider);
    if (user == null) return;
    await ref.read(addressProvider.notifier).loadAddresses(user.id);
  }

  /// Empty-state CTA: open the one address form, then refresh the canonical
  /// collection so the new address (auto-primary per the domain invariant)
  /// appears in this picker.
  Future<void> _openAddressForm() async {
    final saved = await AppBottomSheetBase.show<bool>(
      context: context,
      content: const AddressFormDialog(),
    );
    if (saved != true || !mounted) return;
    await _retry();
  }

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    final addressesAsync = ref.watch(addressesListProvider);

    return addressesAsync.when(
      data: (addresses) {
        if (addresses.isEmpty) {
          return ShippingAddressEmptyState(onAdd: _openAddressForm);
        }
        return Column(
          children: [
            for (final address in addresses)
              ShippingAddressCard(
                address: address,
                isSelected: address.id == widget.selectedAddressId,
                // Selection only: pop the chosen address back to the caller.
                // No primary mutation, no order knowledge.
                onTap: () => Navigator.of(context).pop(address),
              ),
          ],
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
        child: Column(
          children: [
            Text(
              'Gagal memuat alamat: $e',
              style: TextStyle(color: colorScheme.error),
            ),
            TextButton(onPressed: _retry, child: const Text('Coba lagi')),
          ],
        ),
      ),
    );
  }
}
