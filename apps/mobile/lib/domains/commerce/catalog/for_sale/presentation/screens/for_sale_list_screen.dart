/// ForSale List Screen
///
/// Presentation layer - displays marketplace fixed-price forSales (a
/// sibling sale channel to Auction, over Product).
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/widgets/for_sale_card.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';
import 'package:labuda/shared/widgets/app_bottom_sheet_base.dart';
import 'package:labuda/shared/widgets/empty_state.dart';
import 'package:labuda/shared/widgets/page_error_state.dart';
import 'package:labuda/core/core.dart';

/// ForSale List Screen - Public marketplace
class ForSaleListScreen extends ConsumerStatefulWidget {
  const ForSaleListScreen({super.key});

  @override
  ConsumerState<ForSaleListScreen> createState() => _ForSaleListScreenState();
}

class _ForSaleListScreenState extends ConsumerState<ForSaleListScreen> {
  final _scrollController = ScrollController();
  final _searchController = TextEditingController();

  // Filter state (UI only, passed to providers)
  ForSaleStatus? _selectedStatus;
  double? _minPrice;
  double? _maxPrice;

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_onScroll);
  }

  @override
  void dispose() {
    _scrollController.dispose();
    _searchController.dispose();
    super.dispose();
  }

  void _onScroll() {
    if (_scrollController.position.pixels >=
        _scrollController.position.maxScrollExtent - 200) {
      // Load more triggered by scroll
      // TODO: Implement pagination
    }
  }

  void _onFilterChanged() {
    setState(() {});
  }

  /// Clears the resettable query + filters. One deterministic reset used by
  /// the filter-empty state's "Atur Ulang" action.
  void _resetFilters() {
    _searchController.clear();
    setState(() {
      _selectedStatus = null;
      _minPrice = null;
      _maxPrice = null;
    });
  }

  void _showFilterBottomSheet() {
    AppBottomSheetBase.show<void>(
      context: context,
      title: 'Filter',
      content: _ForSaleFilterSheet(
        selectedStatus: _selectedStatus,
        minPrice: _minPrice,
        maxPrice: _maxPrice,
        onApply: (status, minPrice, maxPrice) {
          setState(() {
            _selectedStatus = status;
            _minPrice = minPrice;
            _maxPrice = maxPrice;
          });
          _onFilterChanged();
        },
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    // Build params for provider
    final params = ForSalesParams(
      status: _selectedStatus,
      searchQuery: _searchController.text.isNotEmpty
          ? _searchController.text
          : null,
      minPrice: _minPrice,
      maxPrice: _maxPrice,
    );

    // Semantic split: query/filter active vs a genuinely empty collection.
    final hasActiveFilter =
        params.status != null ||
        params.searchQuery != null ||
        params.minPrice != null ||
        params.maxPrice != null;

    return PopScope(
      canPop: true,
      child: Scaffold(
        backgroundColor: scheme.surfaceContainerLowest,
        appBar: AppBar(
          title: const Text('For Sale'),
          leading: IconButton(
            icon: const Icon(Icons.arrow_back, semanticLabel: 'Kembali'),
            onPressed: () => Navigator.of(context).pop(),
          ),
          actions: [
            IconButton(
              onPressed: _showFilterBottomSheet,
              icon: const Icon(Icons.tune),
              tooltip: 'Filter',
            ),
          ],
        ),
        body: SafeArea(
          child: Column(
            children: [
              // Search bar
              Padding(
                padding: const EdgeInsets.all(AppMetrics.p16),
                child: TextField(
                  controller: _searchController,
                  decoration: AppTheme.searchDecoration(
                    scheme,
                    hintText: 'Search For Sale...',
                  ).copyWith(
                    prefixIcon: const Icon(Icons.search),
                    suffixIcon: _searchController.text.isNotEmpty
                        ? IconButton(
                            icon: const Icon(Icons.clear, semanticLabel: 'Bersihkan'),
                            onPressed: () {
                              _searchController.clear();
                              _onFilterChanged();
                            },
                          )
                        : null,
                  ),
                  onChanged: (_) => _onFilterChanged(),
                ),
              ),
              // For Sale list
              Expanded(
                child: _ForSalesList(
                  params: params,
                  hasActiveFilter: hasActiveFilter,
                  onResetFilter: _resetFilters,
                  scrollController: _scrollController,
                  onRefresh: () => _onFilterChanged(),
                  onForSaleTap: (forSale) {
                    context.push('/for-sale/${forSale.forSaleId}');
                  },
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Internal widget for For Sale list
class _ForSalesList extends ConsumerWidget {
  final ForSalesParams params;

  /// True when the visible result set is narrowed by an active query/filter,
  /// so an empty result means "nothing matched", not "nothing exists".
  final bool hasActiveFilter;

  /// Clears that query/filter — owned by the screen that holds the state.
  final VoidCallback onResetFilter;

  final ScrollController scrollController;
  final VoidCallback onRefresh;
  final void Function(ForSale) onForSaleTap;

  const _ForSalesList({
    required this.params,
    required this.hasActiveFilter,
    required this.onResetFilter,
    required this.scrollController,
    required this.onRefresh,
    required this.onForSaleTap,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final forSalesAsync = ref.watch(forSalesProvider(params));
    final forSales = forSalesAsync.asData?.value ?? const <ForSale>[];

    // CANONICAL LAYOUT: shared 2-column grid (public commerce surface).
    return RefreshIndicator(
      onRefresh: () async {
        onRefresh();
        await ref.read(forSalesProvider(params).future);
      },
      child: CustomScrollView(
        controller: scrollController,
        physics: const AlwaysScrollableScrollPhysics(),
        slivers: [
          CommerceMarketplaceGrid(
            itemCount: forSales.length,
            isLoading: forSalesAsync.isLoading,
            error: forSalesAsync.hasError ? forSalesAsync.error : null,
            itemBuilder: (context, index) {
              final forSale = forSales[index];
              return ForSaleCard(
                forSale: forSale,
                onTap: () => onForSaleTap(forSale),
              );
            },
            emptyBuilder: (context) {
              final l10n = context.l10n;
              // Filter/search empty: the marketplace has listings, this
              // query/filter simply matched none — offer the reset.
              if (hasActiveFilter) {
                return EmptyState(
                  icon: Icons.filter_alt_off_outlined,
                  title: l10n.emptySearchTitle,
                  subtitle: l10n.emptySearchMessage,
                  actionLabel: l10n.resetFilterAction,
                  onAction: onResetFilter,
                );
              }
              return EmptyState(
                icon: Icons.storefront_outlined,
                title: l10n.emptyForSaleTitle,
                subtitle: l10n.emptyCollectionMessage,
              );
            },
            // CANONICAL page-level error (PageErrorState): safe localized
            // copy only, the raw [error] never reaches the screen.
            errorBuilder: (context, error, stackTrace) => PageErrorState(
              onRetry: onRefresh,
            ),
          ),
        ],
      ),
    );
  }
}

/// Filter bottom sheet (UI only)
class _ForSaleFilterSheet extends StatefulWidget {
  final ForSaleStatus? selectedStatus;
  final double? minPrice;
  final double? maxPrice;
  final void Function(ForSaleStatus?, double?, double?) onApply;

  const _ForSaleFilterSheet({
    required this.selectedStatus,
    required this.minPrice,
    required this.maxPrice,
    required this.onApply,
  });

  @override
  State<_ForSaleFilterSheet> createState() => _ForSaleFilterSheetState();
}

class _ForSaleFilterSheetState extends State<_ForSaleFilterSheet> {
  late ForSaleStatus? _status;
  late double? _minPrice;
  late double? _maxPrice;

  @override
  void initState() {
    super.initState();
    _status = widget.selectedStatus;
    _minPrice = widget.minPrice;
    _maxPrice = widget.maxPrice;
  }

  @override
  Widget build(BuildContext context) {
    return Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          // Status filter
          DropdownButtonFormField<ForSaleStatus>(
            initialValue: _status,
            // Border/fill come from `inputDecorationTheme` (AppTheme) — the
            // one form-field authority.
            decoration: const InputDecoration(labelText: 'Status'),
            items: ForSaleStatus.values.map((status) {
              return DropdownMenuItem(value: status, child: Text(status.name));
            }).toList(),
            onChanged: (value) => setState(() => _status = value),
          ),
          const SizedBox(height: 16),
          // Apply button
          SizedBox(
            width: double.infinity,
            child: ElevatedButton(
              onPressed: () {
                widget.onApply(_status, _minPrice, _maxPrice);
                Navigator.pop(context);
              },
              child: const Text('Apply'),
            ),
          ),
        ],
    );
  }
}
