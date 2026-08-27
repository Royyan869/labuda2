/// Object Preview Card Widget
///
/// Reusable widget that displays live object preview with proper merge logic.
/// Extracted from content_detail_screen.dart to avoid duplication.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';
import 'package:labuda/shared/object/object_preview.dart';
import 'package:labuda/shared/object/object_preview_provider.dart';
import 'package:labuda/shared/object/object_reference.dart';

/// Reusable widget for displaying object preview with live data
///
/// BUSINESS RULE (STRICT ENFORCEMENT):
/// - STATUS  = HARD LIVE (TIDAK BOLEH pakai snapshot untuk UI)
/// - TITLE   = SOFT UPDATE (live jika ada, else snapshot)
/// - IMAGE   = SNAPSHOT INITIAL, LIVE BOLEH override
/// - PRICE   = HARD LIVE (always from live)
///
/// BATCHING SUPPORT:
/// - If preResolved is provided, use it directly (no provider call)
/// - Otherwise, fallback to individual objectPreviewProvider call
class ObjectPreviewCard extends ConsumerWidget {
  /// ShareReference containing snapshot data
  final ShareReference reference;

  /// Callback when card is tapped
  final VoidCallback? onTap;

  /// Whether to show the type badge
  final bool showTypeBadge;

  /// Pre-resolved live preview data (from batch provider)
  /// If provided, will be used directly without calling objectPreviewProvider
  final ObjectPreview? preResolved;

  const ObjectPreviewCard({
    super.key,
    required this.reference,
    this.onTap,
    this.showTypeBadge = true,
    this.preResolved,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // STEP 1: Use pre-resolved data if available (BATCH MODE)
    if (preResolved != null) {
      return _buildCardWithLivePreview(context, preResolved);
    }

    // STEP 2: Fallback to individual provider call (LEGACY MODE)
    // Convert ShareReference to ObjectReference for resolver
    final objectRef = ObjectReference(
      type: reference.objectType,
      id: reference.targetId,
    );

    // Watch live preview from object resolver
    final livePreviewAsync = ref.watch(objectPreviewProvider(objectRef));

    return livePreviewAsync.when(
      data: (livePreview) {
        return _buildCardWithLivePreview(context, livePreview);
      },
      loading: () => _buildCardWithSnapshot(context),
      error: (_, _) => _buildCardWithSnapshot(context),
    );
  }

  /// Build card with live preview data (HARD LIVE enforcement)
  Widget _buildCardWithLivePreview(
    BuildContext context,
    ObjectPreview? livePreview,
  ) {
    // =====================================================
    // OBJECT RESOLVER MERGE LOGIC (STRICT BUSINESS RULE ENFORCEMENT)
    // =====================================================
    // BUSINESS RULE:
    // - STATUS  = HARD LIVE (TIDAK BOLEH pakai snapshot untuk UI)
    // - TITLE   = SOFT UPDATE (live jika ada, else snapshot)
    // - IMAGE   = SNAPSHOT INITIAL, LIVE BOLEH override
    // - PRICE   = HARD LIVE (always from live)
    // =====================================================

    // STEP 1: TITLE (SOFT UPDATE)
    // - Live jika ada
    // - Else fallback ke snapshot
    final title = livePreview?.title ?? reference.preview.title;

    // STEP 2: IMAGE (LIVE CAN OVERRIDE SNAPSHOT)
    // - Live jika ada (update terbaru)
    // - Else fallback ke snapshot (initial render)
    final imageUrl = livePreview?.imageUrl ?? reference.preview.imageUrl;

    // STEP 3: STATUS (HARD LIVE - NO SNAPSHOT FOR UI)
    // - JANGAN gunakan snapshot untuk status
    // - Tampilkan "Memuat..." jika livePreview belum tersedia
    String? status;
    final String statusBadge;
    Color statusColor = AppColors.neutralGray400;

    if (livePreview != null) {
      // Live data tersedia → gunakan live status
      status = livePreview.status;
      statusBadge = _getStatusLabel(status);
      statusColor = _getStatusColorFromStatus(status);
    } else {
      // Live data belum tersedia → tampilkan loading
      // ❌ DILARANG menggunakan snapshot status untuk UI
      statusBadge = 'Memuat...';
      statusColor = AppColors.neutralGray400;
    }

    // STEP 4: PRICE (HARD LIVE)
    // - SELALU dari livePreview
    // - Null jika livePreview tidak available
    final price = livePreview?.price;

    // Determine availability based on live data ONLY (HARD LIVE)
    final isUnavailable =
        livePreview != null &&
        (livePreview.status == 'sold' ||
            livePreview.status == 'withdrawn' ||
            livePreview.status == 'ended' ||
            livePreview.status == 'cancelled' ||
            livePreview.status == 'deleted');

    // Show live data indicator
    final String? liveIndicator = livePreview != null ? ' • Live' : null;

    return Card(
      margin: EdgeInsets.zero,
      child: InkWell(
        // Disable navigation if target is unavailable
        onTap: isUnavailable ? null : onTap,
        borderRadius: BorderRadius.circular(8),
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Row(
            children: [
              // Thumbnail
              if (imageUrl != null)
                ClipRRect(
                  borderRadius: BorderRadius.circular(8),
                  child: Image.network(
                    imageUrl,
                    width: 60,
                    height: 60,
                    fit: BoxFit.cover,
                    errorBuilder: (context, error, stackTrace) => Container(
                      width: 60,
                      height: 60,
                      color: AppColors.neutralGray200,
                      child: const Icon(Icons.image_not_supported),
                    ),
                  ),
                ),
              if (imageUrl != null) const SizedBox(width: 12),
              // Info
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        if (showTypeBadge)
                          Text(
                            reference.targetType.displayName,
                            style: TextStyle(
                              fontSize: 12,
                              color: AppColors.primaryRed,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        if (showTypeBadge) const SizedBox(width: 8),
                        Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 6,
                            vertical: 2,
                          ),
                          decoration: BoxDecoration(
                            color: statusColor.withValues(alpha: 0.2),
                            borderRadius: BorderRadius.circular(4),
                          ),
                          child: Text(
                            statusBadge,
                            style: TextStyle(
                              fontSize: 10,
                              color: statusColor,
                              fontWeight: FontWeight.w500,
                            ),
                          ),
                        ),
                        if (liveIndicator != null)
                          Text(
                            liveIndicator,
                            style: TextStyle(
                              fontSize: 10,
                              color: AppColors.successGreen,
                              fontStyle: FontStyle.italic,
                            ),
                          ),
                      ],
                    ),
                    const SizedBox(height: 4),
                    Text(
                      title,
                      style: TextStyle(
                        fontWeight: FontWeight.w600,
                        color: isUnavailable ? AppColors.neutralGray400 : null,
                      ),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                    // Show price if available (HARD LIVE)
                    if (price != null)
                      Text(
                        'Rp ${price.toStringAsFixed(0).replaceAllMapped(RegExp(r'(\d{1,3})(?=(\d{3})+(?!\d))'), (Match m) => '${m[1]}.')}',
                        style: TextStyle(
                          fontSize: 12,
                          color: AppColors.primaryRed,
                          fontWeight: FontWeight.w500,
                        ),
                      ),
                  ],
                ),
              ),
              Icon(
                Icons.chevron_right,
                color: isUnavailable
                    ? AppColors.neutralGray300
                    : AppColors.neutralGray400,
              ),
            ],
          ),
        ),
      ),
    );
  }

  /// Fallback: use preview when live data is loading/error
  Widget _buildCardWithSnapshot(BuildContext context) {
    final isDeleted = reference.preview.isDeleted;
    final isSold = reference.preview.isSold;
    final isClosed = reference.preview.isClosed;
    final isUnavailable =
        !reference.preview.isAvailable || isDeleted || isSold || isClosed;

    String? statusBadge;
    Color statusColor = AppColors.neutralGray400;

    if (isDeleted) {
      statusBadge = 'Dihapus';
      statusColor = AppColors.neutralGray400;
    } else if (isSold) {
      statusBadge = 'Terjual';
      statusColor = AppColors.neutralGray400;
    } else if (isClosed) {
      statusBadge = 'Ditutup';
      statusColor = AppColors.neutralGray400;
    }

    return Card(
      margin: EdgeInsets.zero,
      child: InkWell(
        onTap: isUnavailable ? null : onTap,
        borderRadius: BorderRadius.circular(8),
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Row(
            children: [
              if (reference.preview.imageUrl != null)
                ClipRRect(
                  borderRadius: BorderRadius.circular(8),
                  child: Image.network(
                    reference.preview.imageUrl!,
                    width: 60,
                    height: 60,
                    fit: BoxFit.cover,
                    errorBuilder: (context, error, stackTrace) => Container(
                      width: 60,
                      height: 60,
                      color: AppColors.neutralGray200,
                      child: const Icon(Icons.image_not_supported),
                    ),
                  ),
                ),
              if (reference.preview.imageUrl != null) const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        if (showTypeBadge)
                          Text(
                            reference.targetType.displayName,
                            style: TextStyle(
                              fontSize: 12,
                              color: AppColors.primaryRed,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        if (statusBadge != null) ...[
                          if (showTypeBadge) const SizedBox(width: 8),
                          Container(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 6,
                              vertical: 2,
                            ),
                            decoration: BoxDecoration(
                              color: statusColor.withValues(alpha: 0.2),
                              borderRadius: BorderRadius.circular(4),
                            ),
                            child: Text(
                              statusBadge,
                              style: TextStyle(
                                fontSize: 10,
                                color: statusColor,
                                fontWeight: FontWeight.w500,
                              ),
                            ),
                          ),
                        ],
                      ],
                    ),
                    const SizedBox(height: 4),
                    Text(
                      reference.preview.title,
                      style: TextStyle(
                        fontWeight: FontWeight.w600,
                        color: isUnavailable ? AppColors.neutralGray400 : null,
                      ),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ],
                ),
              ),
              Icon(
                Icons.chevron_right,
                color: isUnavailable
                    ? AppColors.neutralGray300
                    : AppColors.neutralGray400,
              ),
            ],
          ),
        ),
      ),
    );
  }

  /// Get status label from status string
  String _getStatusLabel(String status) {
    switch (status) {
      case 'active':
        return 'Tersedia';
      case 'sold':
        return 'Terjual';
      case 'withdrawn':
        return 'Ditarik';
      case 'ended':
        return 'Berakhir';
      case 'cancelled':
        return 'Dibatalkan';
      case 'deleted':
        return 'Dihapus';
      case 'scheduled':
        return 'Terjadwal';
      case 'waitingSettlement':
        return 'Menunggu Pembayaran';
      default:
        return status;
    }
  }

  /// Get status color from status string
  Color _getStatusColorFromStatus(String status) {
    switch (status) {
      case 'active':
        return AppColors.successGreen;
      case 'sold':
      case 'ended':
      case 'deleted':
        return AppColors.neutralGray400;
      case 'withdrawn':
      case 'cancelled':
        return AppColors.koiOrange;
      case 'scheduled':
        return AppColors.primaryPurple;
      case 'waitingSettlement':
        return AppColors.koiOrange;
      default:
        return AppColors.neutralGray400;
    }
  }
}
