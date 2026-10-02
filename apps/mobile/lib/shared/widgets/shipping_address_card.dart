import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';

/// Canonical SHIPPING-ADDRESS selection surfaces.
///
/// ONE AUTHORITY for every flow that lets a viewer choose where goods ship
/// from — checkout and the auction claim sheet, plus anything after them.
/// Both screens used to keep their own copy, and the copies had drifted:
/// selected fill `alpha 0.05` vs `0.08`, a Material radio icon in one and a
/// hand-drawn circle in the other, badge weight `w500` vs `w600` at two
/// different alphas, a nickname line in one but not the other, an address
/// line assembled by hand in one and read from `AddressEntity.fullAddress`
/// in the other, and `maxLines 3` vs `2`.
///
/// The split of responsibility: LAYOUT lives here, the ACTION belongs to the
/// caller. The empty state renders one button, each flow decides what it
/// does (checkout pushes the address book; the claim sheet closes itself),
/// because that difference is about navigation, not about how the card looks.
class ShippingAddressCard extends StatelessWidget {
  const ShippingAddressCard({
    super.key,
    required this.address,
    required this.isSelected,
    required this.onTap,
  });

  /// The address being offered for selection.
  final AddressEntity address;

  /// Whether this card is the current pick.
  final bool isSelected;

  /// Selection handler — the card itself never mutates provider state.
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return GestureDetector(
      onTap: onTap,
      child: Container(
        margin: const EdgeInsets.only(bottom: AppMetrics.p8),
        padding: const EdgeInsets.all(AppMetrics.p12),
        decoration: BoxDecoration(
          color: isSelected
              ? scheme.primary.withValues(alpha: 0.05)
              : scheme.surface,
          borderRadius: BorderRadius.circular(AppShape.r8),
          border: Border.all(
            color: isSelected ? scheme.primary : scheme.outlineVariant,
            width: isSelected ? 2 : 1,
          ),
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Icon(
              isSelected ? Icons.radio_button_checked : Icons.radio_button_off,
              color: isSelected ? scheme.primary : scheme.onSurfaceVariant,
              size: AppIconSize.action,
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Labelled addresses (Home, Gudang, …) lead with the label.
                  if (address.nickname != null) ...[
                    Text(
                      address.nickname!,
                      style: TextStyle(
                        fontSize: AppType.s12,
                        fontWeight: FontWeight.w600,
                        color: scheme.primary,
                      ),
                    ),
                  ],
                  Row(
                    children: [
                      Flexible(
                        child: Text(
                          address.recipientName,
                          style: const TextStyle(
                            fontSize: AppType.s14,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                      ),
                      if (address.isPrimary) ...[
                        const SizedBox(width: 8),
                        Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: AppMetrics.p8,
                            vertical: AppMetrics.p4,
                          ),
                          decoration: BoxDecoration(
                            color: scheme.primary.withValues(alpha: 0.1),
                            borderRadius: BorderRadius.circular(AppShape.r4),
                          ),
                          child: Text(
                            'Utama',
                            style: TextStyle(
                              fontSize: AppType.s12,
                              color: scheme.primary,
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
                      fontSize: AppType.s12,
                      color: scheme.onSurfaceVariant,
                    ),
                  ),
                  const SizedBox(height: 4),
                  // The entity owns how an address string is composed.
                  Text(
                    address.fullAddress,
                    style: TextStyle(
                      fontSize: AppType.s12,
                      color: scheme.onSurfaceVariant,
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

/// Canonical empty prompt shown when the viewer has no shipping address yet.
///
/// The button LOOK is owned here; [onAdd] is owned by the flow, so a caller
/// can close a sheet, push the address book, or both — without forking the
/// layout to do it.
class ShippingAddressEmptyState extends StatelessWidget {
  const ShippingAddressEmptyState({super.key, required this.onAdd});

  /// What "Tambah Alamat" does in the calling flow.
  final VoidCallback onAdd;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(AppShape.r8),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Column(
        children: [
          Icon(
            Icons.location_off,
            size: AppIconSize.emphasis,
            color: scheme.onSurfaceVariant,
          ),
          const SizedBox(height: 8),
          const Text(
            'Belum ada alamat pengiriman',
            style: TextStyle(fontWeight: FontWeight.w500),
          ),
          const SizedBox(height: 4),
          Text(
            'Tambahkan alamat pengiriman terlebih dahulu',
            style: TextStyle(
              fontSize: AppType.s12,
              color: scheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 12),
          ElevatedButton.icon(
            onPressed: onAdd,
            icon: const Icon(Icons.add, size: AppIconSize.inlineGlyph),
            label: const Text('Tambah Alamat'),
          ),
        ],
      ),
    );
  }
}
