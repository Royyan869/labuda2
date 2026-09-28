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
import 'package:labuda/shared/widgets/empty_state.dart';
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

  void _showFilterBottomSheet() {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (context) => _ForSaleFilterSheet(
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

    return PopScope(
      canPop: true,
      child: Scaffold(
        backgroundColor: scheme.surfaceContainerLowest,
        appBar: AppBar(
          title: const Text('For Sale'),
          leading: IconButton(
            icon: const Icon(Icons.arrow_back),
            onPressed: () => Navigator.of(context).pop(),
          ),
          backgroundColor: scheme.surface,
          foregroundColor: scheme.onSurface,
          elevation: AppElevation.none,
          surfaceTintColor: Colors.transparent,
          scrolledUnderElevation: 0,
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
                  decoration: InputDecoration(
                    hintText: 'Search For Sale...',
                    prefixIcon: const Icon(Icons.search),
                    suffixIcon: _searchController.text.isNotEmpty
                        ? IconButton(
                            icon: const Icon(Icons.clear),
                            onPressed: () {
                              _searchController.clear();
                              _onFilterChanged();
                            },
                          )
                        : null,
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(AppShape.r12),
                    ),
                  ),
                  onChanged: (_) => _onFilterChanged(),
                ),
              ),
              // For Sale list
              Expanded(
                child: _ForSalesList(
                  params: params,
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
  final ScrollController scrollController;
  final VoidCallback onRefresh;
  final void Function(ForSale) onForSaleTap;

  const _ForSalesList({
    required this.params,
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
            emptyBuilder: (context) => const EmptyState(
              icon: Icons.storefront_outlined,
              title: 'Belum ada for sale',
              subtitle: 'Coba kata kunci atau filter lain.',
            ),
            errorBuilder: (context, error, stackTrace) => EmptyState.error(
              title: 'Data belum bisa dimuat.',
              subtitle: 'Periksa koneksi kamu lalu coba lagi.',
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
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(AppShape.r20)),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          // Status filter
          DropdownButtonFormField<ForSaleStatus>(
            initialValue: _status,
            decoration: const InputDecoration(
              labelText: 'Status',
              border: OutlineInputBorder(),
            ),
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
      ),
    );
  }
}
