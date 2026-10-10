/// Auction Detail Header
///
/// The canonical DETAIL MEDIA BLOCK — identical to the ForSale gallery:
/// the shared `MediaCarouselWidget` at 4:5 contain (same as the card — koi
/// never cropped), edge to edge, tap opens the fullscreen viewer.
///
/// The title is NOT part of the header; both channels render it as the first
/// item of the detail body (same style, same spacing).
library;

import 'package:flutter/material.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';
import 'package:hishumi/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:hishumi/shared/shared.dart';
import 'package:hishumi/shared/utils/media_extensions.dart';

class AuctionDetailHeader extends StatelessWidget {
  final Auction auction;

  const AuctionDetailHeader({super.key, required this.auction});

  void _openViewer(BuildContext context, int index) {
    if (auction.media.isEmpty) return;
    showDialog(
      context: context,
      barrierColor:
          Theme.of(context).colorScheme.scrim.withValues(alpha: 0.87),
      builder: (_) => MediaViewerWidget(
        media: auction.media,
        initialIndex: index.clamp(0, auction.media.length - 1),
        title: auction.title,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    if (auction.media.isNotEmptyUrls) {
      return MediaCarouselWidget(
        media: auction.media,
        aspectRatio: 4 / 5,
        fit: BoxFit.contain,
        borderRadius: BorderRadius.zero,
        onImageTapWithIndex: (index) => _openViewer(context, index),
      );
    }

    return AspectRatio(
      aspectRatio: 4 / 5,
      child: Container(
        color: colorScheme.surfaceContainerHighest,
        child: Center(
          child: Icon(
            Icons.image_outlined,
            size: AppIconSize.display,
            color: colorScheme.onSurfaceVariant,
          ),
        ),
      ),
    );
  }
}
