import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/shared.dart';
import 'package:hishumi/domains/user/profile/domain/entities/address_entity.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/notifiers/address_notifier.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/state/address_state.dart';
import 'package:hishumi/domains/user/profile/presentation/widgets/address_form_dialog.dart';

/// Address List Screen — the account's ONE address book.
///
/// CANONICAL DESIGN (single page, no tabs):
/// - One list of every saved address. Exactly one is Primary.
/// - Add / edit goes through AddressFormDialog — the only address form.
/// - Min 1 address, max 10 addresses per account.
class AddressListScreen extends ConsumerStatefulWidget {
  const AddressListScreen({super.key});

  @override
  ConsumerState<AddressListScreen> createState() => _AddressListScreenState();
}

class _AddressListScreenState extends ConsumerState<AddressListScreen> {
  /// Guards the single initial-load trigger for this surface.
  bool _loadScheduled = false;

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
              icon: const Icon(Icons.arrow_back, semanticLabel: 'Kembali'),
              onPressed: () => Navigator.of(context).pop(),
            ),
          ),
          body: const Center(child: Text('Please login to continue')),
        ),
      );
    }

    final userId = currentUser.id;

    // THE canonical Address Book authority (shared with checkout). The
    // screen keeps no second list read.
    final addressState = ref.watch(addressProvider);
    final addressesAsync = addressState.addresses;
    final addresses = addressesAsync.value ?? const <AddressEntity>[];

    // THE single initial-load trigger for this surface. The canonical
    // authority does not fetch in build(), so this schedules the one request
    // on first mount (and covers late auth hydration).
    if (!_loadScheduled) {
      _loadScheduled = true;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) {
          ref.read(addressProvider.notifier).loadAddresses(userId);
        }
      });
    }

    return PopScope(
      canPop: true,
      child: Scaffold(
        appBar: AppBar(
          title: const Text('Addresses'),
          leading: IconButton(
            icon: const Icon(Icons.arrow_back, semanticLabel: 'Kembali'),
            onPressed: () => Navigator.of(context).pop(),
          ),
          actions: [
            IconButton(
              icon: Icon(Icons.info_outline, color: scheme.onSurfaceVariant),
              tooltip: 'Info',
              // Canonical Page Info surface: the page owns only the trigger.
              onPressed: () => AppDialog.info(
                context: context,
                title: 'Address Information',
                content: _buildAddressInfoContent(context, scheme),
                closeLabel: 'Got it',
              ),
            ),
          ],
        ),
        // LOADING FOUNDATION (owner-locked):
        // - No addresses yet → first-load states only: LoadingIndicator,
        //   PageErrorState, or EmptyState.
        // - Addresses present → they stay visible during refresh; the update
        //   indicator and refresh failure render inline, never as full-page
        //   loading/error and never as an empty list.
        body: SafeArea(
          child: Column(
            children: [
              Expanded(
                child: RefreshIndicator(
                  onRefresh: () => _reload(userId),
                  child: CustomScrollView(
                    physics: const AlwaysScrollableScrollPhysics(),
                    slivers: _buildSlivers(
                      context,
                      addressState,
                      addresses,
                      userId,
                      scheme,
                    ),
                  ),
                ),
              ),
              if (addressesAsync.hasValue)
                _buildStickyAddButton(context, addresses),
            ],
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
      body: const Center(child: LoadingIndicator()),
    );
  }

  /// Canonical reload for this surface: initial-load retry, pull-to-refresh,
  /// and every post-mutation reload use this one operation on the canonical
  /// authority. Existing addresses remain visible; failure preserves them and
  /// surfaces the inline refresh error.
  Future<void> _reload(String userId) async {
    await ref.read(addressProvider.notifier).loadAddresses(userId);
  }

  /// Canonical page-state composition over the Address Book authority.
  List<Widget> _buildSlivers(
    BuildContext context,
    AddressState addressState,
    List<AddressEntity> addresses,
    String userId,
    ColorScheme scheme,
  ) {
    final addressesAsync = addressState.addresses;

    if (addressesAsync.isLoading && addresses.isEmpty) {
      // First request with no data → LoadingIndicator. Never EmptyState
      // (not yet loaded) and never a raw spinner.
      return const [
        SliverFillRemaining(
          hasScrollBody: false,
          child: Center(child: LoadingIndicator()),
        ),
      ];
    }

    if (addressesAsync.hasError && addresses.isEmpty) {
      // CANONICAL page-level load error (PageErrorState): controlled
      // localized copy only — the raw backend error never reaches the screen.
      // Retry re-executes the canonical load.
      return [
        SliverFillRemaining(
          hasScrollBody: false,
          child: PageErrorState(onRetry: () => _reload(userId)),
        ),
      ];
    }

    if (addresses.isEmpty) {
      // Successful zero-result: the ONE canonical EmptyState. First-use
      // action preserved (at most one primary action).
      return [
        SliverFillRemaining(
          hasScrollBody: false,
          child: _buildEmptyState(context),
        ),
      ];
    }

    // One account, one book: the account-level minimum is 1 address.
    final canDelete = addresses.length > 1;

    return [
      // Refresh with existing data: rows stay, update indication on top.
      if (addressState.isRefreshing)
        const SliverToBoxAdapter(child: LinearProgressIndicator(minHeight: 2)),
      // Refresh failure: rows stay, inline banner with retry that
      // re-executes the canonical load. Never a full-page error here.
      if (addressState.refreshError != null)
        SliverToBoxAdapter(child: _buildRefreshErrorBanner(userId)),
      SliverPadding(
        padding: const EdgeInsets.only(
          left: AppMetrics.p16,
          right: AppMetrics.p16,
          top: AppMetrics.p16,
          bottom: AppMetrics.p16,
        ),
        sliver: SliverList(
          delegate: SliverChildBuilderDelegate((context, index) {
            final address = addresses[index];
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
          }, childCount: addresses.length),
        ),
      ),
    ];
  }

  Widget _buildEmptyState(BuildContext context) {
    // First-use: ONE primary action inside the canonical state. The sticky
    // Add button below stays the affordance for a populated address book.
    return EmptyState(
      icon: Icons.location_off_outlined,
      title: context.l10n.emptyAddressTitle,
      subtitle: context.l10n.emptyAddressMessage,
      actionLabel: context.l10n.addAddressAction,
      onAction: () => _showAddressDialog(context),
    );
  }

  /// Minimum bounded refresh-failure indication: persistent inline banner
  /// with safe localized copy and a retry action. Not a new foundation —
  /// composition of canonical tokens, matching the established banners.
  Widget _buildRefreshErrorBanner(String userId) {
    final scheme = Theme.of(context).colorScheme;
    final l10n = context.l10n;
    return Semantics(
      container: true,
      liveRegion: true,
      child: Container(
        width: double.infinity,
        margin: const EdgeInsets.fromLTRB(
          AppMetrics.p16,
          AppMetrics.p12,
          AppMetrics.p16,
          AppMetrics.p4,
        ),
        padding: const EdgeInsets.symmetric(
          horizontal: AppMetrics.p12,
          vertical: AppMetrics.p8,
        ),
        decoration: BoxDecoration(
          color: scheme.errorContainer,
          borderRadius: BorderRadius.circular(AppShape.r12),
          border: Border.all(color: scheme.error),
        ),
        child: Row(
          children: [
            Icon(
              Icons.refresh_outlined,
              size: AppIconSize.action,
              color: scheme.onErrorContainer,
            ),
            const SizedBox(width: AppMetrics.p8),
            Expanded(
              child: Text(
                l10n.pageErrorMessage,
                style: context.typeRoles.bodyDense.copyWith(
                  color: scheme.onErrorContainer,
                ),
              ),
            ),
            TextButton(
              onPressed: () => _reload(userId),
              child: Text(l10n.retryAction),
            ),
          ],
        ),
      ),
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
      padding: const EdgeInsets.only(
        left: AppMetrics.p16,
        right: AppMetrics.p16,
        top: AppMetrics.p12,
        bottom: AppMetrics.p12,
      ),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        border: Border(
          top: BorderSide(color: Theme.of(context).colorScheme.outlineVariant),
        ),
      ),
      child: SizedBox(
        width: double.infinity,
        child: ElevatedButton.icon(
          onPressed: () => _showAddressDialog(context),
          icon: const Icon(Icons.add_location_alt),
          label: Text(context.l10n.addAddressAction),
          style: ElevatedButton.styleFrom(
            padding: const EdgeInsets.symmetric(vertical: AppMetrics.p16),
          ),
        ),
      ),
    );
  }

  /// Content for the canonical Page Info surface ([AppDialog.info]).
  Widget _buildAddressInfoContent(BuildContext context, ColorScheme scheme) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _buildInfoRow(
          Icons.home,
          'Address',
          'A saved place where you receive or ship from',
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
              Icon(
                Icons.rule,
                color: scheme.primary,
                size: AppIconSize.action,
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Text(
                  'The primary address is used as the default destination '
                  'and as the origin for every product.\n'
                  'Min. 1 address, max. 10 per account.',
                  style: context.typeRoles.bodyDense.copyWith(
                    color: scheme.onSurfaceVariant,
                  ),
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildInfoRow(
    IconData icon,
    String title,
    String desc,
    ColorScheme scheme,
  ) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(icon, size: AppIconSize.action, color: scheme.onSurfaceVariant),
        const SizedBox(width: 10),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                title,
                style: context.typeRoles.bodyDense.copyWith(
                  fontWeight: FontWeight.w600,
                  color: scheme.onSurface,
                ),
              ),
              Text(
                desc,
                style: context.typeRoles.labelMicro.copyWith(
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
                  Icons.location_on_outlined,
                  size: AppIconSize.action,
                  color: scheme.primary,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    _labelFor(address),
                    style: context.typeRoles.titleCompact.copyWith(
                      fontWeight: FontWeight.bold,
                      color: scheme.onSurface,
                    ),
                    overflow: TextOverflow.ellipsis,
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
                      style: context.typeRoles.labelMicro.copyWith(
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
                        _deleteAddress(context, address, totalAddresses);
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
                    if (totalAddresses >= 2 && !address.isPrimary)
                      const PopupMenuItem(
                        value: 'setPrimary',
                        child: Row(
                          children: [
                            Icon(Icons.star, size: AppIconSize.action),
                            SizedBox(width: 8),
                            Text('Jadikan alamat utama'),
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
                              style: TextStyle(
                                color: context.statusColors.error,
                              ),
                            ),
                          ],
                        ),
                      ),
                  ],
                ),
              ],
            ),
            const SizedBox(height: 10),

            // Recipient Name & Phone
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
                    style: context.typeRoles.bodyDense.copyWith(
                      fontWeight: FontWeight.w500,
                      color: scheme.onSurfaceVariant,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),

            // Full Address — detail Address/Location authority (wraps).
            AddressLocationText(
              location: address.fullAddress,
              mode: AddressLocationMode.detail,
              style: context.typeRoles.bodyDense.copyWith(
                color: scheme.onSurfaceVariant,
              ),
            ),

            // Notes (if available)
            if (address.notes != null && address.notes!.isNotEmpty) ...[
              const SizedBox(height: 8),
              Container(
                padding: const EdgeInsets.all(AppMetrics.p8),
                decoration: BoxDecoration(
                  color: scheme.surfaceContainerHighest,
                  borderRadius: BorderRadius.circular(AppShape.r6),
                ),
                child: Row(
                  children: [
                    Icon(
                      Icons.note,
                      size: AppIconSize.inlineGlyph,
                      color: scheme.onSurfaceVariant,
                    ),
                    const SizedBox(width: 6),
                    Expanded(
                      child: Text(
                        address.notes!,
                        style: context.typeRoles.labelMicro.copyWith(
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
                      color: context.statusColors.success.withValues(
                        alpha: 0.3,
                      ),
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
                        style: context.typeRoles.labelMicro.copyWith(
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

  /// Recognition label: the user's nickname, falling back to the recipient name.
  String _labelFor(AddressEntity address) {
    final nickname = address.nickname?.trim();
    if (nickname != null && nickname.isNotEmpty) return nickname;
    return address.recipientName;
  }

  Future<void> _showAddressDialog(
    BuildContext context, [
    AddressEntity? address,
  ]) async {
    await AppBottomSheetBase.show<bool>(
      context: context,
      content: AddressFormDialog(addressToEdit: address),
    );
    // The form persists through the canonical authority, which reloads the
    // shared collection itself — no page-local reload path.
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
        'Tidak dapat menghapus. Anda harus memiliki minimal 1 alamat.',
      );
      return;
    }

    if (address.isPrimary) {
      AppSnackBar.showWarning(
        context,
        'Tidak dapat menghapus alamat utama. Jadikan alamat lain sebagai utama terlebih dahulu.',
      );
      return;
    }

    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Delete Address'),
        content: Text(
          'Are you sure you want to delete this address?\n\n${_labelFor(address)}\n${address.fullAddress}',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(context, true),
            style: TextButton.styleFrom(
              foregroundColor: context.statusColors.error,
            ),
            child: const Text('Delete'),
          ),
        ],
      ),
    );

    if (confirmed != true) return;

    final userId = ref.read(authenticatedUserProvider)?.id;
    if (userId == null) return;

    final ok = await ref
        .read(addressProvider.notifier)
        .deleteAddress(address.id, userId);

    if (!context.mounted) return;

    if (ok) {
      AppSnackBar.showSuccess(context, 'Alamat berhasil dihapus');
    } else {
      AppSnackBar.showError(context, 'Gagal menghapus alamat. Coba lagi.');
    }
  }

  void _setPrimaryAddress(
    BuildContext context,
    AddressEntity address,
    String userId,
  ) async {
    if (address.isPrimary) {
      AppSnackBar.showInfo(context, 'Alamat ini sudah menjadi utama');
      return;
    }

    final ok = await ref
        .read(addressProvider.notifier)
        .setPrimaryAddress(address.id, userId);

    if (!context.mounted) return;

    if (ok) {
      AppSnackBar.showSuccess(context, 'Alamat utama diperbarui');
    } else {
      AppSnackBar.showError(
        context,
        'Gagal menetapkan alamat utama. Coba lagi.',
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
    final updatedAddress = address.copyWith(
      latitude: latitude,
      longitude: longitude,
      updatedAt: DateTime.now(),
    );

    final ok = await ref
        .read(addressProvider.notifier)
        .updateAddress(updatedAddress);

    if (!context.mounted) return;

    if (ok) {
      AppSnackBar.showSuccess(context, 'Titik lokasi berhasil diperbarui');
    } else {
      AppSnackBar.showError(
        context,
        'Gagal memperbarui titik lokasi. Coba lagi.',
      );
    }
  }
}
