import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_resource_occurrence_request.dart';

const _resourceIdA = '33333333-3333-3333-3333-333333333333';
const _resourceIdB = '44444444-4444-4444-4444-444444444444';
const _resourceIdC = '55555555-5555-5555-5555-555555555555';

void main() {
  // Product-chat write contract is EXCLUSIVELY the resource occurrence
  // (`direct_commerce_insert_chat`). The objectReference/attachment_json write
  // path is purged; there is no share_to_chat producer.
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
        () => ChatResourceOccurrenceRequest.directCommerceInsertChat(
          resourceType: ChatResourceOccurrenceResourceType.forSale,
          resourceId: '00000000-0000-0000-0000-000000000000',
        ),
        throwsFormatException,
      );
    });

    test('rejects empty resource ID structurally', () {
      expect(
        () => ChatResourceOccurrenceRequest.directCommerceInsertChat(
          resourceType: ChatResourceOccurrenceResourceType.forSale,
          resourceId: '   ',
        ),
        throwsFormatException,
      );
    });
  });
}
