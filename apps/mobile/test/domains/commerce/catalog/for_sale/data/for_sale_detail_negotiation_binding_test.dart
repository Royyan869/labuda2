// for_sale detail wire — DEAL BINDING contract (viewer_negotiation_id).
//
// Backend authority (for-sale detail handler): the DETAIL payload carries
// `viewer_negotiation_id` ONLY when the viewer holds a settleable accepted
// negotiation (accepted, unexpired, unsettled) for this listing. The CTA
// forwards it into checkout as `negotiation_id` so the DEAL price — not the
// list price — is charged (owner truth: deal valid 24h from accept).
//
// Discovery payloads never carry the slot; when absent the CTA must send NO
// negotiation_id (list price stays authoritative).
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/data/dto/for_sale_dto.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/data/mappers/for_sale_dto_mapper.dart';

Map<String, dynamic> _detailJson() => <String, dynamic>{
  'id': 'for-sale-1',
  'product_id': 'product-1',
  'seller_id': 'seller-1',
  'title': 'Kohaku 30cm',
  'description': 'Premium kohaku',
  'price': 750000,
  'quantity': 1,
  'status': 'active',
  'created_at': '2026-09-24T00:00:00.000Z',
  'updated_at': '2026-09-24T00:00:00.000Z',
};

void main() {
  test('detail wire maps the settleable deal binding', () {
    final payload = _detailJson()
      ..['viewer_negotiation_id'] = 'nego-session-9';

    final dto = ForSaleResponseDto.fromJson(payload);
    final entity = ForSaleDtoMapper.toEntity(dto);

    expect(entity.viewerNegotiationId, 'nego-session-9');
  });

  test('payloads without a deal carry NO binding', () {
    final dto = ForSaleResponseDto.fromJson(_detailJson());
    final entity = ForSaleDtoMapper.toEntity(dto);

    expect(entity.viewerNegotiationId, isNull);
  });
}
