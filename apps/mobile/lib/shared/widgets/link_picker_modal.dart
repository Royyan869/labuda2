import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/widgets/link_picker/link_list_item.dart';
import 'package:labuda/shared/widgets/link_picker/link_picker_tab_label.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/for_sale.dart';

/// Link Picker Modal with tabs for Fixed-price sale, Auction
/// Provides a unified interface for selecting links to attach
///
/// Shows only seller's own content:
/// - Fixed-price sale: active fixed-price sales
/// - Auction: active + scheduled auctions
class LinkPickerModal extends ConsumerStatefulWidget {
  final Function(ShareReference shareRef) onLinkSelected;

  const LinkPickerModal({super.key, required this.onLinkSelected});

  static Future<void> show({
    required BuildContext context,
    required Function(ShareReference shareRef) onLinkSelected,
  }) {
    return showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (context) => LinkPickerModal(onLinkSelected: onLinkSelected),
    );
  }

  @override
  ConsumerState<LinkPickerModal> createState() => _LinkPickerModalState();
}

class _LinkPickerModalState extends ConsumerState<LinkPickerModal>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;
  final TextEditingController _searchController = TextEditingController();
  String _searchQuery = '';

  // Batch selection state
  final List<ShareReference> _selectedItems = [];

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
  }

  @override
  void dispose() {
    _tabController.dispose();
    _searchController.dispose();
    super.dispose();
  }

  // Count selected items by type
  int _countByType(ShareTargetType type) {
    return _selectedItems.where((item) => item.targetType == type).length;
  }

  // Check if item is selected
  bool _isSelected(String id) {
    return _selectedItems.any((item) => item.targetId == id);
  }

  // Toggle item selection
  void _toggleSelection(ShareReference shareRef, String id) {
    setState(() {
      if (_isSelected(id)) {
        // Remove from selection
        _selectedItems.removeWhere((item) => item.targetId == id);
      } else {
        // Add to selection
        _selectedItems.add(shareRef);
      }
    });
  }

  // Add all selected items
  void _addSelectedItems() {
    if (_selectedItems.isEmpty) return;

    for (final shareRef in _selectedItems) {
      widget.onLinkSelected(shareRef);
    }

    // Clear selections and close modal
    setState(() => _selectedItems.clear());
    Navigator.pop(context);
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      height: MediaQuery.of(context).size.height * 0.9,
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : Colors.white,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(20)),
      ),
      child: Column(
        children: [
          // Handle bar
          Container(
            margin: const EdgeInsets.only(top: 12),
            width: 40,
            height: 4,
            decoration: BoxDecoration(
              color: isDark
                  ? AppColors.neutralGray600
                  : AppColors.neutralGray300,
              borderRadius: BorderRadius.circular(2),
            ),
          ),

          // Header
          Padding(
            padding: const EdgeInsets.all(16),
            child: Row(
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Pilih Link',
                        style: TextStyle(
                          fontSize: 20,
                          fontWeight: FontWeight.bold,
                          color: isDark
                              ? Colors.white
                              : AppColors.neutralGray900,
                        ),
                      ),
                      const SizedBox(height: 2),
                      Text(
                        _selectedItems.isEmpty
                            ? 'Pilih item, lalu tap "Tambahkan Link"'
                            : '${_selectedItems.length} item dipilih',
                        style: TextStyle(
                          fontSize: 12,
                          color: _selectedItems.isEmpty
                              ? (isDark
                                    ? AppColors.neutralGray400
                                    : AppColors.neutralGray600)
                              : AppColors.primaryRed,
                          fontWeight: _selectedItems.isEmpty
                              ? FontWeight.normal
                              : FontWeight.w600,
                        ),
                      ),
                    ],
                  ),
                ),
                IconButton(
                  onPressed: () => Navigator.pop(context),
                  icon: Icon(
                    Icons.close,
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray600,
                  ),
                ),
              ],
            ),
          ),

          // Tab Bar
          Container(
            decoration: BoxDecoration(
              border: Border(
                bottom: BorderSide(
                  color: isDark
                      ? AppColors.darkGray700
                      : AppColors.neutralGray200,
                  width: 1,
                ),
              ),
            ),
            child: TabBar(
              controller: _tabController,
              labelColor: AppColors.primaryRed,
              unselectedLabelColor: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
              indicatorColor: AppColors.primaryRed,
              tabs: [
                Tab(
                  child: buildLinkPickerTabLabel(
                    'Produk Dijual',
                    _countByType(ShareTargetType.forSale),
                  ),
                ),
                Tab(
                  child: buildLinkPickerTabLabel(
                    'Lelang',
                    _countByType(ShareTargetType.auction),
                  ),
                ),
              ],
            ),
          ),

          // Search Bar
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
            child: TextField(
              controller: _searchController,
              decoration: InputDecoration(
                hintText: 'Cari...',
                prefixIcon: const Icon(Icons.search),
                filled: true,
                fillColor: isDark
                    ? AppColors.darkGray700
                    : AppColors.neutralGray100,
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(12),
                  borderSide: BorderSide.none,
                ),
              ),
              onChanged: (value) {
                setState(() => _searchQuery = value.toLowerCase());
              },
            ),
          ),

          // Tab Content
          Expanded(
            child: TabBarView(
              controller: _tabController,
              children: [
                _ListingTab(
                  searchQuery: _searchQuery,
                  selectedItems: _selectedItems,
                  onToggleSelection: _toggleSelection,
                ),
                _AuctionTab(
                  searchQuery: _searchQuery,
                  selectedItems: _selectedItems,
                  onToggleSelection: _toggleSelection,
                ),
              ],
            ),
          ),

          // Add Button
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: isDark ? AppColors.darkGray800 : Colors.white,
              border: Border(
                top: BorderSide(
                  color: isDark
                      ? AppColors.darkGray700
                      : AppColors.neutralGray200,
                  width: 1,
                ),
              ),
            ),
            child: SafeArea(
              top: false,
              child: SizedBox(
                width: double.infinity,
                child: ElevatedButton(
                  onPressed: _selectedItems.isEmpty ? null : _addSelectedItems,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppColors.primaryRed,
                    foregroundColor: Colors.white,
                    padding: const EdgeInsets.symmetric(vertical: 16),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                  child: Text(
                    'Tambahkan Link',
                    style: const TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// Listing Tab
class _ListingTab extends ConsumerWidget {
  final String searchQuery;
  final List<ShareReference> selectedItems;
  final Function(ShareReference, String) onToggleSelection;

  const _ListingTab({
    required this.searchQuery,
    required this.selectedItems,
    required this.onToggleSelection,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Get current user ID
    final currentUserId = ref.watch(currentUserIdProvider);

    // Watch seller for-sales
    final listingsAsync = currentUserId.isEmpty
        ? const AsyncValue.data(<ForSale>[])
        : ref.watch(
            sellerForSalesProvider(
              SellerForSalesParams(sellerId: currentUserId),
            ),
          );

    return listingsAsync.when(
      data: (listings) {
        // Filter: only active listings (not sold, not withdrawn) + search query
        final filteredListings = listings.where((l) {
          // AVAILABILITY ENFORCEMENT: Only show listings that are available for commerce
          // This prevents sellers from attaching sold/withdrawn listings to content
          final isAvailable = l.status == ForSaleStatus.active;
          final matchesSearch =
              searchQuery.isEmpty ||
              l.title.toLowerCase().contains(searchQuery);
          return isAvailable && matchesSearch;
        }).toList();

        if (filteredListings.isEmpty) {
          return _EmptyState(
            icon: Icons.collections_bookmark_outlined,
            message: 'Tidak ada listing',
          );
        }

        return ListView.builder(
          padding: const EdgeInsets.all(16),
          itemCount: filteredListings.length,
          itemBuilder: (context, index) {
            final listing = filteredListings[index];
            final isSelected = selectedItems.any(
              (item) =>
                  item.targetType == ShareTargetType.forSale &&
                  item.targetId == listing.forSaleId,
            );

            return LinkListItem(
              imageUrl: listing.media.firstOrNull?.originalUrl,
              title: listing.title,
              subtitle: listing.description,
              price: listing.price > 0
                  ? 'Rp${listing.price.toStringAsFixed(0)}'
                  : null,
              badge: listing.status.displayName,
              badgeColor: AppColors.primaryGreen,
              isSelected: isSelected,
              onTap: () => onToggleSelection(
                ShareReference.forSale(
                  forSaleId: listing.forSaleId,
                  title: listing.title,
                  imageUrl: listing.media.firstOrNull?.originalUrl,
                  isAvailable: true, // Filtered to active only
                  isSold: false, // Filtered out sold
                  isDeleted: false, // No deleted status in ForSaleStatus
                ),
                listing.forSaleId,
              ),
            );
          },
        );
      },
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, stack) => _EmptyState(
        icon: Icons.error_outline,
        message: 'Gagal memuat listing',
      ),
    );
  }
}

/// Auction Tab
class _AuctionTab extends ConsumerWidget {
  final String searchQuery;
  final List<ShareReference> selectedItems;
  final Function(ShareReference, String) onToggleSelection;

  const _AuctionTab({
    required this.searchQuery,
    required this.selectedItems,
    required this.onToggleSelection,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // TODO: Implement myAuctionsProvider in auction module
    return _EmptyState(
      icon: Icons.gavel_outlined,
      message: 'Fitur lelang belum tersedia',
    );
  }
}

/// Empty State Widget
class _EmptyState extends StatelessWidget {
  final IconData icon;
  final String message;

  const _EmptyState({required this.icon, required this.message});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              icon,
              size: 64,
              color: isDark
                  ? AppColors.neutralGray600
                  : AppColors.neutralGray400,
            ),
            const SizedBox(height: 16),
            Text(
              message,
              style: TextStyle(
                fontSize: 16,
                color: isDark
                    ? AppColors.neutralGray500
                    : AppColors.neutralGray600,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
