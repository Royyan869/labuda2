/// My ForSales Screen
///
/// Seller-facing screen to view and manage their own forSales.
/// Shows only forSales owned by the current seller.
/// Deleted/withdrawn forSales are hidden by default.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/shared/domain/services/time_format_service.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';
import 'package:labuda/shared/widgets/empty_state.dart';
import 'package:labuda/shared/widgets/loading_indicator.dart';
import 'package:labuda/shared/widgets/page_error_state.dart';
import 'package:labuda/shared/utils/media_extensions.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/seller_management_row.dart';

/// Canonical My For Sale filter choices, in presentation order.
///
/// `null` is the synthetic "Semua Status" choice (no status restriction); the
/// remaining entries are the canonical [ForSaleStatus] values. This list is the
/// single source for tab identity, order, selected state and labels.
const List<ForSaleStatus?> _kMyForSaleFilters = <ForSaleStatus?>[
  null,
  ForSaleStatus.active,
  ForSaleStatus.sold,
  ForSaleStatus.withdrawn,
];

String _myForSaleFilterLabel(ForSaleStatus? status) =>
    status == null ? 'Semua Status' : status.displayName;

/// My ForSales Screen
///
/// Shows seller's own forSales with management actions:
/// - View forSale details
/// - Edit forSale
/// - Change status (active/sold/withdrawn)
/// - Delete forSale (soft delete - marks as withdrawn, hidden from default view)
class MyForSalesScreen extends ConsumerStatefulWidget {
  const MyForSalesScreen({super.key});

  @override
  ConsumerState<MyForSalesScreen> createState() => _MyForSalesScreenState();
}

class _MyForSalesScreenState extends ConsumerState<MyForSalesScreen>
    with SingleTickerProviderStateMixin {
  /// Default to showing only active forSales (excludes withdrawn/deleted).
  /// This is the single canonical filter state; the tabs only mutate it.
  ForSaleStatus? _statusFilter = ForSaleStatus.active;

  late final TabController _tabController;

  int get _selectedFilterIndex => _kMyForSaleFilters.indexOf(_statusFilter);

  @override
  void initState() {
    super.initState();
    _tabController = TabController(
      length: _kMyForSaleFilters.length,
      initialIndex: _selectedFilterIndex,
      vsync: this,
    );
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  /// Single params authority for this management surface. Every read,
  /// refresh, retry, and post-delete invalidation goes through this exact
  /// key (page 1, pageSize 50, owner-inventory opt-in), so an invalidation
  /// can never miss the watched provider via a mismatched page size.
  ///
  /// includeWithdrawn: true — this is the owner's own management surface,
  /// so the status filter must be able to reach the canonical `withdrawn`
  /// state. The default filter (active) still hides withdrawn items until
  /// the seller asks for them.
  SellerForSalesParams _params(String sellerId) => SellerForSalesParams(
    sellerId: sellerId,
    page: 1,
    pageSize: 50,
    includeWithdrawn: true,
  );

  /// Single canonical reload: initial load, pull-to-refresh, every retry,
  /// and post-delete invalidation are this one operation. Failure is never
  /// rethrown or rendered raw — it stays in the provider state and renders
  /// as [PageErrorState] (no data yet) or the inline refresh banner
  /// (existing data preserved).
  Future<void> _reload(SellerForSalesParams params) async {
    try {
      // The reloaded collection reaches the screen through the provider
      // state, not through this future — `.then((_) {})` adapts it to
      // `Future<void>` so the `unused_result` contract is satisfied while
      // failures still propagate to the `catch` below.
      await ref.refresh(sellerForSalesProvider(params).future).then((_) {});
    } catch (_) {
      // No-op: the failure remains observable via async.hasError with the
      // last-known-good collection preserved in async.value.
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final authState = ref.watch(authControllerProvider);

    if (authState is! AuthStateAuthenticated) {
      return _buildAuthRequired(context);
    }

    final sellerId = authState.user.id;
    final params = _params(sellerId);
    final forSalesAsync = ref.watch(sellerForSalesProvider(params));

    // Last-known-good collection. Present once the first request settles —
    // including during a refresh and after a failed refresh, where the
    // previous value is kept alongside the in-flight/failed state.
    final forSales = forSalesAsync.value ?? const <ForSale>[];

    return Scaffold(
      backgroundColor: scheme.surfaceContainerLowest,
      appBar: AppBar(
        title: const Text('For Sale Saya'),
        bottom: TabBar(
          controller: _tabController,
          isScrollable: true,
          tabs: [
            for (final filter in _kMyForSaleFilters)
              Tab(text: _myForSaleFilterLabel(filter)),
          ],
          onTap: (index) {
            // Single canonical filter state: the tab only selects it, the body
            // reads it. No second filter state exists.
            setState(() => _statusFilter = _kMyForSaleFilters[index]);
          },
        ),
      ),
      // LOADING FOUNDATION (owner-locked):
      // - No collection yet → first-load states only: LoadingIndicator,
      //   PageErrorState, or EmptyState.
      // - Collection present → it stays visible during refresh; the update
      //   indicator and refresh failure render inline, never as full-page
      //   loading/error.
      // SAFE-AREA-32: the body content owns the bottom system inset —
      // /seller/for-sale is a STANDALONE pushed route (ForSaleModule
      // top-level GoRoute, no shell bar), so no shell owns it. FAB
      // positioning stays the Scaffold endFloat authority, measured
      // OUTSIDE this SafeArea (never conflated with body inset).
      body: SafeArea(
        child: RefreshIndicator(
          onRefresh: () => _reload(params),
          child: CustomScrollView(
            physics: const AlwaysScrollableScrollPhysics(),
            slivers: [
              if (forSalesAsync.isLoading && forSales.isEmpty)
                // First request with no data → LoadingIndicator. Never
                // EmptyState (not yet loaded) and never a raw spinner.
                const SliverFillRemaining(
                  hasScrollBody: false,
                  child: Center(child: LoadingIndicator()),
                )
              else if (forSalesAsync.hasError && forSales.isEmpty)
                // CANONICAL page-level load error (PageErrorState): safe
                // localized copy only; the raw provider error never reaches
                // the screen. Retry re-executes the canonical reload.
                SliverFillRemaining(
                  hasScrollBody: false,
                  child: PageErrorState(onRetry: () => _reload(params)),
                )
              else
                ..._buildCollectionSlivers(
                  context,
                  forSalesAsync,
                  forSales,
                  params,
                ),
            ],
          ),
        ),
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () => _createNewForSale(context),
        backgroundColor: scheme.primary,
        icon: Icon(Icons.add, color: scheme.onPrimary),
        label: Text(
          context.l10n.createForSaleAction,
          style: TextStyle(color: scheme.onPrimary),
        ),
      ),
    );
  }

  /// Collection branch: runs only when a settled collection exists
  /// (possibly preserved across a failed refresh). Applies the local status
  /// filter, then renders EmptyState (zero-result success) or the rows with
  /// the inline refresh indicator / refresh-error banner on top.
  List<Widget> _buildCollectionSlivers(
    BuildContext context,
    AsyncValue<List<ForSale>> forSalesAsync,
    List<ForSale> forSales,
    SellerForSalesParams params,
  ) {
    // Apply status filter: show all if null, otherwise filter by selected status
    // Default is active, so withdrawn (deleted) For Sale are hidden by default
    final filteredForSales = _statusFilter == null
        ? forSales
        : forSales.where((l) => l.status == _statusFilter).toList();

    if (filteredForSales.isEmpty) {
      return [
        SliverFillRemaining(
          hasScrollBody: false,
          child: _buildEmptyState(context, collectionEmpty: forSales.isEmpty),
        ),
      ];
    }

    return [
      // Refresh with existing data: rows stay, update indication on top.
      if (forSalesAsync.isLoading)
        const SliverToBoxAdapter(child: LinearProgressIndicator(minHeight: 2)),
      // Refresh failure: rows stay, inline banner with retry that
      // re-executes the canonical reload. Never a full-page error here.
      if (forSalesAsync.hasError)
        SliverToBoxAdapter(child: _buildRefreshErrorBanner(params)),
      SliverPadding(
        padding: const EdgeInsets.all(AppMetrics.p16),
        sliver: SliverList(
          delegate: SliverChildBuilderDelegate((context, index) {
            final forSale = filteredForSales[index];
            return _SellerForSaleManagementCard(
              forSale: forSale,
              onTap: () => _viewForSaleDetail(context, forSale.forSaleId),
              onDelete: () => _deleteForSale(context, forSale),
            );
          }, childCount: filteredForSales.length),
        ),
      ),
    ];
  }

  /// Minimum bounded refresh-failure indication: persistent inline banner
  /// with safe localized copy ([pageErrorMessage]) and a retry action that
  /// re-executes the canonical reload. Not a new foundation — composition
  /// of canonical tokens for this screen, matching the Home / Chat / Coin /
  /// Search / Seller Auctions refresh banners.
  Widget _buildRefreshErrorBanner(SellerForSalesParams params) {
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
              onPressed: () => _reload(params),
              child: Text(l10n.retryAction),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildAuthRequired(BuildContext context) {
    return Scaffold(
      body: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.lock_outline,
              size: AppIconSize.display,
              color: Theme.of(context).colorScheme.primary,
            ),
            const SizedBox(height: 16),
            Text(
              'Login Diperlukan',
              style: context.typeRoles.titleProminent.copyWith(
                fontWeight: FontWeight.bold,
              ),
            ),
            const SizedBox(height: 8),
            const Text('Silakan login untuk mengelola For Sale Anda'),
          ],
        ),
      ),
    );
  }

  /// Two distinct states, one renderer:
  /// - the seller has never listed anything → first-use empty with the ONE
  ///   primary action (create). The persistent FAB stays the screen-level
  ///   affordance, this is the state-level action.
  /// - listings exist but the active status tab matched none → filter empty
  ///   with a reset that returns to "Semua Status".
  Widget _buildEmptyState(
    BuildContext context, {
    required bool collectionEmpty,
  }) {
    final l10n = context.l10n;

    if (!collectionEmpty) {
      return EmptyState(
        icon: Icons.filter_alt_off_outlined,
        title: l10n.emptySearchTitle,
        subtitle: l10n.emptySearchMessage,
        actionLabel: l10n.resetFilterAction,
        onAction: () {
          setState(() {
            _statusFilter = null;
            _tabController.index = _selectedFilterIndex;
          });
        },
      );
    }

    return EmptyState(
      icon: Icons.inventory_2_outlined,
      title: l10n.myForSalesTitle,
      subtitle: l10n.firstUseForSaleMessage,
      actionLabel: l10n.createForSaleAction,
      onAction: () => _createNewForSale(context),
    );
  }

  void _viewForSaleDetail(BuildContext context, String forSaleId) {
    context.push(
      RoutePaths.forSaleDetail.replaceFirst(':forSaleId', forSaleId),
    );
  }

  void _createNewForSale(BuildContext context) {
    context.push(RoutePaths.createForSale);
  }

  Future<void> _deleteForSale(BuildContext context, ForSale forSale) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Hapus For Sale'),
        content: Text(
          'Apakah Anda yakin ingin menghapus "${forSale.title}"? Tindakan ini tidak dapat dibatalkan.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Batal'),
          ),
          ElevatedButton(
            onPressed: () => Navigator.pop(context, true),
            style: ElevatedButton.styleFrom(
              backgroundColor: Theme.of(context).colorScheme.error,
            ),
            child: const Text('Ya, Hapus'),
          ),
        ],
      ),
    );

    if (confirmed == true && mounted) {
      final controller = ref.read(forSaleControllerProvider);
      final result = await controller.deleteForSale(forSale.forSaleId);

      if (mounted) {
        await result.fold(
          (_) async {
            AppSnackBar.showError(
              context,
              'Gagal menghapus For Sale. Coba lagi.',
            );
          },
          (_) async {
            AppSnackBar.showSuccess(context, 'For Sale berhasil dihapus');
            // Canonical invalidation: the single reload path with the single
            // params authority — the watched key (page 1, pageSize 50,
            // owner-inventory opt-in) is rebuilt, so the collection is fresh.
            // A reload failure is not swallowed: it renders as the inline
            // refresh banner with retry, data preserved.
            await _reload(_params(forSale.sellerId));
          },
        );
      }
    }
  }
}

/// ForSale Card for My ForSales
class _SellerForSaleManagementCard extends StatelessWidget {
  final ForSale forSale;
  final VoidCallback onTap;
  final VoidCallback onDelete;

  const _SellerForSaleManagementCard({
    required this.forSale,
    required this.onTap,
    required this.onDelete,
  });

  @override
  Widget build(BuildContext context) {
    return SellerManagementRow(
      onTap: onTap,
      leading: SellerManagementThumbnail(
        imageUrl: forSale.media.isNotEmptyUrls ? forSale.media.firstUrl : null,
      ),
      title: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Title row with status
          Row(
            children: [
              Expanded(
                child: Text(
                  forSale.title,
                  style: context.typeRoles.titleCompact.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              _StatusBadge(status: forSale.status),
            ],
          ),
          const SizedBox(height: 4),
          // Price
          Text(
            forSale.formattedPrice,
            style: context.typeRoles.titleCompact.copyWith(
              color: Theme.of(context).colorScheme.primary,
              fontWeight: FontWeight.bold,
            ),
          ),
          const SizedBox(height: 4),
          // Date
          Text(
            'Dibuat ${const TimeFormatService().formatTimeAgo(forSale.createdAt)}',
            style: context.typeRoles.labelMicro.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
      trailing: PopupMenuButton<String>(
        onSelected: (value) {
          if (value == 'delete') {
            onDelete();
          }
        },
        itemBuilder: (context) => [
          PopupMenuItem(
            value: 'delete',
            child: Row(
              children: [
                Icon(
                  Icons.delete,
                  size: AppIconSize.action,
                  color: Theme.of(context).colorScheme.error,
                ),
                SizedBox(width: 12),
                Text(
                  'Hapus',
                  style: TextStyle(color: Theme.of(context).colorScheme.error),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// Status Badge Widget
class _StatusBadge extends StatelessWidget {
  final ForSaleStatus status;

  const _StatusBadge({required this.status});

  @override
  Widget build(BuildContext context) {
    // Single label authority: the canonical ForSaleStatus presentation getter.
    // This widget only owns the status -> color mapping, never the label.
    final label = status.displayName;

    Color color;
    final scheme = Theme.of(context).colorScheme;
    switch (status) {
      case ForSaleStatus.active:
        color = context.statusColors.success;
        break;
      case ForSaleStatus.withdrawn:
        color = scheme.onSurfaceVariant;
        break;
      case ForSaleStatus.sold:
        color = scheme.primary;
        break;
    }

    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p8,
        vertical: AppMetrics.p4,
      ),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(AppShape.r6),
      ),
      child: Text(
        label,
        style: context.typeRoles.labelMicro.copyWith(
          fontWeight: FontWeight.w600,
          color: color,
        ),
      ),
    );
  }
}
