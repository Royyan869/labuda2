import '../../domain/entities/negotiation.dart';
import '../dto/negotiation_dto.dart';

/// Mapper untuk konversi API DTO → Domain Entity
///
/// **Data Layer** - mapping logic isolated here
///
/// **OWNERSHIP TRUTH:**
/// - Backend API is the single source of truth for negotiation data
/// - This mapper only converts backend response to local entity format
///
/// **FIELDS NOT IN BACKEND RESPONSE:**
/// - chatId: populated from chatRoomId context at call site
/// - listingName, listingImage: populated from UI context
/// - buyerName, buyerAvatar, sellerAvatar: populated from chat context
/// - originalPrice: not tracked by backend, populated from fixed-price sale context
/// - offers: not provided by backend, reconstructed from chat messages
class NegotiationMapper {
  /// Convert API Response DTO to Domain Entity
  static Negotiation toEntity(
    NegotiationResponseDto dto, {
    String chatRoomId = '',
  }) {
    // Determine last offer by: buyer starts (seq 1), then alternates
    // seq 0 = no proposals yet, seq 1 = buyer, seq 2 = seller, etc.
    final lastOfferBy = dto.proposalSequence > 0 && dto.proposalSequence.isEven
        ? 'seller'
        : 'buyer';

    return Negotiation(
      id: dto.id,
      chatId: chatRoomId.isNotEmpty ? chatRoomId : (dto.chatRoomId ?? ''),
      fixedPriceSaleId: dto.forSaleId ?? dto.resourceId,
      listingName: '', // POPULATED SEPARATELY from UI context
      originalPrice: dto.currentPrice?.toDouble() ?? 0,
      buyerId: dto.buyerId,
      buyerName: '', // POPULATED SEPARATELY from chat context
      sellerId: dto.sellerId,
      status: NegotiationStatusExtension.fromString(dto.status),
      currentOfferPrice: dto.currentPrice?.toDouble() ?? 0,
      lastOfferBy: lastOfferBy,
      round: dto.proposalSequence,
      offers: [],
      agreedPrice: dto.acceptedPrice?.toDouble(),
      createdAt: dto.createdAt,
      updatedAt: dto.updatedAt,
      completedAt: dto.acceptedAt,
    );
  }
}
