/// Auction Detail Header
///
/// Header widget showing auction media and basic info
library;

import 'package:flutter/material.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/shared/utils/media_extensions.dart';

/// Header widget for auction detail
class AuctionDetailHeader extends StatelessWidget {
  final Auction auction;

  const AuctionDetailHeader({super.key, required this.auction});

  @override
  Widget build(BuildContext context) {
    return Container(
      color: Colors.white,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Media gallery
          if (auction.media.isNotEmptyUrls)
            SizedBox(
              height: 300,
              child: PageView.builder(
                itemCount: auction.media.length,
                itemBuilder: (context, index) {
                  return Image.network(
                    auction.media.urls[index],
                    fit: BoxFit.contain,
                    errorBuilder: (context, error, stackTrace) {
                      return Container(
                        color: Colors.grey[200],
                        child: const Center(
                          child: Icon(Icons.image_not_supported, size: 64),
                        ),
                      );
                    },
                  );
                },
              ),
            )
          else
            Container(
              height: 300,
              color: Colors.grey[200],
              child: const Center(child: Icon(Icons.image, size: 64)),
            ),
          // Title
          Padding(
            padding: const EdgeInsets.all(16),
            child: Text(
              auction.title,
              style: const TextStyle(fontSize: 20, fontWeight: FontWeight.bold),
            ),
          ),
        ],
      ),
    );
  }
}
