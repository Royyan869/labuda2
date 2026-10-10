import 'package:equatable/equatable.dart';
import 'package:hishumi/domains/chat/chat/data/dto/chat_resource_occurrence_request.dart';

/// Canonical pending product attachment held by the Chat composer.
///
/// ONE authority for every entry point that parks a not-yet-sent For Sale or
/// Auction product in Chat: the composer picker (`Lampirkan Produk`), the
/// For Sale detail Chat CTA, the Auction detail Chat CTA and the checkout
/// "Hubungi Penjual" CTA. Every producer seeds this exact model; there is no
/// second pending representation and no per-entry send authority.
///
/// Display data here is a SNAPSHOT from the picker (identity + display
/// hints), never commerce authority: price and availability are re-resolved
/// by the server at send into the viewer-aware projection.
///
/// Sending is the composer's send icon only — this model carries no send
/// action. The composer turns it into a canonical resource occurrence
/// (`direct_commerce_insert_chat`).
class PendingCommerceAttachment extends Equatable {
  final ChatResourceOccurrenceResourceType resourceType;
  final String resourceId;
  final String title;
  final String? imageUrl;
  final int? price;

  const PendingCommerceAttachment({
    required this.resourceType,
    required this.resourceId,
    required this.title,
    this.imageUrl,
    this.price,
  });

  /// Canonical construction for a pending For Sale attachment.
  factory PendingCommerceAttachment.forSale({
    required String forSaleId,
    required String title,
    String? imageUrl,
    int? price,
  }) => PendingCommerceAttachment(
    resourceType: ChatResourceOccurrenceResourceType.forSale,
    resourceId: forSaleId,
    title: title,
    imageUrl: imageUrl,
    price: price,
  );

  /// Canonical construction for a pending Auction attachment.
  factory PendingCommerceAttachment.auction({
    required String auctionId,
    required String title,
    String? imageUrl,
    int? price,
  }) => PendingCommerceAttachment(
    resourceType: ChatResourceOccurrenceResourceType.auction,
    resourceId: auctionId,
    title: title,
    imageUrl: imageUrl,
    price: price,
  );

  /// Canonical send contract derived from the pending identity.
  ///
  /// Identity + operation only — it never carries Commerce business state
  /// (price/availability/order/payment).
  ChatResourceOccurrenceRequest toSendRequest() =>
      ChatResourceOccurrenceRequest(
        operation: ChatResourceOccurrenceOperation.directCommerceInsertChat,
        resourceType: resourceType,
        resourceId: resourceId,
      );

  @override
  List<Object?> get props => [resourceType, resourceId, title, imageUrl, price];
}
