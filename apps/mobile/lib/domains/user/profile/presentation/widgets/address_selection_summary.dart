/// Canonical shipping-address SUMMARY + change-address entry.
///
/// THE transaction-surface address presentation, shared by every caller that
/// needs the viewer to choose where goods ship from (Checkout, Auction Claim,
/// and anything after them):
///
/// - renders EXACTLY ONE address — the caller-selected one while it is still
///   in the account book, else the CANONICAL primary
///   (`primaryAddressProvider`, never a caller-local resolver);
/// - "Ubah alamat" (and tapping the tile) opens the canonical
///   [AddressPickerSheet] — the ONLY surface that may list the inventory;
/// - the canonical [ShippingAddressEmptyState] whose CTA opens the one
///   [AddressFormDialog].
///
/// Selection stays caller-owned: the resolved address travels through
/// [onAddressSelected], the caller's single selection entry point (which owns
/// delivery/preview/claim side effects). This widget never mutates primary,
/// never creates orders, and knows nothing about the caller's pipeline.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/presentation/providers/address_providers.dart';
import 'package:labuda/domains/user/profile/presentation/providers/notifiers/address_notifier.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/address_form_dialog.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/address_picker_sheet.dart';
import 'package:labuda/shared/shared.dart';

class AddressSelectionSummary extends ConsumerStatefulWidget {
  const AddressSelectionSummary({
    super.key,
    required this.selectedAddressId,
    required this.onAddressSelected,
  });

  /// The caller's current selection (null → the canonical primary defaults).
  final String? selectedAddressId;

  /// The caller's ONE selection entry point: auto-default, re-default and
  /// picker results all travel through this callback.
  final ValueChanged<AddressEntity> onAddressSelected;

  @override
  ConsumerState<AddressSelectionSummary> createState() =>
      _AddressSelectionSummaryState();
}

class _AddressSelectionSummaryState
    extends ConsumerState<AddressSelectionSummary> {
  @override
  void initState() {
    super.initState();
    // The canonical authority does not fetch on its own; each consuming
    // surface schedules the ONE canonical load on first mount (same contract
    // as Address List and the picker).
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final user = ref.read(authenticatedUserProvider);
      if (user != null) {
        ref.read(addressProvider.notifier).loadAddresses(user.id);
      }
    });
  }

  /// "Ubah alamat" → the canonical address picker. The chosen address returns
  /// here and travels through the caller's existing selection pipeline.
  Future<void> _openAddressPicker() async {
    final selected = await AddressPickerSheet.show(
      context,
      selectedAddressId: widget.selectedAddressId,
    );
    if (selected == null || !mounted) return;
    widget.onAddressSelected(selected);
  }

  /// Empty-state CTA: open the one address form directly. On save the
  /// canonical authority reloads the shared collection; the domain invariant
  /// (first address auto-primary) then feeds the summary via
  /// `primaryAddressProvider`.
  Future<void> _openAddressForm() async {
    final saved = await AppBottomSheetBase.show<bool>(
      context: context,
      content: const AddressFormDialog(),
    );
    if (saved != true || !mounted) return;
    final user = ref.read(authenticatedUserProvider);
    if (user != null) {
      await ref.read(addressProvider.notifier).loadAddresses(user.id);
    }
  }

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    final addressesAsync = ref.watch(addressesListProvider);
    final primaryAsync = ref.watch(primaryAddressProvider);
    final knownAddresses = addressesAsync.value ?? const <AddressEntity>[];

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
            children: [
              // Flexible title: the header must never overflow narrow
              // viewports when the change-address action sits beside it.
              Flexible(
                child: Text(
                  'Alamat Pengiriman',
                  // Section-header role (canonical map: section → titleMedium).
                  style: Theme.of(context).textTheme.titleMedium,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              // "Ubah alamat" — available whenever the account owns at least
              // one address, even when there is only one (selection surface,
              // never management).
              if (knownAddresses.isNotEmpty)
                TextButton.icon(
                  onPressed: _openAddressPicker,
                  icon: const Icon(
                    Icons.edit_location_alt_outlined,
                    size: AppIconSize.inlineGlyph,
                  ),
                  label: const Text('Ubah alamat'),
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
              if (addresses.isEmpty) {
                return ShippingAddressEmptyState(onAdd: _openAddressForm);
              }
              // Selection priority: caller-selected (while still valid in the
              // book) → canonical primary → first row (invariant-failure
              // defensive fallback only; the backend reconciler guarantees
              // exactly one primary when the book is non-empty).
              AddressEntity? callerSelected;
              final selectedId = widget.selectedAddressId;
              if (selectedId != null) {
                callerSelected = addresses
                    .where((a) => a.id == selectedId)
                    .firstOrNull;
              }
              final summary =
                  callerSelected ?? primaryAsync.value ?? addresses.firstOrNull;
              if (summary == null) {
                return ShippingAddressEmptyState(onAdd: _openAddressForm);
              }
              // Default (or re-default): nothing selected yet, or the caller's
              // selection left the address book → (re)select the summary
              // through the caller's ONE selection entry point.
              if (widget.selectedAddressId != summary.id) {
                WidgetsBinding.instance.addPostFrameCallback((_) {
                  if (mounted) widget.onAddressSelected(summary);
                });
              }
              return ShippingAddressCard(
                address: summary,
                isSelected: widget.selectedAddressId == summary.id,
                // Tapping the summary is a change-address affordance: it
                // opens the same canonical picker as the header action.
                onTap: _openAddressPicker,
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
}
