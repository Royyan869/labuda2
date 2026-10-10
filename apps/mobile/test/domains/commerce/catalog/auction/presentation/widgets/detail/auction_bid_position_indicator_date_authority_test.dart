// AUCTION BID POSITION INDICATOR — CLAIM DEADLINE DATE AUTHORITY GATE.
//
// Owner decision (authority purge):
//   Winner claim deadline renders through canonical AppFormatters.formatDateTime
//   (id_ID). The private unlocalized `DateFormat('MMM dd, yyyy • HH:mm')`
//   helper is purged.
//
// Semantic field (unchanged):
//   auction.settlementDeadline = endTime + 24h
//   (backend Auction.SettlementDeadline derivation; absolute claim deadline
//   for auction winners — not relative time, not countdown family).
//
// Canonical month abbreviation exposed by this gate:
//   May → Mei
//
// This gate proves the REAL AuctionBidPositionIndicator winner banner,
// source residue, and that AppFormatters remains the sole authority on this
// surface. It does NOT redesign the widget or touch other auction timestamps.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:intl/date_symbol_data_local.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:hishumi/domains/commerce/catalog/auction/domain/entities/auction_bid.dart';
import 'package:hishumi/domains/commerce/catalog/auction/domain/entities/auction_status.dart';
import 'package:hishumi/domains/commerce/catalog/auction/presentation/widgets/detail/auction_bid_position_indicator.dart';
import 'package:hishumi/shared/utils/app_formatters.dart';

const String _userId = 'winner-1';
const String _screenPath =
    'lib/domains/commerce/catalog/auction/presentation/widgets/detail/'
    'auction_bid_position_indicator.dart';

/// endTime chosen so endTime + 24h lands in May (Indonesian month proof).
final DateTime _endTime = DateTime(2024, 4, 30, 12, 0);
final DateTime _settlementDeadline =
    _endTime.add(const Duration(hours: 24)); // 2024-05-01 12:00
final String _canonicalDeadline =
    AppFormatters.formatDateTime(_settlementDeadline.toLocal());

Auction _winningAuction() => Auction(
  id: 'auction-claim-1',
  sellerId: 'seller-1',
  sellerUsername: 'seller_user',
  title: 'Sanke Auction',
  description: 'Ended auction with winner',
  koiDetails: const KoiDetails(
    variety: 'Kohaku',
    sizeInCm: 40,
    ageInMonths: 24,
    gender: 'male',
  ),
  openingBid: 1000000,
  currentBid: 1500000,
  bidIncrement: 50000,
  startTime: _endTime.subtract(const Duration(days: 1)),
  endTime: _endTime,
  status: AuctionStatus.ended,
  winnerId: _userId,
  createdAt: _endTime.subtract(const Duration(days: 2)),
);

List<AuctionBid> _userBids() => <AuctionBid>[
  AuctionBid(
    id: 'bid-1',
    auctionId: 'auction-claim-1',
    bidderId: _userId,
    bidderUsername: 'winner_user',
    amount: 1500000,
    createdAt: _endTime.subtract(const Duration(hours: 1)),
  ),
];

Future<void> _pumpIndicator(WidgetTester tester) async {
  await tester.pumpWidget(
    MaterialApp(
      theme: AppTheme.lightTheme,
      home: Scaffold(
        body: AuctionBidPositionIndicator(
          auction: _winningAuction(),
          userBids: _userBids(),
          currentUserId: _userId,
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() async {
    await initializeDateFormatting('id_ID');
  });

  group('AuctionBidPositionIndicator — AppFormatters claim deadline', () {
    test('canonical AppFormatters uses Indonesian month for the deadline', () {
      expect(_settlementDeadline, DateTime(2024, 5, 1, 12, 0));
      expect(_canonicalDeadline, '01 Mei 2024, 12:00');
      expect(AppFormatters.formatDateTime(_settlementDeadline.toLocal()),
          _canonicalDeadline);
    });

    testWidgets(
      'winner banner renders canonical Indonesian claim deadline',
      (tester) async {
        await _pumpIndicator(tester);

        expect(tester.takeException(), isNull);

        // Winner semantics remain represented.
        expect(find.textContaining('Anda Menang'), findsWidgets);
        expect(find.textContaining('Selesaikan sebelum'), findsOneWidget);

        // Canonical Indonesian month output appears in-card.
        expect(find.text('Selesaikan sebelum: $_canonicalDeadline'),
            findsOneWidget);

        // English month alternative from the purged local formatter must not.
        expect(find.textContaining('May 2024'), findsNothing);
        expect(find.textContaining('•'), findsNothing);
      },
    );

    test('production source has no private local date formatter residue', () {
      final String source = File(_screenPath)
          .readAsStringSync()
          .replaceAll('\r\n', '\n');

      expect(source.contains('DateFormat('), isFalse);
      expect(source.contains('MMM dd, yyyy • HH:mm'), isFalse);
      expect(source.contains("import 'package:intl/intl.dart'"), isFalse);
      expect(
        source.contains('AppFormatters.formatDateTime(deadline.toLocal())'),
        isTrue,
      );
      // Semantic family unchanged: absolute claim deadline, not relative.
      expect(source.contains('settlementDeadline'), isTrue);
    });
  });
}
