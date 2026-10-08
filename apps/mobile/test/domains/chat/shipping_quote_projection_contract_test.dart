// SHIPPING QUOTE → CONVERSATION PROJECTION CONTRACT (Layer 1B).
//
// The backend projects a viewer-scoped actionability envelope onto a
// shipping-quote message. The mobile chain must carry it
// JSON → DTO → mapper → domain VERBATIM and must NEVER recompute quote
// lifecycle (status/isActive) or buyer eligibility itself.

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/chat/chat/data/dto/message_dto.dart';
import 'package:labuda/domains/chat/chat/data/mappers/chat_mapper.dart';

Map<String, dynamic> _messageJson({
  required String status,
  bool? isCurrent,
  bool? viewerActionable,
}) {
  final json = <String, dynamic>{
    'id': 'm1',
    'chat_room_id': 'room-1',
    'sender_id': 'seller-1',
    'content': '',
    'message_type': 'shipping_quote',
    'status': 'sent',
    'created_at': '2026-06-01T00:00:00.000Z',
    'attachment_json': {
      'type': 'shipping_quote',
      'data': {
        'offer_id': 'offer-1',
        'linked_item_id': 'for-sale-1',
        'linked_item_type': 'forSale',
        'shipping_type': 'manual',
        'shipping_type_name': 'Ongkir Manual',
        'shipping_type_emoji': '🚚',
        'rate': 25000,
        'status': status,
        'seller_id': 'seller-1',
      },
    },
  };
  if (isCurrent != null && viewerActionable != null) {
    json['shipping_quote_projection'] = {
      'is_current': isCurrent,
      'viewer_actionable': viewerActionable,
    };
  }
  return json;
}

void main() {
  test('JSON → DTO preserves the viewer-scoped projection verbatim', () {
    final dto = MessageDto.fromJson(
      _messageJson(status: 'ACTIVE', isCurrent: true, viewerActionable: true),
    );
    final projection = dto.shippingQuoteProjection;
    expect(projection, isNotNull);
    expect(projection!.isCurrent, isTrue);
    expect(projection.viewerActionable, isTrue);
  });

  test('mapper carries the projection onto the shipping quote domain model', () {
    final dto = MessageDto.fromJson(
      _messageJson(status: 'ACTIVE', isCurrent: true, viewerActionable: true),
    );
    final message = ChatMapper.messageToDomain(dto);
    final attachment = message.shippingQuote;
    expect(attachment, isNotNull);
    expect(attachment!.isCurrent, isTrue);
    expect(attachment.viewerActionable, isTrue);
  });

  test(
    'NEGATIVE: actionability is NOT recomputed from embedded status '
    '(status ACTIVE but projection says not actionable)',
    () {
      final dto = MessageDto.fromJson(
        _messageJson(
          status: 'ACTIVE',
          isCurrent: false,
          viewerActionable: false,
        ),
      );
      final attachment = ChatMapper.messageToDomain(dto).shippingQuote!;
      // The embedded status says ACTIVE, yet the attachment must stay
      // not-actionable: only Commerce may decide actionability.
      expect(attachment.status, 'ACTIVE');
      expect(attachment.isCurrent, isFalse);
      expect(attachment.viewerActionable, isFalse);
    },
  );

  test(
    'NEGATIVE: actionability follows the projection, not the embedded status '
    '(status USED but projection says actionable)',
    () {
      final dto = MessageDto.fromJson(
        _messageJson(status: 'USED', isCurrent: true, viewerActionable: true),
      );
      final attachment = ChatMapper.messageToDomain(dto).shippingQuote!;
      expect(attachment.status, 'USED');
      expect(attachment.isCurrent, isTrue);
      expect(attachment.viewerActionable, isTrue);
    },
  );

  test('absent projection does not invent actionability', () {
    final dto = MessageDto.fromJson(_messageJson(status: 'ACTIVE'));
    expect(dto.shippingQuoteProjection, isNull);
    final attachment = ChatMapper.messageToDomain(dto).shippingQuote!;
    expect(attachment.isCurrent, isFalse);
    expect(attachment.viewerActionable, isFalse);
  });
}
