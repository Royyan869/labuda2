import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';

/// Widget to display extra info for search results
///
/// AUCTION RICHNESS ACTIVATION (renderer-only, no schema change):
/// For [SearchResultType.auction] rows, surfaces metadata that the
/// repository mapper already preserves but no consumer reads:
///   - status   → "Live" / "Soon" / "Ended" badge
///   - bidCount → small gavel chip when > 0
///   - currentBid (existence) → "Bid Rp X" vs "Start Rp X" prefix
///   - endAt    → "Ends in Nh/Nm" countdown when status == active
///                AND remaining ≤ 24h
/// All auction reads are gated on [SearchResult.type] == auction —
/// listings, users, and content rows are unchanged.
class SearchResultExtraInfo extends StatelessWidget {
  final SearchResult result;

  const SearchResultExtraInfo({super.key, required this.result});

  bool get _isAuction => result.type == SearchResultType.auction;

  /// Check if result has extra info to display
  bool get hasExtraInfo {
    if (result.metadata.containsKey('price')) return true;
    if (result.metadata.containsKey('likesCount')) return true;
    if (result.metadata.containsKey('commentsCount')) return true;
    if (_isAuction) {
      if (_auctionStatusLabel() != null) return true;
      if (_auctionBidCount() > 0) return true;
      if (_endingSoonLabel() != null) return true;
    }
    return false;
  }

  String? _auctionStatusLabel() {
    final status = result.metadata['status'];
    if (status is! String) return null;
    switch (status) {
      case 'active':
        return 'Live';
      case 'scheduled':
        return 'Soon';
      case 'ended':
        return 'Ended';
      default:
        return null;
    }
  }

  Color _auctionStatusColor() {
    switch (result.metadata['status']) {
      case 'active':
        return AppColors.primaryGreen;
      case 'scheduled':
        return const Color(0xFFFF8C00);
      case 'ended':
        return AppColors.neutralGray500;
      default:
        return AppColors.neutralGray500;
    }
  }

  int _auctionBidCount() {
    final raw = result.metadata['bidCount'];
    if (raw is int) return raw;
    if (raw is num) return raw.toInt();
    return 0;
  }

  /// Returns a countdown label only when:
  /// - status == active
  /// - endAt parses as a valid future DateTime
  /// - remaining time ≤ 24h
  /// Otherwise null. Never throws on missing/malformed metadata.
  String? _endingSoonLabel() {
    if (result.metadata['status'] != 'active') return null;
    final raw = result.metadata['endAt'];
    if (raw is! String) return null;
    final endAt = DateTime.tryParse(raw);
    if (endAt == null) return null;
    final remaining = endAt.difference(DateTime.now());
    if (remaining.isNegative) return null;
    if (remaining.inHours > 24) return null;
    if (remaining.inHours >= 1) return 'Ends in ${remaining.inHours}h';
    if (remaining.inMinutes >= 1) return 'Ends in ${remaining.inMinutes}m';
    return 'Ends in <1m';
  }

  String _priceLabel(num price) {
    final formatted = 'Rp ${price.toInt()}';
    if (!_isAuction) return formatted;
    final prefix = result.metadata['currentBid'] != null ? 'Bid' : 'Start';
    return '$prefix $formatted';
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final price = result.metadata['price'] as num?;
    final likesCount = result.metadata['likesCount'] as int?;
    final commentsCount = result.metadata['commentsCount'] as int?;
    final auctionStatusLabel = _isAuction ? _auctionStatusLabel() : null;
    final endingSoonLabel = _isAuction ? _endingSoonLabel() : null;
    final bidCount = _isAuction ? _auctionBidCount() : 0;

    // Wrap (not Row) so multi-chip auction rows reflow on narrow widths
    // instead of horizontally overflowing. Single-chip listing rows and
    // 0-2-chip user/content rows render identically to the prior Row
    // because they fit on one line.
    return Wrap(
      spacing: 8,
      runSpacing: 4,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        if (price != null)
          _buildInfoChip(
            context,
            _priceLabel(price),
            AppColors.primaryGreen,
            isDark,
          ),
        if (auctionStatusLabel != null)
          _buildInfoChip(
            context,
            auctionStatusLabel,
            _auctionStatusColor(),
            isDark,
          ),
        if (endingSoonLabel != null)
          _buildInfoChip(
            context,
            endingSoonLabel,
            const Color(0xFFFF8C00),
            isDark,
          ),
        if (_isAuction && bidCount > 0)
          _buildSmallInfo(context, Icons.gavel, bidCount.toString(), isDark),
        if (likesCount != null)
          _buildSmallInfo(
            context,
            Icons.favorite,
            likesCount.toString(),
            isDark,
          ),
        if (commentsCount != null)
          _buildSmallInfo(
            context,
            Icons.comment,
            commentsCount.toString(),
            isDark,
          ),
      ],
    );
  }

  Widget _buildInfoChip(
    BuildContext context,
    String text,
    Color color,
    bool isDark,
  ) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text(
        text,
        style: TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w600,
          color: color,
        ),
      ),
    );
  }

  Widget _buildSmallInfo(
    BuildContext context,
    IconData icon,
    String text,
    bool isDark,
  ) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(
          icon,
          size: 12,
          color: isDark ? AppColors.neutralGray500 : AppColors.neutralGray400,
        ),
        const SizedBox(width: 2),
        Text(
          text,
          style: TextStyle(
            fontSize: 11,
            color: isDark ? AppColors.neutralGray500 : AppColors.neutralGray400,
          ),
        ),
      ],
    );
  }
}
