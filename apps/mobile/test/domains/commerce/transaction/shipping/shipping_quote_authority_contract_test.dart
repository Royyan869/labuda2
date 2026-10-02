// Shipping quote AUTHORITY contract — Owner decision 2026-10-01.
//
// Canonical authority: the Shipping domain owns the manual shipping quote
// (ongkir) END-TO-END — target gate, form, request builder, transport.
// Chat is a display layer that forwards the intent
// (`openSellerShippingQuoteSheet`) and carries ZERO write capability: the
// former ChatNotifier / ChatRepository / ChatApiDatasource
// `createShippingQuote` path was a competing authority and is killed.
//
// These proofs lock both directions:
//   positive — Shipping's resolver + builder produce the canonical wire,
//              and the chat screen forwards (never builds) the intent;
//   negative — the chat domain contains no quote producer, no DTO import
//              and no endpoint, so the competitor cannot resurrect.
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/shipping_quote_intent.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';

const _productId = '11111111-1111-1111-1111-111111111111';
const _forSaleId = '22222222-2222-2222-2222-222222222222';

/// Server-resolved projection fixtures: the ONLY gate signal the CTA sees.
ResourceProjection _liveForSaleProjection({required bool owner}) {
  return LiveResourceProjection(
    state: ResourceProjectionState.live,
    resourceType: ResourceProjectionType.fixedPriceSale,
    viewerCapabilities: ResourceViewerCapabilities.live(canInteract: owner),
    resourceId: _forSaleId,
    canonicalUrl: 'https://labuda.test/for-sale/$_forSaleId',
    commerceActions: CommerceActionCapabilities(
      role: owner ? 'owner' : 'buyer',
      canChat: !owner,
      canNegotiate: !owner,
      canBuy: !owner,
      canBid: false,
      canManage: owner,
    ),
    payload: const ForSaleLivePayload(
      title: 'Ikan Nemo Sehat',
      media: [],
      price: LivePrice(amount: 150000, currency: 'IDR'),
      status: 'available',
      quantityAvailable: 3,
      seller: ResourceSellerCard(
        user: ResourceUserCard(id: 'seller-1', username: 'peternak'),
      ),
    ),
  );
}

void main() {
  group('Positive — Shipping resolver gates on server-owned capability', () {
    test('owner of a LIVE for_sale projection resolves the target', () {
      final target = resolveSellerShippingQuoteTarget(
        _liveForSaleProjection(owner: true),
      );

      expect(target, isNotNull);
      expect(target!.forSaleId, _forSaleId);
      expect(target.title, 'Ikan Nemo Sehat');
      // The for-sale surface id, never the physical product id — the
      // intent resolves the product fresh, exactly like checkout.
      expect(target.forSaleId, isNot(equals(_productId)));
    });

    test('buyer never resolves a target (canManage is false)', () {
      expect(
        resolveSellerShippingQuoteTarget(_liveForSaleProjection(owner: false)),
        isNull,
      );
    });

    test('tombstone never resolves a target', () {
      expect(
        resolveSellerShippingQuoteTarget(
          const TombstoneResourceProjection(
            state: ResourceProjectionState.tombstone,
            resourceType: ResourceProjectionType.fixedPriceSale,
            viewerCapabilities: ResourceViewerCapabilities.tombstone(),
            resourceId: _forSaleId,
          ),
        ),
        isNull,
      );
    });

    test('a missing projection never resolves a target', () {
      expect(resolveSellerShippingQuoteTarget(null), isNull);
    });
  });

  group('Positive — canonical create-quote wire (Shipping-owned builder)', () {
    test('builds the canonical backend fields', () {
      final request = buildForSaleShippingQuoteRequest(
        productId: _productId,
        forSaleId: _forSaleId,
        cost: 25000,
        note: 'catatan',
        destinationCityId: '3171',
        destinationProvinceId: '31',
      );

      final json = request.toJson();
      expect(json['product_id'], _productId);
      expect(json['source_type'], 'for_sale');
      expect(json['source_id'], _forSaleId);
      expect(json['cost'], 25000);
      expect(json['note'], 'catatan');
      expect(json.containsKey('for_sale_id'), isFalse);
      expect(json.containsKey('auction_id'), isFalse);
      // Expiry is backend-authoritative (24h default, max 7d) — the client
      // never sends it.
      expect(json.containsKey('expires_in_hours'), isFalse);
      // DESTINATION LOCK (Owner 2026-10-01): the wire MUST carry the
      // kota/kabupaten lock — the backend rejects a for_sale quote without
      // it, and an unlocked quote would be consumable by any buyer address.
      expect(json['destination_city_id'], '3171');
      expect(json['destination_province_id'], '31');
    });

    test('the seller form requires the kota/kabupaten destination (no deeper)', () {
      final form = File(
        'lib/domains/commerce/transaction/shipping/presentation/widgets/'
        'shipping_quote_form_sheet.dart',
      ).readAsStringSync();
      expect(form, contains('ProvinceDropdown'));
      expect(form, contains('CityDropdown'));
      expect(form, contains('Kota/Kabupaten Tujuan'));
      expect(form, contains('destinationCityId'));
      // Kota/kabupaten level only — never kecamatan or desa selectors.
      expect(form, isNot(contains('DistrictDropdown')));
      expect(form, isNot(contains('VillageDropdown')));
    });
  });

  group('Negative — chat carries zero shipping-quote write capability', () {
    test('no chat source names the producer, the DTO or the endpoint', () {
      final chatDir = Directory('lib/domains/chat');
      expect(chatDir.existsSync(), isTrue, reason: 'scan scope must be real');

      final offenders = <String>[];
      for (final entity in chatDir.listSync(recursive: true)) {
        if (entity is! File || !entity.path.endsWith('.dart')) continue;
        final lines = entity.readAsStringSync().split('\n');
        for (var i = 0; i < lines.length; i++) {
          final line = lines[i];
          if (line.contains('createShippingQuote') ||
              line.contains('shipping_quote_dto.dart') ||
              line.contains('/shipping-quote')) {
            offenders.add('${entity.path}:${i + 1}: ${line.trim()}');
          }
        }
      }

      expect(
        offenders,
        isEmpty,
        reason: 'chat must forward the ongkir intent to the Shipping domain, '
            'never build or transport it (Owner 2026-10-01)',
      );
    });

    test('the chat screen FORWARDS via the Shipping entry only', () {
      final screen = File(
        'lib/domains/chat/chat/presentation/screens/chat_detail_screen.dart',
      ).readAsStringSync();
      expect(screen, contains('openSellerShippingQuoteSheet'));
      expect(screen, isNot(contains('CreateShippingQuoteRequestDto')));
      expect(screen, isNot(contains('buildForSaleShippingQuoteRequest')));
    });

    test('the projection card gates the CTA on server-owned canManage', () {
      final card = File(
        'lib/domains/chat/chat/presentation/widgets/'
        'chat_resource_projection_card.dart',
      ).readAsStringSync();
      expect(card, contains('actions.canManage'));
      expect(card, contains("ResourceProjectionType.fixedPriceSale"));
      expect(card, contains("'Kirim Ongkir'"));
    });
  });
}
