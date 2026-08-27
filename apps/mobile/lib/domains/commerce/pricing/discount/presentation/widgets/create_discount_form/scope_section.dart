import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';

class ScopeSection extends StatefulWidget {
  final DiscountAppliesTo appliesTo;
  final DiscountTargetMode targetMode;
  final List<String> applicableListingIds;
  final List<String> applicableAuctionIds;
  final ValueChanged<DiscountAppliesTo> onAppliesToChanged;
  final ValueChanged<DiscountTargetMode> onTargetModeChanged;
  final ValueChanged<List<String>> onListingIdsChanged;
  final ValueChanged<List<String>> onAuctionIdsChanged;

  const ScopeSection({
    super.key,
    required this.appliesTo,
    required this.targetMode,
    required this.applicableListingIds,
    required this.applicableAuctionIds,
    required this.onAppliesToChanged,
    required this.onTargetModeChanged,
    required this.onListingIdsChanged,
    required this.onAuctionIdsChanged,
  });

  @override
  State<ScopeSection> createState() => _ScopeSectionState();
}

class _ScopeSectionState extends State<ScopeSection> {
  late final TextEditingController _listingController;
  late final TextEditingController _auctionController;

  @override
  void initState() {
    super.initState();
    _listingController = TextEditingController(
      text: widget.applicableListingIds.join(', '),
    );
    _auctionController = TextEditingController(
      text: widget.applicableAuctionIds.join(', '),
    );
  }

  @override
  void didUpdateWidget(covariant ScopeSection oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.applicableListingIds != widget.applicableListingIds) {
      _listingController.text = widget.applicableListingIds.join(', ');
    }
    if (oldWidget.applicableAuctionIds != widget.applicableAuctionIds) {
      _auctionController.text = widget.applicableAuctionIds.join(', ');
    }
  }

  @override
  void dispose() {
    _listingController.dispose();
    _auctionController.dispose();
    super.dispose();
  }

  List<String> _parseIds(String value) {
    return value
        .split(',')
        .map((item) => item.trim())
        .where((item) => item.isNotEmpty)
        .toList();
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark
            ? core.AppColors.darkGray800
            : core.AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Discount Applicability',
            style: TextStyle(
              fontSize: 16,
              fontWeight: FontWeight.bold,
              color: isDark
                  ? core.AppColors.neutralWhite
                  : core.AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 16),
          DropdownButtonFormField<DiscountAppliesTo>(
            initialValue: widget.appliesTo,
            decoration: const InputDecoration(
              labelText: 'Applies To',
              border: OutlineInputBorder(),
            ),
            items: const [
              DropdownMenuItem(
                value: DiscountAppliesTo.listing,
                child: Text('Listing'),
              ),
              DropdownMenuItem(
                value: DiscountAppliesTo.auction,
                child: Text('Auction'),
              ),
              DropdownMenuItem(
                value: DiscountAppliesTo.both,
                child: Text('Both'),
              ),
            ],
            onChanged: (value) {
              if (value != null) {
                widget.onAppliesToChanged(value);
              }
            },
          ),
          const SizedBox(height: 12),
          DropdownButtonFormField<DiscountTargetMode>(
            initialValue: widget.targetMode,
            decoration: const InputDecoration(
              labelText: 'Target Mode',
              border: OutlineInputBorder(),
            ),
            items: const [
              DropdownMenuItem(
                value: DiscountTargetMode.sellerWide,
                child: Text('Seller-wide'),
              ),
              DropdownMenuItem(
                value: DiscountTargetMode.selectedItems,
                child: Text('Selected items'),
              ),
            ],
            onChanged: (value) {
              if (value != null) {
                widget.onTargetModeChanged(value);
              }
            },
          ),
          if (widget.targetMode == DiscountTargetMode.selectedItems) ...[
            const SizedBox(height: 16),
            if (widget.appliesTo != DiscountAppliesTo.auction) ...[
              TextField(
                controller: _listingController,
                decoration: const InputDecoration(
                  labelText: 'Listing IDs',
                  hintText: 'Comma-separated listing UUIDs',
                  border: OutlineInputBorder(),
                ),
                onChanged: (value) =>
                    widget.onListingIdsChanged(_parseIds(value)),
              ),
              const SizedBox(height: 12),
            ],
            if (widget.appliesTo != DiscountAppliesTo.listing) ...[
              TextField(
                controller: _auctionController,
                decoration: const InputDecoration(
                  labelText: 'Auction IDs',
                  hintText: 'Comma-separated auction UUIDs',
                  border: OutlineInputBorder(),
                ),
                onChanged: (value) =>
                    widget.onAuctionIdsChanged(_parseIds(value)),
              ),
            ],
            const SizedBox(height: 8),
            Text(
              'Use selected-items only when you need to target specific listings or auctions.',
              style: TextStyle(
                fontSize: 12,
                color: isDark
                    ? core.AppColors.neutralGray400
                    : core.AppColors.neutralGray600,
              ),
            ),
          ],
        ],
      ),
    );
  }
}
