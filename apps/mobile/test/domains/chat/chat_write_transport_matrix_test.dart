import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/chat/chat/data/dto/attachment_dto.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_resource_occurrence_request.dart';
import 'package:labuda/domains/chat/chat/data/dto/message_dto.dart';

const _resourceIdA = '33333333-3333-3333-3333-333333333333';
const _resourceIdB = '44444444-4444-4444-4444-444444444444';
const _resourceIdC = '55555555-5555-5555-5555-555555555555';
const _idempotencyKey = '66666666-6666-6666-6666-666666666666';
const _senderId = '22222222-2222-2222-2222-222222222222';

void main() {
  group('Chat write transport — SendMessageDto serialization', () {
    test('W3 location/negotiation/shipping attachments serialize as attachment_json', () {
      final cases = <AttachmentDto>[
        LocationAttachmentDto(
          latitude: -6.2,
          longitude: 106.8,
          placeName: 'Jakarta',
          address: 'Jakarta',
        ),
        NegotiationProposalAttachmentDto(
          sessionId: 'session-1',
          proposalSequence: 1,
          price: 150000,
          resourceType: 'for_sale',
          resourceId: _resourceIdB,
          note: 'counter',
        ),
        ShippingQuoteAttachmentDto(
          offerId: 'offer-1',
          linkedItemId: _resourceIdB,
          linkedItemType: 'listing',
          shippingType: 'manual',
          shippingTypeName: 'Manual',
          shippingTypeEmoji: '🚚',
          rate: 25000.0,
          status: 'ACTIVE',
          sellerId: _senderId,
        ),
      ];

      for (final attachment in cases) {
        final payload = SendMessageDto(
          body: 'hello',
          messageType: 'text',
          idempotencyKey: _idempotencyKey,
          attachment: attachment,
        ).toJson();
        expect(payload.containsKey('attachment_json'), isTrue);
      }
    });

    test('W24 unrelated legacy/non-resource attachments remain unchanged', () {
      final attachment = ShippingQuoteAttachmentDto(
        offerId: 'offer-9',
        linkedItemId: _resourceIdB,
        linkedItemType: 'listing',
        shippingType: 'manual',
        shippingTypeName: 'Manual',
        shippingTypeEmoji: '🚚',
        rate: 25000.0,
        status: 'ACTIVE',
        sellerId: _senderId,
      );
      final payload = SendMessageDto(
        body: 'hello',
        messageType: 'text',
        idempotencyKey: _idempotencyKey,
        attachment: attachment,
      ).toJson();

      expect(payload['attachment_json'], isNotNull);
    });
  });

  group('ChatResourceOccurrenceRequest — share_to_chat serialization', () {
    test('W4 profile share request serializes share_to_chat', () {
      final request = ChatResourceOccurrenceRequest.shareToChat(
        resourceType: ChatResourceOccurrenceResourceType.profile,
        resourceId: _resourceIdA,
      );
      expect(request.toJson(), {
        'operation': 'share_to_chat',
        'resource_type': 'profile',
        'resource_id': _resourceIdA,
      });
    });

    test('W5 content share request serializes share_to_chat', () {
      final request = ChatResourceOccurrenceRequest.shareToChat(
        resourceType: ChatResourceOccurrenceResourceType.content,
        resourceId: _resourceIdB,
      );
      expect(request.toJson(), {
        'operation': 'share_to_chat',
        'resource_type': 'content',
        'resource_id': _resourceIdB,
      });
    });

    test('W6 FPS share request serializes share_to_chat', () {
      final request = ChatResourceOccurrenceRequest.shareToChat(
        resourceType: ChatResourceOccurrenceResourceType.forSale,
        resourceId: _resourceIdC,
      );
      expect(request.toJson(), {
        'operation': 'share_to_chat',
        'resource_type': 'for_sale',
        'resource_id': _resourceIdC,
      });
    });

    test('W7 Auction share request serializes share_to_chat', () {
      final request = ChatResourceOccurrenceRequest.shareToChat(
        resourceType: ChatResourceOccurrenceResourceType.auction,
        resourceId: _resourceIdA,
      );
      expect(request.toJson(), {
        'operation': 'share_to_chat',
        'resource_type': 'auction',
        'resource_id': _resourceIdA,
      });
    });

    test('W14 request contains identity only', () {
      final request = ChatResourceOccurrenceRequest.shareToChat(
        resourceType: ChatResourceOccurrenceResourceType.profile,
        resourceId: _resourceIdA,
      );
      expect(request.toJson().keys.toSet(), {
        'operation',
        'resource_type',
        'resource_id',
      });
    });

    test('W18 same FPS can produce distinct share/direct operations', () {
      final share = ChatResourceOccurrenceRequest.shareToChat(
        resourceType: ChatResourceOccurrenceResourceType.forSale,
        resourceId: _resourceIdB,
      );
      final direct = ChatResourceOccurrenceRequest.directCommerceInsertChat(
        resourceType: ChatResourceOccurrenceResourceType.forSale,
        resourceId: _resourceIdB,
      );
      expect(share, isNot(equals(direct)));
      expect(share.toJson()['operation'], 'share_to_chat');
      expect(direct.toJson()['operation'], 'direct_commerce_insert_chat');
    });

    test('W19 same Auction can produce distinct share/direct operations', () {
      final share = ChatResourceOccurrenceRequest.shareToChat(
        resourceType: ChatResourceOccurrenceResourceType.auction,
        resourceId: _resourceIdC,
      );
      final direct = ChatResourceOccurrenceRequest.directCommerceInsertChat(
        resourceType: ChatResourceOccurrenceResourceType.auction,
        resourceId: _resourceIdC,
      );
      expect(share, isNot(equals(direct)));
      expect(share.toJson()['operation'], 'share_to_chat');
      expect(direct.toJson()['operation'], 'direct_commerce_insert_chat');
    });
  });

  group('ChatResourceOccurrenceRequest — direct_commerce_insert_chat serialization', () {
    test('W8 FPS direct insert serializes direct_commerce_insert_chat', () {
      final request = ChatResourceOccurrenceRequest.directCommerceInsertChat(
        resourceType: ChatResourceOccurrenceResourceType.forSale,
        resourceId: _resourceIdB,
      );
      expect(request.toJson(), {
        'operation': 'direct_commerce_insert_chat',
        'resource_type': 'for_sale',
        'resource_id': _resourceIdB,
      });
    });

    test('W9 Auction direct insert serializes direct_commerce_insert_chat', () {
      final request = ChatResourceOccurrenceRequest.directCommerceInsertChat(
        resourceType: ChatResourceOccurrenceResourceType.auction,
        resourceId: _resourceIdC,
      );
      expect(request.toJson(), {
        'operation': 'direct_commerce_insert_chat',
        'resource_type': 'auction',
        'resource_id': _resourceIdC,
      });
    });

    test('W15 no preview/title/image/price/seller fields', () {
      final request = ChatResourceOccurrenceRequest.directCommerceInsertChat(
        resourceType: ChatResourceOccurrenceResourceType.forSale,
        resourceId: _resourceIdB,
      );
      final json = request.toJson();
      for (final forbidden in <String>[
        'preview',
        'title',
        'image',
        'price',
        'seller',
        'username',
      ]) {
        expect(json.containsKey(forbidden), isFalse);
      }
    });
  });

  group('ChatResourceOccurrenceRequest — structural rejection', () {
    test('W10 direct Profile is rejected structurally', () {
      expect(
        () => ChatResourceOccurrenceRequest.directCommerceInsertChat(
          resourceType: ChatResourceOccurrenceResourceType.profile,
          resourceId: _resourceIdA,
        ),
        throwsFormatException,
      );
    });

    test('W11 direct Content is rejected structurally', () {
      expect(
        () => ChatResourceOccurrenceRequest.directCommerceInsertChat(
          resourceType: ChatResourceOccurrenceResourceType.content,
          resourceId: _resourceIdB,
        ),
        throwsFormatException,
      );
    });

    test('rejects unknown operation at parse boundary', () {
      expect(
        () => ChatResourceOccurrenceOperation.fromWire('unknown'),
        throwsFormatException,
      );
    });

    test('rejects unknown resource type at parse boundary', () {
      expect(
        () => ChatResourceOccurrenceResourceType.fromWire('unknown'),
        throwsFormatException,
      );
    });

    test('rejects nil resource ID structurally', () {
      expect(
        () => ChatResourceOccurrenceRequest.shareToChat(
          resourceType: ChatResourceOccurrenceResourceType.profile,
          resourceId: '00000000-0000-0000-0000-000000000000',
        ),
        throwsFormatException,
      );
    });

    test('rejects empty resource ID structurally', () {
      expect(
        () => ChatResourceOccurrenceRequest.shareToChat(
          resourceType: ChatResourceOccurrenceResourceType.profile,
          resourceId: '   ',
        ),
        throwsFormatException,
      );
    });
  });
}
