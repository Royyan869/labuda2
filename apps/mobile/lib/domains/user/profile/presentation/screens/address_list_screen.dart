import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/presentation/providers/address_list_provider.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/address_form_dialog.dart';

/// Address List Screen — the account's ONE address book.
///
/// CANONICAL DESIGN (single page, no tabs):
/// - One list of every saved address; each card carries its role TAGS
///   (Shipping / Sender). One address may carry both.
/// - Exactly one address per account is Primary. There is no per-tag primary.
/// - Add / edit goes through AddressFormDialog — the only address form.
/// - Min 1 address, max 10 addresses per account.
class AddressListScreen extends ConsumerStatefulWidget {
  const AddressListScreen({super.key});

  @override
  ConsumerState<AddressListScreen> createState() => _AddressListScreenState();
}

class _AddressListScreenState extends ConsumerState<AddressListScreen> {
  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    // Use centralized providers (TANGGUNG_JAWAB_MODUL compliance)
    final authState = ref.watch(authControllerProvider);
    final currentUser = ref.watch(authenticatedUserProvider);
    if (currentUser == null) {
      if (_isUnresolvedAuthState(authState)) {
        return _buildUnknownSellerState(context);
      }

      return PopScope(
        canPop: true,
        child: Scaffold(
          appBar: AppBar(
            title: const Text('Addresses'),
            leading: IconButton(
              icon: const Icon(Icons.arrow_back),
              onPressed: () => Navigator.of(context).pop(),
            ),
          ),
          body: const Center(child: Text('Please login to continue')),
        ),
      );
    }

    final userId = currentUser.id;

    final addressesAsync = ref.watch(addressesStreamProvider(userId));

    return PopScope(
      canPop: true,
      child: Scaffold(
        // ONE rule for this screen: the theme owns every chrome colour
        // (scaffold, app bar, title ink, icons). The ink role
        // `onSurfaceVariant` used to be pasted in here — a grey slab with
        // back button and title in the exact same grey as their background.
        appBar: AppBar(
          title: const Text('Addresses'),
          leading: IconButton(
            icon: const Icon(Icons.arrow_back),
            onPressed: () => Navigator.of(context).pop(),
          ),
          actions: [
            IconButton(
              icon: Icon(
                Icons.info_outline,
                color: scheme.onSurfaceVariant,
              ),
              tooltip: 'Info',
              onPressed: () => _showAddressInfoDialog(context, scheme),
            ),
          ],
        ),
        body: addressesAsync.when(
          data: (result) {
            if (result.isError) {
              return Center(
                child: Text(
                  result.error ?? 'Failed to load addresses',
                  style: TextStyle(
                    color: scheme.onSurfaceVariant,
                  ),
                ),
              );
            }

            final addresses = result.data ?? [];

            // ONE list for the whole account. Role tags live on the card,
            // never in a tab that splits the book in two.
            return SafeArea(
              child: Column(
                children: [
                  Expanded(
                    child: _buildAddressList(
                      context,
                      addresses,
                      userId,
                      scheme,
                    ),
                  ),
                  _buildStickyAddButton(context, addresses),
                ],
              ),
            );
          },
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (error, stack) => Center(
            child: Text(
              'Error: $error',
              style: TextStyle(
                color: scheme.onSurfaceVariant,
              ),
            ),
          ),
        ),
      ),
    );
  }

  bool _isUnresolvedAuthState(AuthState authState) {
    return authState is AuthStateInitial ||
        authState is AuthStateLoading ||
        authState is AuthStateFirebaseAuthenticated ||
        authState is AuthStateSyncingWithBackend;
  }

  Widget _buildUnknownSellerState(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Addresses')),
      body: const Center(child: CircularProgressIndicator()),
    );
  }

  Widget _buildAddressList(
    BuildContext context,
    List<AddressEntity> addresses,
    String userId,
    ColorScheme scheme,
  ) {
    // One account, one book: the account-level minimum is 1 address.
    final canDelete = addresses.length > 1;

    if (addresses.isEmpty) {
      // Canonical empty state (shared) — the screen no longer owns its own
      // copy; the dead `AddressEmptyState`/`AddressEmptyStateWidget`
      // duplicates were the competing one.
      return EmptyState(
        icon: Icons.location_off_outlined,
        title: 'No Address Yet',
        subtitle: 'Add an address to shop and to ship from',
      );
    }

    return ListView(
      padding: const EdgeInsets.only(left: AppMetrics.p16, right: AppMetrics.p16, top: AppMetrics.p16, bottom: AppMetrics.p16),
      children: [
        ...addresses.map((address) {
          return Padding(
            padding: const EdgeInsets.only(bottom: AppMetrics.p12),
            child: _buildAddressCard(
              context,
              address,
              canDelete,
              userId,
              addresses.length,
              scheme,
            ),
          );
        }),
      ],
    );
  }

  Widget _buildStickyAddButton(
    BuildContext context,
    List<AddressEntity> addresses,
  ) {
    // Max 10 addresses per account - hide button if limit reached
    if (addresses.length >= 10) {
      return const SizedBox.shrink();
    }

    return Container(
      padding: const EdgeInsets.only(left: AppMetrics.p16, right: AppMetrics.p16, top: AppMetrics.p12, bottom: AppMetrics.p12),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        border: Border(
          top: BorderSide(
            color: Theme.of(context).colorScheme.outlineVariant,
          ),
        ),
      ),
      child: SizedBox(
        width: double.infinity,
        child: ElevatedButton.icon(
          onPressed: () => _showAddressDialog(context),
          icon: const Icon(Icons.add_location_alt),
          label: const Text('Add Address'),
          style: ElevatedButton.styleFrom(
            padding: const EdgeInsets.symmetric(vertical: AppMetrics.p16),
          ),
        ),
      ),
    );
  }

  void _showAddressInfoDialog(BuildContext context, ColorScheme scheme) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        // Dialog surface + title ink come from the theme (dialogTheme); the
        // old copy pasted `onSurfaceVariant` into both, so the dialog was a
        // grey card with grey-on-grey text.
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppShape.r16),
        ),
        title: Row(
          children: [
            Icon(Icons.info_outline, color: scheme.primary, size: AppIconSize.header),
            const SizedBox(width: 12),
            const Text('Address Information'),
          ],
        ),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _buildInfoRow(
              Icons.home,
              'Shipping tag',
              'Use this address as a delivery destination at checkout',
              scheme,
            ),
            const SizedBox(height: 12),
            _buildInfoRow(
              Icons.agriculture,
              'Sender tag',
              'Use this address as the origin of goods you ship from',
              scheme,
            ),
            const SizedBox(height: 12),
            _buildInfoRow(
              Icons.star_outline,
              'Primary',
              'One address per account is the default everywhere',
              scheme,
            ),
            const SizedBox(height: 16),
            Container(
              padding: const EdgeInsets.all(AppMetrics.p12),
              decoration: BoxDecoration(
                color: scheme.primary.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(AppShape.r8),
              ),
              child: Row(
                children: [
                  Icon(Icons.rule, color: scheme.primary, size: AppIconSize.action),
                  const SizedBox(width: 10),
                  Expanded(
                    child: Text(
                      'One address can carry both tags.\nMin. 1 address, max. 10 per account.',
                      style: TextStyle(
                        fontSize: AppType.s14,
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: Text(
              'Got it',
              style: TextStyle(
                color: scheme.primary,
                fontWeight: FontWeight.w600,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildInfoRow(IconData icon, String title, String desc, ColorScheme scheme) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(
          icon,
          size: AppIconSize.action,
          color: scheme.onSurfaceVariant,
        ),
        const SizedBox(width: 10),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                title,
                // Primary line reads the primary ink, description the
                // secondary one — both used to be secondary.
                style: TextStyle(
                  fontSize: AppType.s14,
                  fontWeight: FontWeight.w600,
                  color: scheme.onSurface,
                ),
              ),
              Text(
                desc,
                style: TextStyle(
                  fontSize: AppType.s12,
                  color: scheme.onSurfaceVariant,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildAddressCard(
    BuildContext context,
    AddressEntity address,
    bool canDelete,
    String userId,
    int totalAddresses,
    ColorScheme scheme,
  ) {
    return Container(
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(
          color: address.isPrimary ? scheme.primary : scheme.outlineVariant,
          width: address.isPrimary ? 2 : 1,
        ),
      ),
      child: Padding(
        padding: const EdgeInsets.all(AppMetrics.p16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Header: Label, Primary badge, Actions
            Row(
              children: [
                Icon(
                  _getTagIcon(address.tags.isNotEmpty
                      ? address.tags.first
                      : AddressTag.shipping),
                  size: 20,
                  color: scheme.primary,
                ),
                const SizedBox(width: 8),
                Text(
                  address.displayLabel,
                  style: TextStyle(
                    fontSize: AppType.s16,
                    fontWeight: FontWeight.bold,
                    color: scheme.onSurface,
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
                      color: scheme.primary,
                      borderRadius: BorderRadius.circular(AppShape.r4),
                    ),
child: Text(
                       'Primary',
                      style: TextStyle(
                        fontSize: AppType.s12,
                        fontWeight: FontWeight.w600,
                        color: scheme.onPrimary,
                      ),
                    ),
                  ),
                ],
                const Spacer(),
                PopupMenuButton<String>(
                  onSelected: (value) {
                    switch (value) {
                      case 'edit':
                        _showAddressDialog(context, address);
                        break;
                      case 'setPrimary':
                        _setPrimaryAddress(context, address, userId);
                        break;
                      case 'delete':
                        _deleteAddress(
                          context,
                          address,
                          totalAddresses,
                        );
                        break;
                    }
                  },
                  itemBuilder: (context) => [
                    const PopupMenuItem(
                      value: 'edit',
                      child: Row(
                        children: [
                          Icon(Icons.edit, size: AppIconSize.action),
                          SizedBox(width: 8),
                          Text('Edit'),
                        ],
                      ),
                    ),
                    // "Set as Primary" only makes sense as a CHOICE —
                    // at1 address the reconciler already owns the flag.
                    if (totalAddresses >= 2 && !address.isPrimary)
                      const PopupMenuItem(
                        value: 'setPrimary',
                        child: Row(
                          children: [
                            Icon(Icons.star, size: AppIconSize.action),
                            SizedBox(width: 8),
                            Text('Set as Primary'),
                          ],
                        ),
                      ),
                    if (canDelete && !address.isPrimary)
                      PopupMenuItem(
                        value: 'delete',
                        child: Row(
                          children: [
                            Icon(
                              Icons.delete,
                              size: AppIconSize.action,
                              color: context.statusColors.error,
                            ),
                            const SizedBox(width: 8),
                            Text(
                              'Delete',
                              style: TextStyle(color: context.statusColors.error),
                            ),
                          ],
                        ),
                      ),
                  ],
                ),
              ],
            ),
            const SizedBox(height: 12),

            // Role tags: what this address is FOR. One address may carry both.
            Wrap(
              spacing: AppMetrics.p8,
              runSpacing: AppMetrics.p8,
              children: [
                for (final tag in address.tags)
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: AppMetrics.p8,
                      vertical: AppMetrics.p4,
                    ),
                    decoration: BoxDecoration(
                      color: scheme.primary.withValues(alpha: 0.10),
                      borderRadius: BorderRadius.circular(AppShape.r6),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          _getTagIcon(tag),
                          size: 12,
                          color: scheme.primary,
                        ),
                        const SizedBox(width: 4),
                        Text(
                          tag.shortLabel,
                          style: TextStyle(
                            fontSize: AppType.s12,
                            fontWeight: FontWeight.w600,
                            color: scheme.primary,
                          ),
                        ),
                      ],
                    ),
                  ),
              ],
            ),
            const SizedBox(height: 10),

            // Recipient/Sender Name & Phone
            Row(
              children: [
                Icon(
                  Icons.person_outline,
                  size: AppIconSize.inlineGlyph,
                  color: scheme.onSurfaceVariant,
                ),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    '${address.recipientName} • ${address.phone}',
                    style: TextStyle(
                      fontSize: AppType.s14,
                      fontWeight: FontWeight.w500,
                      color: scheme.onSurfaceVariant,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),

            // Full Address
            Text(
              address.fullAddress,
              style: TextStyle(
                fontSize: AppType.s14,
                color: scheme.onSurfaceVariant,
              ),
            ),

            // Notes (if available)
            if (address.notes != null && address.notes!.isNotEmpty) ...[
              const SizedBox(height: 8),
              Container(
                padding: const EdgeInsets.all(AppMetrics.p8),
                // Subtle fill role, not an ink role: a solid `onSurfaceVariant`
                // box with `onSurfaceVariant` text inside it was invisible.
                decoration: BoxDecoration(
                  color: scheme.surfaceContainerHighest,
                  borderRadius: BorderRadius.circular(AppShape.r6),
                ),
                child: Row(
                  children: [
                    Icon(Icons.note, size: AppIconSize.inlineGlyph, color: scheme.onSurfaceVariant),
                    const SizedBox(width: 6),
                    Expanded(
                      child: Text(
                        address.notes!,
                        style: TextStyle(
                          fontSize: AppType.s12,
                          fontStyle: FontStyle.italic,
                          color: scheme.onSurfaceVariant,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ],

            // Coordinate indicator (clickable)
            if (address.hasCoordinates) ...[
              const SizedBox(height: 8),
              InkWell(
                onTap: () => _showCoordinatePreview(context, address, scheme),
                borderRadius: BorderRadius.circular(AppShape.r6),
                child: Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppMetrics.p12,
                    vertical: AppMetrics.p8,
                  ),
                  decoration: BoxDecoration(
                    color: context.statusColors.success.withValues(alpha: 0.1),
                    borderRadius: BorderRadius.circular(AppShape.r6),
                    border: Border.all(
                      color: context.statusColors.success.withValues(alpha: 0.3),
                    ),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(
                        Icons.location_on,
                        size: AppIconSize.inlineGlyph,
                        color: context.statusColors.success,
                      ),
                      const SizedBox(width: 6),
                      Text(
                        'Pinpoint Location Saved',
                        style: TextStyle(
                          fontSize: AppType.s12,
                          fontWeight: FontWeight.w500,
                          color: context.statusColors.success,
                        ),
                      ),
                      const SizedBox(width: 4),
                      Icon(
                        Icons.chevron_right,
                        size: AppIconSize.inlineGlyph,
                        color: context.statusColors.success,
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }

  IconData _getTagIcon(AddressTag tag) {
    switch (tag) {
      case AddressTag.shipping:
        return Icons.home; // Shipping destination
      case AddressTag.sender:
        return Icons.agriculture; // Sender origin (farm/warehouse)
    }
  }

  Future<void> _showAddressDialog(
    BuildContext context, [
    AddressEntity? address,
  ]) async {
    await showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (context) => AddressFormDialog(addressToEdit: address),
    );
  }

  void _deleteAddress(
    BuildContext context,
    AddressEntity address,
    int totalAddresses,
  ) async {
    // Min 1 address for the account
    if (totalAddresses <= 1) {
      AppSnackBar.showError(
        context,
        'Cannot delete. You need at least 1 address.',
      );
      return;
    }

    if (address.isPrimary) {
      AppSnackBar.showWarning(
        context,
        'Cannot delete primary address. Set another address as primary first.',
      );
      return;
    }

    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Delete Address'),
        content: Text(
          'Are you sure you want to delete this address?\n\n${address.displayLabel}\n${address.fullAddress}',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(context, true),
            style: TextButton.styleFrom(foregroundColor: context.statusColors.error),
            child: const Text('Delete'),
          ),
        ],
      ),
    );

    if (confirmed != true) return;

    final repository = ref.read(addressRepositoryProvider);
    final result = await repository.deleteAddress(address.id);

    if (!context.mounted) return;

    if (result.isSuccess) {
      AppSnackBar.showSuccess(context, 'Address deleted successfully');
    } else {
      AppSnackBar.showError(
        context,
        result.error ?? 'Failed to delete address',
      );
    }
  }

  void _setPrimaryAddress(
    BuildContext context,
    AddressEntity address,
    String userId,
  ) async {
    if (address.isPrimary) {
      AppSnackBar.showInfo(context, 'This address is already the primary');
      return;
    }

    final repository = ref.read(addressRepositoryProvider);
    final result = await repository.setPrimaryAddress(address.id, userId);

    if (!context.mounted) return;

    if (result.isSuccess) {
      AppSnackBar.showSuccess(context, 'Primary address updated');
    } else {
      AppSnackBar.showError(
        context,
        result.error ?? 'Failed to set primary address',
      );
    }
  }

  void _showCoordinatePreview(
    BuildContext context,
    AddressEntity address,
    ColorScheme scheme,
  ) {
    if (!address.hasCoordinates) return;

    CoordinatePreviewModal.show(
      context,
      latitude: address.latitude!,
      longitude: address.longitude!,
      address: address.streetAddress,
      onCoordinatesChanged: (lat, lng) {
        // Update alamat dengan koordinat baru
        _updateAddressCoordinates(context, address, lat, lng);
      },
    );
  }

  void _updateAddressCoordinates(
    BuildContext context,
    AddressEntity address,
    double latitude,
    double longitude,
  ) async {
    final repository = ref.read(addressRepositoryProvider);
    final updatedAddress = address.copyWith(
      latitude: latitude,
      longitude: longitude,
      updatedAt: DateTime.now(),
    );

    final result = await repository.updateAddress(updatedAddress);

    if (!context.mounted) return;

    if (result.isSuccess) {
      AppSnackBar.showSuccess(
        context,
        'Pinpoint location updated successfully',
      );
    } else {
      AppSnackBar.showError(
        context,
        result.error ?? 'Failed to update pinpoint location',
      );
    }
  }
}
