/// ForSale Picker Bottom Sheet
///
/// Allows seller to select from their active forSales when responding to buyer requests.
/// Only shows forSales that are:
/// - Owned by the current seller
/// - Active status (not sold, not withdrawn)
/// - Valid for sharing

library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/utils/media_extensions.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';

/// Picker intent determines which canonical ID the caller expects.
///
/// PASS_21B: the auction-source-product intent was removed. Auction
/// creation must never be sourced from a For Sale item — this picker is now
/// exclusively for for-sale attachment surfaces (chat, seller
/// response comments).
enum ForSalePickerIntent {
  /// For-sale attachment surfaces such as chat and seller response
  /// comments. The returned canonical ID is [ForSalePickerSelection.forSaleId].
  forSaleAttachment,
}

/// Explicit selection result for the for-sale picker.
///
/// The underlying for-sale item remains available for rich UI context, but callers
/// must read the canonical ID that matches their intent.
class ForSalePickerSelection {
  final ForSale forSale;
  final String forSaleId;
  final String? productId;
  final String title;
  final String? imageUrl;

  const ForSalePickerSelection({
    required this.forSale,
    required this.forSaleId,
    required this.productId,
    required this.title,
    this.imageUrl,
  });

  factory ForSalePickerSelection.fromForSale(ForSale forSale) {
    return ForSalePickerSelection(
      forSale: forSale,
      forSaleId: forSale.forSaleId,
      productId: forSale.productId,
      title: forSale.title,
      imageUrl: forSale.media.isNotEmptyUrls ? forSale.media.firstUrl : null,
    );
  }

  bool get hasProductId => productId != null && productId!.isNotEmpty;
}

/// Callback when a picker selection is made.
typedef ForSaleSelectedCallback =
    void Function(ForSalePickerSelection selection);

extension ForSalePickerIntentX on ForSalePickerIntent {
  bool matches(ForSale forSale) {
    if (forSale.status != ForSaleStatus.active) {
      return false;
    }

    switch (this) {
      case ForSalePickerIntent.forSaleAttachment:
        // PASS_21C: the forSaleType != 'auction' check was removed —
        // ForSale no longer models a "type" at all (the backend never
        // emits one; every real ForSale is definitionally fixed-price).
        return forSale.forSaleId.isNotEmpty;
    }
  }

  String? selectedId(ForSale forSale) {
    switch (this) {
      case ForSalePickerIntent.forSaleAttachment:
        return forSale.forSaleId;
    }
  }
}

/// For Sale Picker Bottom Sheet
///
/// Shows a modal bottom sheet with seller's active for-sale items.
/// Sellers can select an existing item or create a new one.
class ForSalePickerBottomSheet extends ConsumerStatefulWidget {
  /// Picker intent determines which canonical ID is returned.
  final ForSalePickerIntent intent;

  /// Optional pre-selected for-sale ID for attachment intents.
  final String? selectedForSaleId;

  /// Callback when a for-sale item is selected.
  final ForSaleSelectedCallback onForSaleSelected;

  /// Callback when "Create New For Sale" is tapped
  final VoidCallback? onCreateNewForSale;

  const ForSalePickerBottomSheet({
    super.key,
    required this.intent,
    this.selectedForSaleId,
    required this.onForSaleSelected,
    this.onCreateNewForSale,
  });

  @override
  ConsumerState<ForSalePickerBottomSheet> createState() =>
      _ForSalePickerBottomSheetState();

  /// Show the for-sale picker bottom sheet
  static Future<void> show(
    BuildContext context, {
    required ForSalePickerIntent intent,
    String? selectedForSaleId,
    required ForSaleSelectedCallback onForSaleSelected,
    VoidCallback? onCreateNewForSale,
  }) {
    return showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (context) => ForSalePickerBottomSheet(
        intent: intent,
        selectedForSaleId: selectedForSaleId,
        onForSaleSelected: onForSaleSelected,
        onCreateNewForSale: onCreateNewForSale,
      ),
    );
  }
}

class _ForSalePickerBottomSheetState
    extends ConsumerState<ForSalePickerBottomSheet> {
  final TextEditingController _searchController = TextEditingController();
  String _searchQuery = '';

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final authState = ref.watch(authControllerProvider);

    if (authState is! AuthStateAuthenticated) {
      return _buildAuthRequired(context, isDark);
    }

    final sellerId = authState.user.id;

    // Fetch seller's active forSales
    final params = SellerForSalesParams(
      sellerId: sellerId,
      page: 1,
      pageSize: 50,
    );

    final forSalesAsync = ref.watch(sellerForSalesProvider(params));

    return Container(
      height: MediaQuery.of(context).size.height * 0.7,
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(20)),
      ),
      child: Column(
        children: [
          _buildHeader(context, isDark),
          _buildSearchBar(context, isDark),
          _buildCreateNewForSaleButton(context, isDark),
          Expanded(
            child: forSalesAsync.when(
              data: (forSales) {
                // Filter only active forSales and apply search
                final activeForSales = forSales
                    .where(
                      (l) =>
                          widget.intent.matches(l) &&
                          (_searchQuery.isEmpty ||
                              l.title.toLowerCase().contains(
                                _searchQuery.toLowerCase(),
                              )),
                    )
                    .toList();

                if (activeForSales.isEmpty) {
                  return _buildEmptyState(context, isDark);
                }

                return _buildForSaleList(context, isDark, activeForSales);
              },
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (error, stack) => Center(
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    const Icon(
                      Icons.error_outline,
                      size: 48,
                      color: AppColors.primaryRed,
                    ),
                    const SizedBox(height: 16),
                    Text(
                      'Error loading forSales',
                      style: TextStyle(
                        fontSize: 16,
                        color: AppColors.neutralGray600,
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildAuthRequired(BuildContext context, bool isDark) {
    return Container(
      height: 300,
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(20)),
      ),
      child: const Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.lock_outline, size: 48, color: AppColors.primaryRed),
            SizedBox(height: 16),
            Text('Login Diperlukan'),
          ],
        ),
      ),
    );
  }

  Widget _buildHeader(BuildContext context, bool isDark) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 16),
      decoration: BoxDecoration(
        border: Border(
          bottom: BorderSide(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
          ),
        ),
      ),
      child: Row(
        children: [
          Text(
            'Pilih ForSale',
            style: TextStyle(
              fontSize: 18,
              fontWeight: FontWeight.bold,
              color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
            ),
          ),
          const Spacer(),
          IconButton(
            icon: const Icon(Icons.close),
            onPressed: () => Navigator.of(context).pop(),
          ),
        ],
      ),
    );
  }

  Widget _buildSearchBar(BuildContext context, bool isDark) {
    return Padding(
      padding: const EdgeInsets.all(16),
      child: TextField(
        controller: _searchController,
        decoration: InputDecoration(
          hintText: 'Cari forSale...',
          prefixIcon: const Icon(Icons.search),
          suffixIcon: _searchQuery.isNotEmpty
              ? IconButton(
                  icon: const Icon(Icons.clear),
                  onPressed: () {
                    _searchController.clear();
                    setState(() => _searchQuery = '');
                  },
                )
              : null,
          filled: true,
          fillColor: isDark ? AppColors.darkGray700 : AppColors.neutralGray100,
          border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(12),
            borderSide: BorderSide.none,
          ),
        ),
        onChanged: (value) => setState(() => _searchQuery = value),
      ),
    );
  }

  Widget _buildCreateNewForSaleButton(BuildContext context, bool isDark) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: SizedBox(
        width: double.infinity,
        child: OutlinedButton.icon(
          onPressed: () {
            Navigator.of(context).pop();
            widget.onCreateNewForSale?.call();
          },
          icon: const Icon(Icons.add),
          label: const Text('Buat ForSale Baru'),
          style: OutlinedButton.styleFrom(
            foregroundColor: AppColors.primaryRed,
            side: const BorderSide(color: AppColors.primaryRed),
            padding: const EdgeInsets.symmetric(vertical: 12),
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(12),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildEmptyState(BuildContext context, bool isDark) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.inventory_2_outlined,
            size: 64,
            color: AppColors.neutralGray400,
          ),
          const SizedBox(height: 16),
          Text(
            'Tidak Ada ForSale Aktif',
            style: TextStyle(
              fontSize: 18,
              fontWeight: FontWeight.bold,
              color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Buat forSale baru untuk mulai menjual',
            style: TextStyle(fontSize: 14, color: AppColors.neutralGray600),
          ),
        ],
      ),
    );
  }

  Widget _buildForSaleList(
    BuildContext context,
    bool isDark,
    List<ForSale> forSales,
  ) {
    return ListView.separated(
      padding: const EdgeInsets.all(16),
      itemCount: forSales.length,
      separatorBuilder: (_, _) => const SizedBox(height: 12),
      itemBuilder: (context, index) {
        final forSale = forSales[index];
        final isSelected = switch (widget.intent) {
          ForSalePickerIntent.forSaleAttachment =>
            forSale.forSaleId == widget.selectedForSaleId,
        };

        return _ForSaleTile(
          forSale: forSale,
          isSelected: isSelected,
          onTap: () {
            widget.onForSaleSelected(
              ForSalePickerSelection.fromForSale(forSale),
            );
            Navigator.of(context).pop();
          },
        );
      },
    );
  }
}

/// ForSale Tile for Picker
class _ForSaleTile extends StatelessWidget {
  final ForSale forSale;
  final bool isSelected;
  final VoidCallback onTap;

  const _ForSaleTile({
    required this.forSale,
    required this.isSelected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(12),
      child: Container(
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: isSelected
              ? AppColors.primaryRed.withValues(alpha: 0.1)
              : (isDark ? AppColors.darkGray700 : AppColors.neutralGray50),
          borderRadius: BorderRadius.circular(12),
          border: Border.all(
            color: isSelected ? AppColors.primaryRed : Colors.transparent,
            width: 2,
          ),
        ),
        child: Row(
          children: [
            // Thumbnail
            ClipRRect(
              borderRadius: BorderRadius.circular(8),
              child: forSale.media.isNotEmptyUrls
                  ? Image.network(
                      forSale.media.firstUrl,
                      width: 70,
                      height: 70,
                      fit: BoxFit.cover,
                      errorBuilder: (context, error, stackTrace) =>
                          _buildPlaceholder(isDark),
                    )
                  : _buildPlaceholder(isDark),
            ),
            const SizedBox(width: 12),
            // Content
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    forSale.title,
                    style: const TextStyle(
                      fontWeight: FontWeight.w600,
                      fontSize: 15,
                    ),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: 4),
                  Text(
                    forSale.formattedPrice,
                    style: const TextStyle(
                      color: AppColors.primaryRed,
                      fontWeight: FontWeight.bold,
                      fontSize: 16,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    'Stok: ${forSale.stock}',
                    style: TextStyle(
                      fontSize: 12,
                      color: AppColors.neutralGray600,
                    ),
                  ),
                ],
              ),
            ),
            // Selection indicator
            if (isSelected)
              const Icon(
                Icons.check_circle,
                color: AppColors.primaryRed,
                size: 24,
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildPlaceholder(bool isDark) {
    return Container(
      width: 70,
      height: 70,
      decoration: BoxDecoration(
        color: AppColors.neutralGray200,
        borderRadius: BorderRadius.circular(8),
      ),
      child: const Icon(Icons.image_not_supported, size: 24),
    );
  }
}
