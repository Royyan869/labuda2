import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/user/preference/seller/presentation/providers/current_seller_provider.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/presentation/providers/address_list_provider.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/address_form_dialog.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_state.dart';

/// Address List Screen - Tab-based Multiple Addresses Support
///
/// Features:
/// - Tab-based separation: Shipping vs Sender addresses
/// - Tab visibility: Buyer (shipping only) vs Seller (both tabs)
/// - Add new address via modal dialog (purpose based on active tab)
/// - Edit existing address via modal dialog
/// - Delete address (with validation per purpose)
/// - Set primary address (per purpose)
/// - Max 10 addresses per purpose, Min 1 per purpose
class AddressListScreen extends ConsumerStatefulWidget {
  const AddressListScreen({super.key});

  @override
  ConsumerState<AddressListScreen> createState() => _AddressListScreenState();
}

class _AddressListScreenState extends ConsumerState<AddressListScreen>
    with SingleTickerProviderStateMixin {
  TabController? _tabController;

  @override
  void initState() {
    super.initState();
    // TabController will be initialized after we know if user is seller
  }

  @override
  void dispose() {
    _tabController?.dispose();
    super.dispose();
  }

  void _initializeTabController(bool isSeller) {
    if (_tabController == null && isSeller) {
      _tabController = TabController(length: 2, vsync: this);
      _tabController!.addListener(() {
        // Rebuild to update sticky button text when tab changes
        if (mounted) setState(() {});
      });
    }
  }

  AddressPurpose get _currentPurpose {
    if (_tabController == null) {
      return AddressPurpose.shipping; // Buyer only has shipping
    }
    return _tabController!.index == 0
        ? AddressPurpose.shipping
        : AddressPurpose.sender;
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    // Use centralized providers (TANGGUNG_JAWAB_MODUL compliance)
    final authState = ref.watch(authControllerProvider);
    final currentUser = ref.watch(authenticatedUserProvider);
    final sellerIdentityStatus = ref.watch(sellerIdentityStatusProvider);

    if (currentUser == null) {
      if (_isUnresolvedAuthState(authState) ||
          sellerIdentityStatus == SellerIdentityStatus.unknown) {
        return _buildUnknownSellerState(context, scheme);
      }

      return PopScope(
        canPop: true,
        child: Scaffold(
          appBar: AppBarCustom(
            title: 'Addresses',
            leading: IconButton(
              icon: const Icon(Icons.arrow_back),
              onPressed: () => Navigator.of(context).pop(),
            ),
          ),
          body: const Center(child: Text('Please login to continue')),
        ),
      );
    }

    if (sellerIdentityStatus == SellerIdentityStatus.unknown) {
      return _buildUnknownSellerState(context, scheme);
    }

    final isSeller = sellerIdentityStatus == SellerIdentityStatus.seller;
    final userId = currentUser.id;

    // Initialize TabController for sellers
    _initializeTabController(isSeller);

    final addressesAsync = ref.watch(addressesStreamProvider(userId));

    return PopScope(
      canPop: true,
      child: Scaffold(
        backgroundColor: scheme.onSurfaceVariant,
        appBar: AppBar(
          title: Text(
            'Addresses',
            style: TextStyle(
              fontSize: 18,
              fontWeight: FontWeight.w600,
              color: scheme.onSurfaceVariant,
            ),
          ),
          backgroundColor: scheme.onSurfaceVariant,
          foregroundColor: scheme.onSurfaceVariant,
          elevation: 0,
          surfaceTintColor: Colors.transparent,
          scrolledUnderElevation: 0,
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
          bottom: isSeller && _tabController != null
              ? TabBar(
                  controller: _tabController,
                  labelColor: scheme.primary,
                  unselectedLabelColor: scheme.onSurfaceVariant,
                  indicatorColor: scheme.primary,
                  indicatorWeight: 3,
                  tabs: const [
                    Tab(icon: Icon(Icons.home), text: 'Shipping Address'),
                    Tab(icon: Icon(Icons.agriculture), text: 'Sender Address'),
                  ],
                )
              : null,
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

            if (isSeller && _tabController != null) {
              // Seller: Show TabBarView with sticky button
              return SafeArea(
                child: Column(
                  children: [
                    Expanded(
                      child: TabBarView(
                        controller: _tabController,
                        children: [
                          _buildTabContent(
                            context,
                            addresses,
                            userId,
                            AddressPurpose.shipping,
                            scheme,
                          ),
                          _buildTabContent(
                            context,
                            addresses,
                            userId,
                            AddressPurpose.sender,
                            scheme,
                          ),
                        ],
                      ),
                    ),
                    _buildStickyAddButton(context, addresses, scheme),
                  ],
                ),
              );
            } else {
              // Buyer: Show only shipping addresses with sticky button
              return SafeArea(
                child: Column(
                  children: [
                    Expanded(
                      child: _buildTabContent(
                        context,
                        addresses,
                        userId,
                        AddressPurpose.shipping,
                        scheme,
                      ),
                    ),
                    _buildStickyAddButton(context, addresses, scheme),
                  ],
                ),
              );
            }
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

  Widget _buildUnknownSellerState(BuildContext context, ColorScheme scheme) {
    return Scaffold(
      backgroundColor: scheme.surfaceContainerLowest,
      appBar: AppBar(
        title: const Text('Addresses'),
        backgroundColor: scheme.onSurfaceVariant,
        foregroundColor: scheme.onSurfaceVariant,
        elevation: 0,
        surfaceTintColor: Colors.transparent,
        scrolledUnderElevation: 0,
      ),
      body: const Center(child: CircularProgressIndicator()),
    );
  }

  Widget _buildTabContent(
    BuildContext context,
    List<AddressEntity> allAddresses,
    String userId,
    AddressPurpose purpose,
    ColorScheme scheme,
  ) {
    // Filter addresses by purpose
    final filteredAddresses = allAddresses
        .where((addr) => addr.purpose == purpose)
        .toList();

    final canDelete = filteredAddresses.length > 1; // Min 1 per purpose

    if (filteredAddresses.isEmpty) {
      return _buildEmptyState(context, purpose, scheme);
    }

    return ListView(
      padding: const EdgeInsets.only(left: 16, right: 16, top: 16, bottom: 16),
      children: [
        // Address Cards
        ...filteredAddresses.map((address) {
          return Padding(
            padding: const EdgeInsets.only(bottom: 12),
            child: _buildAddressCard(
              context,
              address,
              canDelete,
              userId,
              filteredAddresses.length,
              scheme,
              purpose,
            ),
          );
        }),
      ],
    );
  }

  Widget _buildEmptyState(
    BuildContext context,
    AddressPurpose purpose,
    ColorScheme scheme,
  ) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              purpose == AddressPurpose.shipping
                  ? Icons.location_off_outlined
                  : Icons.agriculture_outlined,
              size: 80,
              color: scheme.onSurfaceVariant,
            ),
            const SizedBox(height: 24),
            Text(
              'No ${purpose.label} Yet',
              style: TextStyle(
                fontSize: 20,
                fontWeight: FontWeight.bold,
                color: scheme.onSurfaceVariant,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              purpose == AddressPurpose.shipping
                  ? 'Add a shipping address to start shopping'
                  : 'Add a sender address (farm/warehouse location)',
              textAlign: TextAlign.center,
              style: TextStyle(
                fontSize: 14,
                color: scheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildStickyAddButton(
    BuildContext context,
    List<AddressEntity> addresses,
    ColorScheme scheme,
  ) {
    final purpose = _currentPurpose;
    final filteredAddresses = addresses
        .where((addr) => addr.purpose == purpose)
        .toList();

    // Max 10 addresses per purpose - hide button if limit reached
    if (filteredAddresses.length >= 10) {
      return const SizedBox.shrink();
    }

    return Container(
      padding: const EdgeInsets.only(left: 16, right: 16, top: 12, bottom: 12),
      decoration: BoxDecoration(
        color: scheme.surface,
        border: Border(
          top: BorderSide(
            color: scheme.outlineVariant,
          ),
        ),
      ),
      child: SizedBox(
        width: double.infinity,
        child: ElevatedButton.icon(
          onPressed: () => _showAddressDialog(context, purpose),
          icon: const Icon(Icons.add_location_alt),
          label: Text('Add ${purpose.label}'),
          style: ElevatedButton.styleFrom(
            backgroundColor: scheme.primary,
            foregroundColor: scheme.onPrimary,
            padding: const EdgeInsets.symmetric(vertical: 14),
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(12),
            ),
          ),
        ),
      ),
    );
  }

  void _showAddressInfoDialog(BuildContext context, ColorScheme scheme) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: scheme.onSurfaceVariant,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        title: Row(
          children: [
            Icon(Icons.info_outline, color: scheme.primary, size: 24),
            const SizedBox(width: 12),
            Text(
              'Address Information',
              style: TextStyle(
                fontSize: 18,
                fontWeight: FontWeight.bold,
                color: scheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _buildInfoRow(
              Icons.home,
              'Shipping Address',
              'Address for receiving packages/shipments',
              scheme,
            ),
            const SizedBox(height: 12),
            _buildInfoRow(
              Icons.agriculture,
              'Sender Address',
              'Origin address for goods (for seller)',
              scheme,
            ),
            const SizedBox(height: 16),
            Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: scheme.primary.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(8),
              ),
              child: Row(
                children: [
                  Icon(Icons.rule, color: scheme.primary, size: 20),
                  const SizedBox(width: 10),
                  Expanded(
                    child: Text(
                      'Min. 1 address per category\nMax. 10 addresses per category',
                      style: TextStyle(
                        fontSize: 13,
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
          size: 20,
          color: scheme.onSurfaceVariant,
        ),
        const SizedBox(width: 10),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                title,
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w600,
                  color: scheme.onSurfaceVariant,
                ),
              ),
              Text(
                desc,
                style: TextStyle(
                  fontSize: 12,
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
    AddressPurpose purpose,
  ) {
    return Container(
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: address.isPrimary
              ? scheme.primary
              : scheme.onSurfaceVariant,
          width: address.isPrimary ? 2 : 1,
        ),
      ),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Header: Label, Primary badge, Actions
            Row(
              children: [
                Icon(
                  _getPurposeIcon(address.purpose),
                  size: 20,
                  color: scheme.primary,
                ),
                const SizedBox(width: 8),
                Text(
                  address.displayLabel,
                  style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.bold,
                    color: scheme.onSurfaceVariant,
                  ),
                ),
                if (address.isPrimary) ...[
                  const SizedBox(width: 8),
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 2,
                    ),
                    decoration: BoxDecoration(
                      color: scheme.primary,
                      borderRadius: BorderRadius.circular(4),
                    ),
child: Text(
                       'Primary',
                      style: TextStyle(
                        fontSize: 10,
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
                        _showAddressDialog(context, address.purpose, address);
                        break;
                      case 'setPrimary':
                        _setPrimaryAddress(context, address, userId, purpose);
                        break;
                      case 'delete':
                        _deleteAddress(
                          context,
                          address,
                          totalAddresses,
                          purpose,
                        );
                        break;
                    }
                  },
                  itemBuilder: (context) => [
                    const PopupMenuItem(
                      value: 'edit',
                      child: Row(
                        children: [
                          Icon(Icons.edit, size: 18),
                          SizedBox(width: 8),
                          Text('Edit'),
                        ],
                      ),
                    ),
                    if (!address.isPrimary)
                      const PopupMenuItem(
                        value: 'setPrimary',
                        child: Row(
                          children: [
                            Icon(Icons.star, size: 18),
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
                              size: 18,
                              color: AppColors.error,
                            ),
                            const SizedBox(width: 8),
                            Text(
                              'Delete',
                              style: TextStyle(color: AppColors.error),
                            ),
                          ],
                        ),
                      ),
                  ],
                ),
              ],
            ),
            const SizedBox(height: 12),

            // Recipient/Sender Name & Phone
            Row(
              children: [
                Icon(
                  Icons.person_outline,
                  size: 14,
                  color: scheme.onSurfaceVariant,
                ),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    '${address.recipientName} • ${address.phone}',
                    style: TextStyle(
                      fontSize: 13,
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
                fontSize: 14,
                color: scheme.onSurfaceVariant,
              ),
            ),

            // Notes (if available)
            if (address.notes != null && address.notes!.isNotEmpty) ...[
              const SizedBox(height: 8),
              Container(
                padding: const EdgeInsets.all(8),
                decoration: BoxDecoration(
                  color: scheme.onSurfaceVariant,
                  borderRadius: BorderRadius.circular(6),
                ),
                child: Row(
                  children: [
                    Icon(Icons.note, size: 14, color: scheme.onSurfaceVariant),
                    const SizedBox(width: 6),
                    Expanded(
                      child: Text(
                        address.notes!,
                        style: TextStyle(
                          fontSize: 12,
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
                borderRadius: BorderRadius.circular(6),
                child: Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 10,
                    vertical: 6,
                  ),
                  decoration: BoxDecoration(
                    color: AppColors.success.withValues(alpha: 0.1),
                    borderRadius: BorderRadius.circular(6),
                    border: Border.all(
                      color: AppColors.success.withValues(alpha: 0.3),
                    ),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(
                        Icons.location_on,
                        size: 14,
                        color: AppColors.success,
                      ),
                      const SizedBox(width: 6),
                      Text(
                        'Pinpoint Location Saved',
                        style: TextStyle(
                          fontSize: 12,
                          fontWeight: FontWeight.w500,
                          color: AppColors.success,
                        ),
                      ),
                      const SizedBox(width: 4),
                      Icon(
                        Icons.chevron_right,
                        size: 14,
                        color: AppColors.success,
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

  IconData _getPurposeIcon(AddressPurpose purpose) {
    switch (purpose) {
      case AddressPurpose.shipping:
        return Icons.home; // Shipping destination
      case AddressPurpose.sender:
        return Icons.agriculture; // Sender origin (farm/warehouse)
    }
  }

  void _showAddressDialog(
    BuildContext context,
    AddressPurpose initialPurpose, [
    AddressEntity? address,
  ]) async {
    await showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (context) => AddressFormDialog(
        addressToEdit: address,
        // Lock purpose based on active tab (prevents user confusion)
        forcedPurpose: initialPurpose,
      ),
    );
  }

  void _deleteAddress(
    BuildContext context,
    AddressEntity address,
    int totalAddresses,
    AddressPurpose purpose,
  ) async {
    // Min 1 address per purpose
    if (totalAddresses <= 1) {
      AppSnackBar.showError(
        context,
        'Cannot delete. You need at least 1 ${purpose.label.toLowerCase()}.',
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
            style: TextButton.styleFrom(foregroundColor: AppColors.error),
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
    AddressPurpose purpose,
  ) async {
    if (address.isPrimary) {
      AppSnackBar.showInfo(
        context,
        'This ${purpose.label.toLowerCase()} is already set as primary',
      );
      return;
    }

    final repository = ref.read(addressRepositoryProvider);
    final result = await repository.setPrimaryAddress(address.id, userId);

    if (!context.mounted) return;

    if (result.isSuccess) {
      AppSnackBar.showSuccess(
        context,
        'Primary ${purpose.label.toLowerCase()} updated successfully',
      );
    } else {
      AppSnackBar.showError(
        context,
        result.error ?? 'Failed to set primary ${purpose.label.toLowerCase()}',
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
