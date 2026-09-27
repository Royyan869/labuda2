/// Auction Detail Header
///
/// The canonical DETAIL MEDIA BLOCK — identical to the ForSale gallery:
/// the shared `MediaCarouselWidget` at 4/3, edge to edge, no raw
/// `Image.network`, no local `PageView` controller.
///
/// The title is NOT part of the header; both channels render it as the first
/// item of the detail body (same style, same spacing).
library;

import 'package:flutter/material.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/utils/media_extensions.dart';

class AuctionDetailHeader extends StatelessWidget {
  final Auction auction;

  const AuctionDetailHeader({super.key, required this.auction});

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    if (auction.media.isNotEmptyUrls) {
      return MediaCarouselWidget(
        media: auction.media,
        aspectRatio: 4 / 3,
        borderRadius: BorderRadius.zero,
      );
    }

    return Container(
      height: 225,
      color: colorScheme.surfaceContainerHighest,
      child: Center(
        child: Icon(
          Icons.image_outlined,
          size: 64,
          color: colorScheme.onSurfaceVariant,
        ),
      ),
    );
  }
}
