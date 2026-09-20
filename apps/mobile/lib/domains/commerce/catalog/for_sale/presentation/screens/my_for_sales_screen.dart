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
import 'package:labuda/shared/utils/media_extensions.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';

/// My ForSales Screen
///
/// Shows seller's own forSales with management actions:
/// - View forSale details
/// - Edit forSale
/// - Change status (active/inactive/sold)
/// - Delete forSale (soft delete - marks as withdrawn, hidden from default view)
class MyForSalesScreen extends ConsumerStatefulWidget {
  const MyForSalesScreen({super.key});

  @override
  ConsumerState<MyForSalesScreen> createState() => _MyForSalesScreenState();
}

class _MyForSalesScreenState extends ConsumerState<MyForSalesScreen> {
  /// Default to showing only active forSales (excludes withdrawn/deleted)
  ForSaleStatus? _statusFilter = ForSaleStatus.active;

  @override
  void initState() {
    super.initState();
    // Initial data fetch will happen in build via provider
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final authState = ref.watch(authControllerProvider);

    if (authState is! AuthStateAuthenticated) {
      return _buildAuthRequired(context);
    }

    final sellerId = authState.user.id;

    // Build params for fetching seller's forSales
    final params = SellerForSalesParams(
      sellerId: sellerId,
      page: 1,
      pageSize: 50,
    );

    final forSalesAsync = ref.watch(sellerForSalesProvider(params));

    return Scaffold(
      backgroundColor: isDark ? AppColors.darkGray900 : AppColors.neutralGray50,
      appBar: AppBar(
        title: const Text('For Sale Saya'),
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
          // Status filter dropdown
          Padding(
            padding: const EdgeInsets.only(right: 16),
            child: DropdownButtonHideUnderline(
              child: DropdownButton<ForSaleStatus>(
                value: _statusFilter,
                hint: const Text('Semua Status'),
                icon: const Icon(Icons.filter_list),
                onChanged: (status) {
                  setState(() => _statusFilter = status);
                },
                items: [
                  const DropdownMenuItem(
                    value: null,
                    child: Text('Semua Status'),
                  ),
                  ...ForSaleStatus.values.map(
                    (status) => DropdownMenuItem(
                      value: status,
                      child: Text(status.displayName),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
      body: forSalesAsync.when(
        data: (forSales) {
          // Apply status filter: show all if null, otherwise filter by selected status
          // Default is active, so withdrawn (deleted) For Sale are hidden by default
          final filteredForSales = _statusFilter == null
              ? forSales
              : forSales.where((l) => l.status == _statusFilter).toList();

          if (filteredForSales.isEmpty) {
            return _buildEmptyState(context, isDark);
          }

          return RefreshIndicator(
            onRefresh: () async {
              ref.invalidate(sellerForSalesProvider(params));
            },
            child: ListView.builder(
              padding: const EdgeInsets.all(16),
              itemCount: filteredForSales.length,
              itemBuilder: (context, index) {
                final forSale = filteredForSales[index];
                return _SellerForSaleManagementCard(
                  forSale: forSale,
                  onTap: () => _viewForSaleDetail(context, forSale.forSaleId),
                  onEdit: () => _editForSale(context, forSale),
                  onStatusChange: (status) =>
                      _changeStatus(context, forSale, status),
                  onDelete: () => _deleteForSale(context, forSale),
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
              const Icon(
                Icons.error_outline,
                size: 48,
                color: AppColors.primaryRed,
              ),
              const SizedBox(height: 16),
              Text(
                'Error loading For Sale',
                style: TextStyle(fontSize: 16, color: AppColors.neutralGray600),
              ),
              const SizedBox(height: 8),
              Text(
                error.toString(),
                style: TextStyle(fontSize: 12, color: AppColors.neutralGray400),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 16),
              ElevatedButton(
                onPressed: () {
                  ref.invalidate(sellerForSalesProvider(params));
                },
                child: const Text('Retry'),
              ),
            ],
          ),
        ),
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () => _createNewForSale(context),
        backgroundColor: AppColors.primaryRed,
        icon: const Icon(Icons.add, color: Colors.white),
        label: const Text(
          'Buat For Sale',
          style: TextStyle(color: Colors.white),
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
            const Icon(
              Icons.lock_outline,
              size: 64,
              color: AppColors.primaryRed,
            ),
            const SizedBox(height: 16),
            const Text(
              'Login Diperlukan',
              style: TextStyle(fontSize: 20, fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 8),
            const Text('Silakan login untuk mengelola For Sale Anda'),
          ],
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
            'Belum Ada For Sale',
            style: TextStyle(
              fontSize: 20,
              fontWeight: FontWeight.bold,
              color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Mulai buat For Sale untuk menjual produk Anda',
            style: TextStyle(fontSize: 14, color: AppColors.neutralGray600),
          ),
        ],
      ),
    );
  }

  void _viewForSaleDetail(BuildContext context, String forSaleId) {
    context.push(
      RoutePaths.forSaleDetail.replaceFirst(
        ':forSaleId',
        forSaleId,
      ),
    );
  }

  void _editForSale(BuildContext context, ForSale forSale) {
    context.push(
      RoutePaths.editForSale.replaceFirst(
        ':forSaleId',
        forSale.forSaleId,
      ),
    );
  }

  void _createNewForSale(BuildContext context) {
    context.push(RoutePaths.createForSale);
  }

  Future<void> _changeStatus(
    BuildContext context,
    ForSale forSale,
    ForSaleStatus newStatus,
  ) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Ubah Status For Sale'),
        content: Text(
          'Ubah status "${forSale.title}" menjadi ${newStatus.displayName}?',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Batal'),
          ),
          ElevatedButton(
            onPressed: () => Navigator.pop(context, true),
            style: ElevatedButton.styleFrom(
              backgroundColor: AppColors.primaryRed,
            ),
            child: const Text('Ya, Ubah'),
          ),
        ],
      ),
    );

    if (confirmed == true && mounted) {
      final controller = ref.read(forSaleControllerProvider);
      final result = await controller.updateForSaleStatus(
        forSale.forSaleId,
        newStatus,
      );

      if (!context.mounted) return;

      if (result.isSuccess) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Status berhasil diubah'),
            backgroundColor: AppColors.successGreen,
          ),
        );
        ref.invalidate(
          sellerForSalesProvider(
            SellerForSalesParams(sellerId: forSale.sellerId),
          ),
        );
        return;
      }

      // Canonical restriction dispatch. The presenter owns the whole
      // restriction family: `MARKET_AUTHORITY_REQUIRED` → seller renewal
      // navigation, `COMMERCE_RESTRICTED` → restriction snackbar. Unknown
      // codes are NOT consumed here and fall through to this screen's own
      // error handling below.
      final restrictionConsumed = CommerceRestrictionPresenter.handle(
        context,
        errorCode: result.errorCode,
        actionDescription: 'mengubah status For Sale',
      );
      if (restrictionConsumed) return;

      // Phase 0 honesty + Phase 2 routing: publish gate surfaces
      // SHIPPING_NOT_CONFIGURED when the seller has not yet linked any
      // shipping options to the For Sale. Offer two CTAs: one to set up
      // global options (if the catalog is empty), one to pick options for
      // this specific For Sale via the edit screen.
      if (result.errorCode == 'SHIPPING_NOT_CONFIGURED') {
        showDialog<void>(
          context: context,
          builder: (dialogCtx) => AlertDialog(
            title: const Text('Pengiriman Belum Dipilih'),
            content: const Text(
              'Pengiriman belum dipilih. For Sale belum bisa dipublish sampai '
              'Anda memilih opsi pengiriman untuk For Sale ini.\n\n'
              'Jika Anda belum memiliki opsi pengiriman, atur dulu di '
              'Pengaturan → Pengiriman. Jika sudah, buka Edit For Sale untuk '
              'memilih opsi yang berlaku untuk For Sale ini.',
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.of(dialogCtx).pop(),
                child: const Text('Tutup'),
              ),
              TextButton(
                onPressed: () {
                  Navigator.of(dialogCtx).pop();
                  Navigator.of(context).pushNamed(RoutePaths.sellerShipping);
                },
                child: const Text('Atur Opsi'),
              ),
              ElevatedButton(
                onPressed: () {
                  Navigator.of(dialogCtx).pop();
                  context.push(
                    RoutePaths.editForSale.replaceFirst(
                      ':fixedPriceSaleId',
                      forSale.forSaleId,
                    ),
                  );
                },
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppColors.primaryRed,
                  foregroundColor: Colors.white,
                ),
                child: const Text('Edit For Sale'),
              ),
            ],
          ),
        );
        return;
      }

      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Gagal mengubah status: ${result.error}'),
          backgroundColor: AppColors.primaryRed,
        ),
      );
    }
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
              backgroundColor: AppColors.primaryRed,
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
        result.fold(
          (error) {
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(
                content: Text('Gagal menghapus For Sale: $error'),
                backgroundColor: AppColors.primaryRed,
              ),
            );
          },
          (_) {
            ScaffoldMessenger.of(context).showSnackBar(
              const SnackBar(
                content: Text('For Sale berhasil dihapus'),
                backgroundColor: AppColors.successGreen,
              ),
            );
            // Invalidate to refresh
            ref.invalidate(
              sellerForSalesProvider(
                SellerForSalesParams(sellerId: forSale.sellerId),
              ),
            );
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
  final VoidCallback onEdit;
  final void Function(ForSaleStatus) onStatusChange;
  final VoidCallback onDelete;

  const _SellerForSaleManagementCard({
    required this.forSale,
    required this.onTap,
    required this.onEdit,
    required this.onStatusChange,
    required this.onDelete,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
        ),
      ),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Row(
            children: [
              // Thumbnail
              ClipRRect(
                borderRadius: BorderRadius.circular(8),
                child: forSale.media.isNotEmptyUrls
                    ? Image.network(
                        forSale.media.firstUrl,
                        width: 80,
                        height: 80,
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
                    // Title row with status
                    Row(
                      children: [
                        Expanded(
                          child: Text(
                            forSale.title,
                            style: const TextStyle(
                              fontWeight: FontWeight.w600,
                              fontSize: 15,
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
                      style: const TextStyle(
                        color: AppColors.primaryRed,
                        fontWeight: FontWeight.bold,
                        fontSize: 16,
                      ),
                    ),
                    const SizedBox(height: 4),
                    // Date
                    Text(
                      'Dibuat ${_formatDate(forSale.createdAt)}',
                      style: TextStyle(
                        fontSize: 12,
                        color: AppColors.neutralGray600,
                      ),
                    ),
                  ],
                ),
              ),
              // Action menu
              PopupMenuButton<String>(
                onSelected: (value) {
                  switch (value) {
                    case 'promote':
                      _navigateToPromotion(context, forSale);
                      break;
                    case 'edit':
                      onEdit();
                      break;
                    case 'activate':
                      onStatusChange(ForSaleStatus.active);
                      break;
                    case 'deactivate':
                      // Deactivate now means withdraw (remove from sale)
                      onStatusChange(ForSaleStatus.withdrawn);
                      break;
                    case 'mark_sold':
                      onStatusChange(ForSaleStatus.sold);
                      break;
                    case 'delete':
                      onDelete();
                      break;
                  }
                },
                itemBuilder: (context) => [
                  // Promote action (only for active forSales)
                  if (forSale.status == ForSaleStatus.active)
                    const PopupMenuItem(
                      value: 'promote',
                      child: Row(
                        children: [
                          Icon(
                            Icons.campaign,
                            size: 18,
                            color: AppColors.primaryRed,
                          ),
                          SizedBox(width: 12),
                          Text('Promosikan'),
                        ],
                      ),
                    ),
                  // Canonical: seller edit allowed IFF status == draft (active/sold/withdrawn are immutable)
                  if (forSale.status == ForSaleStatus.draft)
                    const PopupMenuItem(
                      value: 'edit',
                      child: Row(
                        children: [
                          Icon(Icons.edit, size: 18),
                          SizedBox(width: 12),
                          Text('Edit'),
                        ],
                      ),
                  ),
                  if (forSale.status != ForSaleStatus.active)
                    const PopupMenuItem(
                      value: 'activate',
                      child: Row(
                        children: [
                          Icon(
                            Icons.check_circle,
                            size: 18,
                            color: AppColors.successGreen,
                          ),
                          SizedBox(width: 12),
                          Text('Aktifkan'),
                        ],
                      ),
                    ),
                  if (forSale.status == ForSaleStatus.active)
                    const PopupMenuItem(
                      value: 'deactivate',
                      child: Row(
                        children: [
                          Icon(Icons.visibility_off, size: 18),
                          SizedBox(width: 12),
                          Text('Nonaktifkan'),
                        ],
                      ),
                    ),
                  if (forSale.status != ForSaleStatus.sold)
                    const PopupMenuItem(
                      value: 'mark_sold',
                      child: Row(
                        children: [
                          Icon(Icons.sell, size: 18),
                          SizedBox(width: 12),
                          Text('Tandai Terjual'),
                        ],
                      ),
                    ),
                  const PopupMenuItem(
                    value: 'delete',
                    child: Row(
                      children: [
                        Icon(
                          Icons.delete,
                          size: 18,
                          color: AppColors.primaryRed,
                        ),
                        SizedBox(width: 12),
                        Text(
                          'Hapus',
                          style: TextStyle(color: AppColors.primaryRed),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildPlaceholder(bool isDark) {
    return Container(
      width: 80,
      height: 80,
      decoration: BoxDecoration(
        color: AppColors.neutralGray200,
        borderRadius: BorderRadius.circular(8),
      ),
      child: const Icon(Icons.image_not_supported, size: 24),
    );
  }

  String _formatDate(DateTime date) {
    final now = DateTime.now();
    final difference = now.difference(date);

    if (difference.inDays == 0) {
      return 'hari ini';
    } else if (difference.inDays == 1) {
      return 'kemarin';
    } else if (difference.inDays < 7) {
      return '${difference.inDays} hari lalu';
    } else if (difference.inDays < 30) {
      final weeks = (difference.inDays / 7).floor();
      return '$weeks minggu lalu';
    } else {
      return '${date.day}/${date.month}/${date.year}';
    }
  }

  void _navigateToPromotion(BuildContext context, ForSale forSale) {
    // Canonical era: promotion is contract-based; seller manages contracts
    // (create + queue targets) from the canonical promotion list screen.
    // The legacy activation flow is purged.
    context.push(RoutePaths.sellerCanonicalPromotions);
  }
}

/// Status Badge Widget
class _StatusBadge extends StatelessWidget {
  final ForSaleStatus status;

  const _StatusBadge({required this.status});

  @override
  Widget build(BuildContext context) {
    Color color;
    String label;

    switch (status) {
      case ForSaleStatus.draft:
        color = AppColors.neutralGray600;
        label = 'Draft';
        break;
      case ForSaleStatus.active:
        color = AppColors.successGreen;
        label = 'Aktif';
        break;
      case ForSaleStatus.withdrawn:
        color = AppColors.neutralGray600;
        label = 'Ditarik';
        break;
      case ForSaleStatus.sold:
        color = AppColors.primaryRed;
        label = 'Terjual';
        break;
    }

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(6),
      ),
      child: Text(
        label,
        style: TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w600,
          color: color,
        ),
      ),
    );
  }
}
