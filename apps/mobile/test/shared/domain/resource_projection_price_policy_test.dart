/// PRICE DISPLAY POLICY RATCHET (owner decision, 2026-09-27).
///
/// 1. The canonical envelope carries the money on LIVE for every surface;
///    every surface renders it — chat and discovery show the SAME string.
/// 2. One formatting authority: the strings come from the envelope
///    (`LivePrice.formatted`, `ForSaleLivePayload.formattedPrice`,
///    `AuctionLivePayload.formattedAmount`). A card that formats money itself
///    would be a second authority for the same number.
/// 3. Current bid wins over buy-now; an auction with neither renders no amount
///    (0 is never invented).
/// 4. TOMBSTONE never carries and never renders money — its wire shape is
///    exactly {state, resource_type, resource_id, viewer_capabilities} and any
///    payload half (including a price) is rejected at parse time.
library;

import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat_resource_projection_card.dart';
import 'package:labuda/domains/social/content/presentation/widgets/content_resource_projection_card.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';

Map<String, dynamic> _liveEnvelope(String resourceType, String id) => {
  'state': 'LIVE',
  'resource_type': resourceType,
  'resource_id': id,
  'canonical_url': '/$resourceType/$id',
  'viewer_capabilities': const {
    'can_view': true,
    'can_interact': false,
    'blocked_by_tombstone': false,
  },
};

Map<String, dynamic> _tombstoneEnvelope(String resourceType, String id) => {
  'state': 'TOMBSTONE',
  'resource_type': resourceType,
  'resource_id': id,
  'viewer_capabilities': const {
    'can_view': false,
    'can_interact': false,
    'blocked_by_tombstone': true,
  },
};

Map<String, dynamic> _commerceActions({
  required bool canBuy,
  required bool canBid,
}) => {
  'role': 'buyer',
  'can_chat': true,
  'can_negotiate': canBuy,
  'can_buy': canBuy,
  'can_bid': canBid,
  'can_manage': false,
};

Map<String, dynamic> _saleJson({
  int price = 1250000,
  String currency = 'IDR',
  String status = 'active',
  bool canBuy = true,
}) => {
  ..._liveEnvelope('for_sale', 'sale-1'),
  'viewer_capabilities': {
    'can_view': true,
    'can_interact': canBuy,
    'blocked_by_tombstone': false,
  },
  'commerce_actions': _commerceActions(canBuy: canBuy, canBid: false),
  'for_sale': {
    'title': 'Kohaku 45 cm',
    'media': const <Map<String, dynamic>>[],
    'price': {'amount': price, 'currency': currency},
    'status': status,
    'quantity_available': 1,
    'seller': {
      'user': {'id': 'seller-1', 'username': 'seller'},
    },
  },
};

Map<String, dynamic> _auctionJson({
  int? currentBid = 1450000,
  int? buyNowPrice = 1750000,
  String lifecycle = 'active',
}) => {
  ..._liveEnvelope('auction', 'auction-1'),
  'viewer_capabilities': {
    'can_view': true,
    'can_interact': false,
    'blocked_by_tombstone': false,
  },
  'commerce_actions': _commerceActions(canBuy: false, canBid: false),
  'auction': {
    'title': 'Lelang Jumbo',
    'media': const <Map<String, dynamic>>[],
    'current_bid': ?currentBid,
    'buy_now_price': ?buyNowPrice,
    'end_at': '2026-12-10T12:34:56Z',
    'lifecycle': lifecycle,
    'seller': {
      'user': {'id': 'seller-2', 'username': 'auction_seller'},
    },
  },
};

Map<String, dynamic> _profileJson() => {
  ..._liveEnvelope('profile', 'profile-1'),
  'profile': const {
    'username': 'alice',
    'store_name': 'Toko Alice',
    'lifecycle': 'active',
  },
};

Map<String, dynamic> _contentJson() => {
  ..._liveEnvelope('content', 'content-1'),
  'content': const {
    'caption': 'Konten utama',
    'media': <Map<String, dynamic>>[],
    'lifecycle': 'active',
    'created_at': '2026-08-08T10:11:12Z',
    'author': {'id': 'author-1', 'username': 'author_user'},
  },
};

Future<void> _pumpChatCard(WidgetTester tester, ResourceProjection projection) {
  return tester.pumpWidget(
    MaterialApp(
      home: Scaffold(
        body: SingleChildScrollView(
          child: ChatResourceProjectionCard(resourceProjection: projection),
        ),
      ),
    ),
  );
}

Future<void> _pumpDiscoveryCard(
  WidgetTester tester,
  ResourceProjection projection,
) {
  return tester.pumpWidget(
    MaterialApp(
      home: Scaffold(
        body: SingleChildScrollView(
          child: ContentResourceProjectionCard(resourceProjection: projection),
        ),
      ),
    ),
  );
}

/// Every text the tree renders, joined for negative assertions.
String _renderedText(WidgetTester tester) => tester
    .widgetList<Text>(find.byType(Text))
    .map((text) => text.data ?? '')
    .join(' | ');

/// The money strings actually rendered (grouped-thousands form only).
List<String> _renderedMoney(WidgetTester tester) =>
    RegExp(r'Rp [\d.]+')
        .allMatches(_renderedText(tester))
        .map((match) => match.group(0)!)
        .toList();

void main() {
  group('for_sale LIVE', () {
    testWidgets('chat and discovery render the identical canonical amount', (
      tester,
    ) async {
      final projection = ResourceProjection.fromJson(_saleJson());

      await _pumpChatCard(tester, projection);
      final chatMoney = _renderedMoney(tester);

      await _pumpDiscoveryCard(tester, projection);
      final discoveryMoney = _renderedMoney(tester);

      expect(chatMoney, ['Rp 1.250.000']);
      expect(discoveryMoney, chatMoney);
    });

    testWidgets('the chat status caption survives the price', (tester) async {
      await _pumpChatCard(
        tester,
        ResourceProjection.fromJson(_saleJson(status: 'sold')),
      );

      expect(find.text('Rp 1.250.000'), findsOneWidget);
      expect(find.text('Terjual'), findsOneWidget);
    });

    testWidgets('a non-IDR amount keeps its explicit currency', (tester) async {
      await _pumpChatCard(
        tester,
        ResourceProjection.fromJson(_saleJson(price: 1250000, currency: 'USD')),
      );

      expect(find.text('USD 1.250.000'), findsOneWidget);
    });
  });

  group('auction LIVE', () {
    testWidgets('the current bid wins over buy-now on both surfaces', (
      tester,
    ) async {
      final projection = ResourceProjection.fromJson(_auctionJson());

      await _pumpChatCard(tester, projection);
      final chatMoney = _renderedMoney(tester);

      await _pumpDiscoveryCard(tester, projection);
      final discoveryMoney = _renderedMoney(tester);

      expect(chatMoney, ['Rp 1.450.000']);
      expect(discoveryMoney, chatMoney);
      expect(_renderedText(tester), isNot(contains('Rp 1.750.000')));
    });

    testWidgets('without any bid the buy-now price is the honest amount', (
      tester,
    ) async {
      await _pumpChatCard(
        tester,
        ResourceProjection.fromJson(
          _auctionJson(currentBid: null, buyNowPrice: 1750000),
        ),
      );

      expect(find.text('Rp 1.750.000'), findsOneWidget);
    });

    testWidgets('an auction with neither amount renders no money at all', (
      tester,
    ) async {
      final projection = ResourceProjection.fromJson(
        _auctionJson(currentBid: null, buyNowPrice: null),
      );

      await _pumpChatCard(tester, projection);
      expect(_renderedMoney(tester), isEmpty);

      await _pumpDiscoveryCard(tester, projection);
      expect(_renderedMoney(tester), isEmpty);
    });
  });

  group('profile/content LIVE', () {
    testWidgets('identity surfaces never render money', (tester) async {
      for (final json in [_profileJson(), _contentJson()]) {
        final projection = ResourceProjection.fromJson(json);

        await _pumpChatCard(tester, projection);
        expect(_renderedMoney(tester), isEmpty);

        await _pumpDiscoveryCard(tester, projection);
        expect(_renderedMoney(tester), isEmpty);
      }
    });
  });

  group('TOMBSTONE', () {
    test('the wire carries exactly the four canonical keys', () {
      for (final type in ['for_sale', 'auction', 'profile', 'content']) {
        final projection = ResourceProjection.fromJson(
          _tombstoneEnvelope(type, '$type-1'),
        );
        expect(
          projection.toJson().keys.toSet(),
          {'state', 'resource_type', 'resource_id', 'viewer_capabilities'},
        );
        expect(projection.payload, isNull);
        expect(projection.canonicalUrl, isNull);
      }
    });

    testWidgets('no surface renders money for a dead resource', (tester) async {
      final sale = ResourceProjection.fromJson(
        _tombstoneEnvelope('for_sale', 'sale-1'),
      );

      await _pumpChatCard(tester, sale);
      expect(_renderedMoney(tester), isEmpty);
      expect(find.text('Tidak dapat ditampilkan'), findsOneWidget);

      await _pumpDiscoveryCard(tester, sale);
      expect(_renderedMoney(tester), isEmpty);
      expect(find.text('TOMBSTONE'), findsWidgets);
    });

    test('a tombstone carrying a price payload half is rejected', () {
      expect(
        () => ResourceProjection.fromJson({
          ..._tombstoneEnvelope('for_sale', 'sale-1'),
          'for_sale': _saleJson()['for_sale'],
        }),
        throwsFormatException,
      );
    });
  });

  group('one formatting authority', () {
    test('cards never format money themselves', () {
      // A card that builds its own 'Rp …' string is a second authority for the
      // same number: the envelope owns the display string.
      for (final path in [
        'lib/domains/chat/chat/presentation/widgets/chat_resource_projection_card.dart',
        'lib/domains/social/content/presentation/widgets/content_resource_projection_card.dart',
      ]) {
        final source = File(path).readAsStringSync();
        expect(
          source.contains("'Rp "),
          isFalse,
          reason: '$path must render the envelope money strings',
        );
      }

      final entity = File(
        'lib/shared/domain/entities/resource_projection.dart',
      ).readAsStringSync();
      expect(entity.contains("'Rp "), isTrue);
      expect(entity.contains('formattedPrice'), isTrue);
      expect(entity.contains('formattedAmount'), isTrue);
    });

    test('thousand separators are implemented once, not per widget', () {
      // A hand-rolled grouping regex is the fingerprint of a second money
      // authority: five widgets had its own copy before this ratchet. The
      // canonical formatter groups with a loop; the only tolerated regex is
      // the price input mask, which groups with ',' while the user types and
      // parses that ',' back out.
      const allowed = {'lib/shared/ui/atomic/input/price_input_component.dart'};
      final offenders = <String>[];
      for (final entity in Directory('lib').listSync(recursive: true)) {
        if (entity is! File || !entity.path.endsWith('.dart')) continue;
        final path = entity.path.replaceAll('\\', '/');
        if (allowed.contains(path)) continue;
        if (entity.readAsStringSync().contains(r'(?=(\d{3})+(?!\d))')) {
          offenders.add(path);
        }
      }
      expect(
        offenders,
        isEmpty,
        reason: 'group money with formatGroupedAmount instead: $offenders',
      );
    });

    test('no surface builds a Rupiah string from a raw number', () {
      // `Rp ${amount.toStringAsFixed(0)}` is the fingerprint of a local money
      // formatter: it silently drops the thousands grouping, so the same
      // number renders as `Rp 1250000` on one screen and `Rp 1.250.000` on
      // another. Money display goes through formatGroupedAmount or a payload
      // getter. Only the compact-shorthand util keeps its own rounding.
      final pattern = RegExp(r'Rp\s*\$\{[^}]*toStringAsFixed');
      final offenders = <String>[];
      for (final entity in Directory('lib').listSync(recursive: true)) {
        if (entity is! File || !entity.path.endsWith('.dart')) continue;
        final path = entity.path.replaceAll('\\', '/');
        if (path.startsWith('lib/generated/')) continue;
        if (path == 'lib/shared/utils/currency_utils.dart') continue;
        for (final line in entity.readAsLinesSync()) {
          if (!pattern.hasMatch(line)) continue;
          offenders.add('$path: $line');
        }
      }
      expect(
        offenders,
        isEmpty,
        reason: 'build Rupiah through formatGroupedAmount: $offenders',
      );
    });

    test('a Rupiah string never interpolates an unformatted number', () {
      // `'Rp $amount'` / `'Rp ${openingBid}'` is the ungrouped sibling of
      // `toStringAsFixed`: the same number then renders `Rp 1000000` on this
      // screen and `Rp 1.000.000` on the next, and a double would even leak
      // its tail (`Rp 50000.0`). The interpolation must go through
      // formatGroupedAmount or a payload getter that already did.
      final pattern = RegExp(r'Rp\s*\$\{?[\w.]');
      const allowed = <String>{
        'lib/generated/', // generated localizations, not money surfaces
        'lib/shared/utils/currency_utils.dart', // shorthand engine (K/Jt/M)
      };
      final offenders = <String>[];
      for (final entity in Directory('lib').listSync(recursive: true)) {
        if (entity is! File || !entity.path.endsWith('.dart')) continue;
        final path = entity.path.replaceAll('\\', '/');
        if (allowed.any(path.startsWith)) continue;
        for (final line in entity.readAsLinesSync()) {
          if (!pattern.hasMatch(line)) continue;
          if (line.contains('formatGroupedAmount(')) continue;
          // Bid input hint: digits-only field, the validator rejects '.', so a
          // grouped hint would invite input that must fail (design exception).
          if (line.contains('hintText:')) continue;
          offenders.add('$path: $line');
        }
      }
      expect(
        offenders,
        isEmpty,
        reason: 'wrap the amount in formatGroupedAmount: $offenders',
      );
    });
  });
}
