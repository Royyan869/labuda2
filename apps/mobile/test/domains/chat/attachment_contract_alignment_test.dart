import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/chat/chat/data/mappers/chat_mapper.dart';
import 'package:labuda/domains/chat/chat/data/dto/attachment_dto.dart';
import 'package:labuda/domains/chat/chat/data/dto/message_dto.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/screens/chat_detail_screen.dart';
import 'package:labuda/shared/attachment/entities/attachment.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';

void main() {
  group('Attachment contract alignment', () {
    test('reference DTO writes canonical snake_case keys', () {
      final dto = ShareReferenceAttachmentDto(
        targetType: ShareTargetType.forSale,
        targetId: 'listing-123',
        preview: const SharePreviewDto(title: 'Produk'),
      );

      final data = dto.toJson()['data'] as Map<String, dynamic>;
      expect(data['target_type'], 'for_sale');
      expect(data['target_id'], 'listing-123');
      expect(data.containsKey('targetType'), isFalse);
      expect(data.containsKey('targetId'), isFalse);
    });

    test('reference DTO reads canonical snake_case keys', () {
      final dto = ShareReferenceAttachmentDto.fromJson({
        'type': 'reference',
        'data': {
          'target_type': 'auction',
          'target_id': 'auction-456',
          'preview': {'title': 'Lelang'},
        },
      });

      expect(dto.targetType, ShareTargetType.auction);
      expect(dto.targetId, 'auction-456');
    });

    test('reference DTO preserves profile wire target type', () {
      final dto = ShareReferenceAttachmentDto.fromJson({
        'type': 'reference',
        'data': {
          'target_type': 'profile',
          'target_id': 'user-123',
          'preview': {'title': 'Owner Profile'},
        },
      });

      expect(dto.targetType, ShareTargetType.profile);
      expect(dto.wireTargetType, 'profile');
      expect(dto.toJson()['data']['target_type'], 'profile');
    });

    test('canonical content share references stay canonical', () {
      final reference = ShareReference.content(
        contentId: 'content-1',
        title: 'Content title',
      );

      expect(reference.chatWireTargetType, 'content');
      expect(reference.asChatReference(), isNotNull);
      expect(reference.asChatReference()!.wireTargetType, 'content');
    });

    test('reference DTO rejects legacy post/request wire types', () {
      for (final wireType in <String>['post', 'request']) {
        expect(
          () => ShareReferenceAttachmentDto.fromJson({
            'type': 'reference',
            'data': {
              'target_type': wireType,
              'target_id': 'content-123',
              'preview': {'title': 'Content'},
            },
          }),
          throwsFormatException,
        );
      }
    });

    test('reference DTO rejects legacy camelCase keys', () {
      expect(
        () => ShareReferenceAttachmentDto.fromJson({
          'type': 'reference',
          'data': {
            'targetType': 'auction',
            'targetId': 'auction-456',
            'preview': {'title': 'Lelang'},
          },
        }),
        throwsFormatException,
      );
    });

    test(
      'parseAttachmentDto rejects legacy listing/auction/post/request types',
      () {
        final legacyTypes = <String>['listing', 'auction', 'post', 'request'];
        for (final t in legacyTypes) {
          expect(
            () => parseAttachmentDto({
              'type': t,
              'data': {'id': 'legacy-1'},
            }),
            throwsFormatException,
          );
        }
      },
    );

    test('negotiation_proposal parses canonical nested data', () {
      final dto =
          parseAttachmentDto({
                'type': 'negotiation_proposal',
                'data': {
                  'session_id': 's1',
                  'proposal_sequence': 2,
                  'price': 150000,
                },
              })
              as NegotiationProposalAttachmentDto;

      expect(dto.sessionId, 's1');
      expect(dto.proposalSequence, 2);
      expect(dto.price, 150000);
    });

    test('negotiation_proposal rejects flat payload shape', () {
      expect(
        () => parseAttachmentDto({
          'type': 'negotiation_proposal',
          'session_id': 's1',
          'proposal_sequence': 1,
          'price': 100000,
        }),
        throwsFormatException,
      );
    });

    test('negotiation_offer DTO uses fixed_price_sale_id', () {
      final dto = NegotiationOfferAttachmentDto(
        negotiationId: 'nego-1',
        forSaleId: 'fps-1',
        status: 'active',
        preview: const SharePreviewDto(title: 'Offer title'),
      );

      final data = dto.toJson()['data'] as Map<String, dynamic>;
      expect(data['negotiation_id'], 'nego-1');
      expect(data['fixed_price_sale_id'], 'fps-1');
      expect(data.containsKey('listing_id'), isFalse);
    });

    test('negotiation_result DTO uses fixed_price_sale_id', () {
      final dto = NegotiationResultAttachmentDto(
        negotiationId: 'nego-2',
        forSaleId: 'fps-2',
        status: 'accepted',
        preview: const SharePreviewDto(title: 'Result title'),
      );

      final data = dto.toJson()['data'] as Map<String, dynamic>;
      expect(data['negotiation_id'], 'nego-2');
      expect(data['fixed_price_sale_id'], 'fps-2');
      expect(data.containsKey('listing_id'), isFalse);
    });

    test('negotiation_offer parses fixed_price_sale_id', () {
      final dto =
          parseAttachmentDto({
                'type': 'negotiation_offer',
                'data': {
                  'negotiation_id': 'nego-3',
                  'fixed_price_sale_id': 'fps-3',
                  'status': 'active',
                  'preview': {'title': 'Offer title'},
                },
              })
              as NegotiationOfferAttachmentDto;

      expect(dto.negotiationId, 'nego-3');
      expect(dto.forSaleId, 'fps-3');
    });

    test('negotiation_result parses fixed_price_sale_id', () {
      final dto =
          parseAttachmentDto({
                'type': 'negotiation_result',
                'data': {
                  'negotiation_id': 'nego-4',
                  'fixed_price_sale_id': 'fps-4',
                  'status': 'accepted',
                  'preview': {'title': 'Result title'},
                },
              })
              as NegotiationResultAttachmentDto;

      expect(dto.negotiationId, 'nego-4');
      expect(dto.forSaleId, 'fps-4');
    });

    test('shipping_quote DTO omits legacy seller_name transport', () {
      final dto = ShippingQuoteAttachmentDto(
        offerId: 'offer-1',
        linkedItemId: 'item-1',
        linkedItemType: 'listing',
        shippingType: 'manual',
        shippingTypeName: 'Ongkir Manual',
        shippingTypeEmoji: '🚚',
        rate: 25000,
        status: 'ACTIVE',
        sellerId: 'seller-1',
      );

      final data = dto.toJson()['data'] as Map<String, dynamic>;
      expect(data.containsKey('seller_name'), isFalse);
      expect(data['seller_id'], 'seller-1');
    });

    test(
      'shipping_quote DTO preserves linked item and auction identifiers',
      () {
        final listingDto = ShippingQuoteAttachmentDto(
          offerId: 'offer-1',
          linkedItemId: 'listing-1',
          linkedItemType: 'listing',
          linkedItemName: 'Listing title',
          linkedItemPrice: 1000,
          shippingType: 'manual',
          shippingTypeName: 'Ongkir Manual',
          shippingTypeEmoji: '🚚',
          rate: 25000,
          status: 'ACTIVE',
          sellerId: 'seller-1',
        );
        final listingData = listingDto.toJson()['data'] as Map<String, dynamic>;
        expect(listingData['linked_item_id'], 'listing-1');
        expect(listingData['linked_item_type'], 'listing');
        expect(listingData.containsKey('listing_id'), isFalse);
        expect(listingData.containsKey('auction_id'), isFalse);

        final auctionDto = ShippingQuoteAttachmentDto(
          offerId: 'offer-2',
          linkedItemId: 'auction-1',
          linkedItemType: 'auction',
          auctionId: 'auction-1',
          linkedItemName: 'Auction title',
          linkedItemPrice: 1200,
          shippingType: 'manual',
          shippingTypeName: 'Ongkir Manual',
          shippingTypeEmoji: '🚚',
          rate: 25000,
          status: 'ACTIVE',
          sellerId: 'seller-1',
        );
        final auctionData = auctionDto.toJson()['data'] as Map<String, dynamic>;
        expect(auctionData['auction_id'], 'auction-1');
        expect(auctionData['linked_item_id'], 'auction-1');
        expect(auctionData['linked_item_type'], 'auction');
        expect(auctionData.containsKey('listing_id'), isFalse);
      },
    );

    test('shipping_quote DTO parses without legacy seller_name', () {
      final dto =
          parseAttachmentDto({
                'type': 'shipping_quote',
                'data': {
                  'offer_id': 'offer-1',
                  'linked_item_id': 'item-1',
                  'linked_item_type': 'listing',
                  'shipping_type': 'manual',
                  'shipping_type_name': 'Ongkir Manual',
                  'shipping_type_emoji': '🚚',
                  'rate': 25000,
                  'status': 'ACTIVE',
                  'seller_id': 'seller-1',
                },
              })
              as ShippingQuoteAttachmentDto;

      expect(dto.sellerId, 'seller-1');
      expect(dto.toJson()['data'].containsKey('seller_name'), isFalse);
    });

    test('shipping_quote mapper tolerates missing optional preview fields', () {
      final dto = MessageDto.fromJson(
        _messageJson(
          attachmentJson: {
            'type': 'shipping_quote',
            'data': {
              'offer_id': 'offer-1',
              'linked_item_id': 'listing-1',
              'linked_item_type': 'listing',
              'shipping_type': 'manual',
              'shipping_type_name': 'Ongkir Manual',
              'shipping_type_emoji': '🚚',
              'rate': 25000,
              'status': 'ACTIVE',
              'seller_id': 'seller-1',
            },
          },
        ),
      );

      final message = ChatMapper.messageToDomain(dto);
      expect(message.shippingQuote, isNotNull);
      expect(message.shippingQuote!.linkedItemName, isNotEmpty);
      expect(message.shippingQuote!.linkedItemPrice, 0);
    });

    test(
      'shipping quote checkout target resolves listing and auction routes',
      () async {
        final listingQuote = ShippingQuoteAttachment(
          offerId: 'offer-1',
          linkedItemId: 'listing-1',
          linkedItemType: 'listing',
          linkedItemName: 'Listing title',
          linkedItemPrice: 1000,
          shippingType: 'manual',
          shippingTypeName: 'Ongkir Manual',
          shippingTypeEmoji: '🚚',
          rate: 25000,
          validUntil: DateTime.utc(2026, 6, 1),
          status: 'ACTIVE',
          sellerId: 'seller-1',
        );

        final listingTarget = await resolveShippingQuoteCheckoutTarget(
          shippingQuote: listingQuote,
          resolveAuctionProductId: (_) async => null,
        );
        expect(listingTarget?.forSaleId, 'listing-1');
        expect(listingTarget?.auctionId, isNull);
        expect(listingTarget?.productId, isNull);

        final auctionQuote = ShippingQuoteAttachment(
          offerId: 'offer-2',
          linkedItemId: 'auction-1',
          linkedItemType: 'auction',
          linkedItemName: 'Auction title',
          linkedItemPrice: 1200,
          shippingType: 'manual',
          shippingTypeName: 'Ongkir Manual',
          shippingTypeEmoji: '🚚',
          rate: 25000,
          validUntil: DateTime.utc(2026, 6, 1),
          status: 'ACTIVE',
          sellerId: 'seller-1',
        );

        final auctionTarget = await resolveShippingQuoteCheckoutTarget(
          shippingQuote: auctionQuote,
          resolveAuctionProductId: (auctionId) async {
            expect(auctionId, 'auction-1');
            return 'product-9'; // product ID distinct from auctionId
          },
        );

        // Auction path: productId in productId slot, auctionId in auctionId slot,
        // fixedPriceSaleId must be null.
        expect(auctionTarget?.productId, 'product-9');
        expect(auctionTarget?.auctionId, 'auction-1');
        expect(auctionTarget?.forSaleId, isNull);
      },
    );

    test('message DTO reads lifecycle from attachment_metadata as primary', () {
      final dto = MessageDto.fromJson(
        _messageJson(
          attachmentMetadata: {'seller_trust_lifecycle': 'suspended'},
          attachmentJson: {
            'type': 'reference',
            'data': {
              'target_type': 'for_sale',
              'target_id': 'listing-1',
              'preview': {'title': 'Produk'},
            },
            'seller_trust_lifecycle': 'active',
          },
        ),
      );

      expect(dto.attachmentSellerTrustLifecycle, 'suspended');
    });

    test('message DTO fallback old lifecycle key still works', () {
      final dto = MessageDto.fromJson(
        _messageJson(
          attachmentJson: {
            'type': 'reference',
            'data': {
              'target_type': 'for_sale',
              'target_id': 'listing-1',
              'preview': {'title': 'Produk'},
            },
            'seller_trust_lifecycle': 'active',
          },
        ),
      );

      expect(dto.attachmentSellerTrustLifecycle, 'active');
    });

    test(
      'chat mapper serializes canonical chat targets and normalized content shares',
      () {
        final canonicalCases = <Map<String, Object>>[
          {
            'reference': ShareReference.forSale(
              forSaleId: 'listing-1',
              title: 'Listing title',
            ),
            'wireType': 'for_sale',
          },
          {
            'reference': ShareReference.auction(
              auctionId: 'auction-1',
              title: 'Auction title',
            ),
            'wireType': 'auction',
          },
          {
            'reference': ShareReference.profile(
              profileId: 'profile-1',
              name: 'Profile title',
            ),
            'wireType': 'profile',
          },
        ];

        for (final testCase in canonicalCases) {
          final reference = testCase['reference'] as ShareReference;
          final expectedWireType = testCase['wireType'] as String;
          final message = Message(
            id: 'm-$expectedWireType',
            chatId: 'c1',
            senderId: 'u1',
            senderName: 'Sender',
            content: 'hello',
            objectReference: reference,
            createdAt: DateTime.parse('2026-06-01T00:00:00.000Z'),
          );

          final attachment =
              ChatMapper.domainAttachmentToDto(message)
                  as ShareReferenceAttachmentDto;
          expect(attachment.toJson()['data']['target_type'], expectedWireType);
        }

        final genericMessage = Message(
          id: 'm-content',
          chatId: 'c1',
          senderId: 'u1',
          senderName: 'Sender',
          content: 'hello',
          objectReference: ShareReference.content(
            contentId: 'content-2',
            title: 'Generic content',
          ),
          createdAt: DateTime.parse('2026-06-01T00:00:00.000Z'),
        );

        final attachment =
            ChatMapper.domainAttachmentToDto(genericMessage)
                as ShareReferenceAttachmentDto;
        expect(attachment.toJson()['data']['target_type'], 'content');
      },
    );
  });
}

Map<String, dynamic> _messageJson({
  Map<String, dynamic>? attachmentMetadata,
  Map<String, dynamic>? attachmentJson,
}) {
  final json = <String, dynamic>{
    'id': 'm1',
    'chat_room_id': 'c1',
    'sender_id': 'u1',
    'sender_name': 'Sender',
    'content': 'hello',
    'type': 'text',
    'status': 'sent',
    'is_read': false,
    'is_edited': false,
    'created_at': '2026-06-01T00:00:00.000Z',
    'updated_at': '2026-06-01T00:00:00.000Z',
  };

  if (attachmentMetadata != null) {
    json['attachment_metadata'] = attachmentMetadata;
  }
  if (attachmentJson != null) {
    json['attachment_json'] = attachmentJson;
  }

  return json;
}
