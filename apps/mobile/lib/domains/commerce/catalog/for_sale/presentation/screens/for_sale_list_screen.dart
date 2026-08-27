/// ForSale List Screen
///
/// Presentation layer - displays marketplace fixed-price forSales (a
/// sibling sale channel to Auction, over Product).
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/widgets/for_sale_card.dart';

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
    final isDark = Theme.of(context).brightness == Brightness.dark;

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
        backgroundColor: isDark
            ? AppColors.darkGray900
            : AppColors.neutralGray50,
        appBar: AppBar(
          title: const Text('Listings'),
          leading: IconButton(
            icon: const Icon(Icons.arrow_back),
            onPressed: () => Navigator.of(context).pop(),
          ),
          backgroundColor: isDark
              ? AppColors.darkGray800
              : AppColors.neutralWhite,
          foregroundColor: isDark
              ? AppColors.neutralWhite
              : AppColors.neutralGray900,
          elevation: 0,
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
                padding: const EdgeInsets.all(16),
                child: TextField(
                  controller: _searchController,
                  decoration: InputDecoration(
                    hintText: 'Search listings...',
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
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                  onChanged: (_) => _onFilterChanged(),
                ),
              ),
              // Listings list
              Expanded(
                child: _ListingsList(
                  params: params,
                  scrollController: _scrollController,
                  onRefresh: () => _onFilterChanged(),
                  onListingTap: (listing) {
                    context.push('/for-sale/${listing.forSaleId}');
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

/// Internal widget for listings list
class _ListingsList extends ConsumerWidget {
  final ForSalesParams params;
  final ScrollController scrollController;
  final VoidCallback onRefresh;
  final void Function(ForSale) onListingTap;

  const _ListingsList({
    required this.params,
    required this.scrollController,
    required this.onRefresh,
    required this.onListingTap,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final listingsAsync = ref.watch(forSalesProvider(params));

    return listingsAsync.when(
      data: (listings) {
        if (listings.isEmpty) {
          return const Center(child: Text('No listings found'));
        }

        return RefreshIndicator(
          onRefresh: () async {
            onRefresh();
            await ref.read(forSalesProvider(params).future);
          },
          child: ListView.builder(
            controller: scrollController,
            padding: const EdgeInsets.all(16),
            itemCount: listings.length,
            itemBuilder: (context, index) {
              final listing = listings[index];
              return ForSaleCard(
                listing: listing,
                onTap: () => onListingTap(listing),
              );
            },
          ),
        );
      },
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, stack) => Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Text('Data belum bisa dimuat.'),
            const SizedBox(height: 16),
            ElevatedButton(
              onPressed: onRefresh,
              child: const Text('Coba Lagi'),
            ),
          ],
        ),
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
      padding: const EdgeInsets.all(16),
      decoration: const BoxDecoration(
        color: AppColors.neutralWhite,
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
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
