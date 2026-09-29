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
import 'package:labuda/shared/widgets/app_image.dart';

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
    final scheme = Theme.of(context).colorScheme;
    final authState = ref.watch(authControllerProvider);

    if (authState is! AuthStateAuthenticated) {
      return _buildAuthRequired(context);
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
        color: scheme.surface,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(AppShape.r20)),
      ),
      child: Column(
        children: [
          _buildHeader(context),
          _buildSearchBar(context),
          _buildCreateNewForSaleButton(context),
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
                  return _buildEmptyState(context);
                }

                return _buildForSaleList(context, activeForSales);
              },
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (error, stack) => Center(
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Icon(
                      Icons.error_outline,
                      size: 48,
                      color: scheme.error,
                    ),
                    const SizedBox(height: 16),
                    Text(
                      'Error loading forSales',
                      style: TextStyle(
                        fontSize: AppType.s16,
                        color: scheme.onSurfaceVariant,
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

  Widget _buildAuthRequired(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      height: 300,
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(AppShape.r20)),
      ),
      child: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.lock_outline, size: 48, color: scheme.primary),
            SizedBox(height: 16),
            Text('Login Diperlukan'),
          ],
        ),
      ),
    );
  }

  Widget _buildHeader(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p20, vertical: AppMetrics.p16),
      decoration: BoxDecoration(
        border: Border(
          bottom: BorderSide(color: scheme.outlineVariant),
        ),
      ),
      child: Row(
        children: [
          const Text(
            'Pilih ForSale',
            style: TextStyle(fontSize: AppType.s18, fontWeight: FontWeight.bold),
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

  Widget _buildSearchBar(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.all(AppMetrics.p16),
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
          fillColor: scheme.surfaceContainerHighest,
          border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(AppShape.r12),
            borderSide: BorderSide.none,
          ),
        ),
        onChanged: (value) => setState(() => _searchQuery = value),
      ),
    );
  }

  Widget _buildCreateNewForSaleButton(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16),
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
            padding: const EdgeInsets.symmetric(vertical: AppMetrics.p12),
          ),
        ),
      ),
    );
  }

  Widget _buildEmptyState(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.inventory_2_outlined,
            size: 64,
            color: scheme.onSurfaceVariant,
          ),
          const SizedBox(height: 16),
          const Text(
            'Tidak Ada ForSale Aktif',
            style: TextStyle(fontSize: AppType.s18, fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 8),
          Text(
            'Buat forSale baru untuk mulai menjual',
            style: TextStyle(
              fontSize: AppType.s14,
              color: scheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildForSaleList(
    BuildContext context,
    List<ForSale> forSales,
  ) {
    return ListView.separated(
      padding: const EdgeInsets.all(AppMetrics.p16),
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
    final scheme = Theme.of(context).colorScheme;

    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(AppShape.r12),
      child: Container(
        padding: const EdgeInsets.all(AppMetrics.p12),
        decoration: BoxDecoration(
          color: isSelected
              ? scheme.primary.withValues(alpha: 0.12)
              : scheme.surfaceContainerHighest,
          borderRadius: BorderRadius.circular(AppShape.r12),
          border: Border.all(
            color: isSelected ? scheme.primary : Colors.transparent,
            width: 2,
          ),
        ),
        child: Row(
          children: [
            // Thumbnail
            ClipRRect(
              borderRadius: BorderRadius.circular(AppShape.r8),
              child: forSale.media.isNotEmptyUrls
          ? AppImage(
              imageUrl: forSale.media.firstUrl,
              width: 70,
              height: 70,
              fit: BoxFit.cover,
              errorWidget: _buildPlaceholder(context),
            )
                  : _buildPlaceholder(context),
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
                      fontSize: AppType.s15,
                    ),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: 4),
                  Text(
                    forSale.formattedPrice,
                    style: TextStyle(
                      color: Theme.of(context).colorScheme.primary,
                      fontWeight: FontWeight.bold,
                      fontSize: AppType.s16,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    'Stok: ${forSale.stock}',
                    style: TextStyle(
                      fontSize: AppType.s12,
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
                    ),
                  ),
                ],
              ),
            ),
            // Selection indicator
            if (isSelected)
              Icon(
                Icons.check_circle,
                color: Theme.of(context).colorScheme.primary,
                size: 24,
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildPlaceholder(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      width: 70,
      height: 70,
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(AppShape.r8),
      ),
      child: Icon(
        Icons.image_not_supported,
        size: 24,
        color: scheme.onSurfaceVariant,
      ),
    );
  }
}
