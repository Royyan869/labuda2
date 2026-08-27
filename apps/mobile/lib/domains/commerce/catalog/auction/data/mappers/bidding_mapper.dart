/// Bidding Mapper
/// Converts between DTOs and domain entities
library;

import 'package:labuda/domains/commerce/catalog/auction/data/dto/bidding_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/bidding_item.dart';

/// Bidding Mapper
class BiddingMapper {
  BiddingMapper._();

  /// Convert BiddingItemDto to BiddingItem entity
  static BiddingItem toItemEntity(BiddingItemDto dto) {
    return BiddingItem(
      auctionId: dto.auctionId,
      title: dto.title,
      yourLastBid: dto.yourLastBid,
      currentBid: dto.currentBid,
      status: BiddingStatus.fromString(dto.status),
      endAt: dto.endAt,
      updatedAt: dto.updatedAt,
    );
  }

  /// Convert BiddingResultDto to BiddingResult entity
  static BiddingResult toResultEntity(BiddingResultDto dto) {
    return BiddingResult(
      items: dto.items.map(toItemEntity).toList(),
      activeCount: dto.activeCount,
      wonCount: dto.wonCount,
      lostCount: dto.lostCount,
    );
  }
}
