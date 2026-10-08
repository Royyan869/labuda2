// Dialog authority — Slice #4: auction cancel-confirmation family.
//
// The two eligible auction confirmations are the seller cancel-auction
// decisions: `AuctionDetailHandlers.handleCancel` and
// `SellerAuctionsScreen._cancelAuction`. Both now delegate composition to the
// canonical `AppDialog.confirm`; their business side effects stay in place.
//
// Proof:
//  1. POSITIVE (widget runtime): confirm performs the action + success
//     callback; cancel performs nothing; barrier dismissal performs nothing;
//     the destructive action carries the canonical error tone.
//  2. NEGATIVE: neither migrated consumer composes a raw dialog or holds type
//     authority; no `Auction*Dialog`/`Auction*Modal` wrapper was introduced.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart' show AppTheme;
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_state.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/detail/auction_detail_handlers.dart';

const _handlersPath =
    'lib/domains/commerce/catalog/auction/presentation/widgets/detail/'
    'auction_detail_handlers.dart';
const _sellerScreenPath =
    'lib/domains/commerce/catalog/auction/presentation/screens/'
    'seller_auctions_screen.dart';

/// Fake notifier: only the cancel funnel the handler actually calls is live.
class _FakeAuctionNotifier extends AuctionNotifier {
  final List<String> cancelled = <String>[];

  @override
  AuctionNotifierState build() => const AuctionNotifierState();

  @override
  Future<bool> cancelAuction({
    required String auctionId,
    required String sellerId,
    required String reason,
  }) async {
    cancelled.add(auctionId);
    return true;
  }
}

Auction _auction() => Auction(
  id: 'a1',
  sellerId: 'seller-1',
  title: 'Kohaku 50cm',
  description: 'desc',
  koiDetails: const KoiDetails(
    variety: 'Kohaku',
    sizeInCm: 50,
    ageInMonths: 12,
    gender: 'male',
  ),
  openingBid: 1000000,
  currentBid: 1200000,
  bidIncrement: 100000,
  startTime: DateTime.utc(2026, 7, 1, 8),
  endTime: DateTime.utc(2026, 7, 2, 8),
  status: AuctionStatus.scheduled,
  createdAt: DateTime.utc(2026, 7, 1, 7),
);

Widget _harness({
  required _FakeAuctionNotifier notifier,
  required VoidCallback onCancelSuccess,
}) => ProviderScope(
  overrides: [auctionNotifierProvider.overrideWith(() => notifier)],
  child: MaterialApp(
    theme: AppTheme.lightTheme,
    home: Scaffold(
      body: Consumer(
        builder: (context, ref, _) => ElevatedButton(
          onPressed: () => AuctionDetailHandlers(
            ref: ref,
            context: context,
            auction: _auction(),
            auctionId: 'a1',
            onEditSuccess: () {},
            onCancelSuccess: onCancelSuccess,
          ).handleCancel(),
          child: const Text('cancel'),
        ),
      ),
    ),
  ),
);

void main() {
  group('auction cancel confirmation — positive (handler runtime)', () {
    testWidgets('confirm cancels once and runs the success callback', (
      tester,
    ) async {
      final notifier = _FakeAuctionNotifier();
      var cancelled = false;
      await tester.pumpWidget(
        _harness(notifier: notifier, onCancelSuccess: () => cancelled = true),
      );

      await tester.tap(find.text('cancel'));
      await tester.pumpAndSettle();

      expect(find.text('Batalkan Lelang'), findsOneWidget);
      final scheme = Theme.of(
        tester.element(find.byType(AlertDialog)),
      ).colorScheme;
      final confirm = tester.widget<ElevatedButton>(
        find.widgetWithText(ElevatedButton, 'Batalkan'),
      );
      expect(
        confirm.style?.backgroundColor?.resolve(const <WidgetState>{}),
        scheme.error,
        reason: 'auction cancellation is destructive',
      );

      await tester.tap(find.text('Batalkan'));
      await tester.pumpAndSettle();

      expect(notifier.cancelled, <String>['a1']);
      expect(cancelled, isTrue);
      expect(find.byType(AlertDialog), findsNothing);
    });

    testWidgets('cancel performs no action', (tester) async {
      final notifier = _FakeAuctionNotifier();
      var cancelled = false;
      await tester.pumpWidget(
        _harness(notifier: notifier, onCancelSuccess: () => cancelled = true),
      );

      await tester.tap(find.text('cancel'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Batal'));
      await tester.pumpAndSettle();

      expect(notifier.cancelled, isEmpty);
      expect(cancelled, isFalse);
    });

    testWidgets('barrier dismissal performs no action', (tester) async {
      final notifier = _FakeAuctionNotifier();
      var cancelled = false;
      await tester.pumpWidget(
        _harness(notifier: notifier, onCancelSuccess: () => cancelled = true),
      );

      await tester.tap(find.text('cancel'));
      await tester.pumpAndSettle();
      await tester.tapAt(const Offset(5, 5));
      await tester.pumpAndSettle();

      expect(notifier.cancelled, isEmpty);
      expect(cancelled, isFalse);
    });
  });

  group('auction cancel confirmation — negative gate', () {
    test('both migrated consumers compose no raw dialog', () {
      for (final path in <String>[_handlersPath, _sellerScreenPath]) {
        final source = File(path).readAsStringSync();
        for (final forbidden in <String>[
          'AlertDialog(',
          'showDialog(',
          'showDialog<bool>(',
          'AppType',
          'fontSize:',
          'Colors.',
          'Color(0x',
        ]) {
          expect(
            source.contains(forbidden),
            isFalse,
            reason: '$path must not compose a raw dialog ($forbidden)',
          );
        }
        expect(
          source.contains('AppDialog.confirm('),
          isTrue,
          reason: '$path must consume the canonical confirmation authority',
        );
        expect(source.contains('AppDialogIntent.destructive'), isTrue);
      }
    });

    test('no auction-local dialog wrapper authority was introduced', () {
      final offenders = <String>[];
      for (final file
          in Directory('lib/domains/commerce/catalog/auction')
              .listSync(recursive: true)
              .whereType<File>()
              .where((f) => f.path.endsWith('.dart'))) {
        final source = file.readAsStringSync();
        for (final name in <String>[
          'class AuctionDialog',
          'class AuctionDialogs',
          'class AuctionModal',
          'class AuctionConfirm',
        ]) {
          if (source.contains(name)) offenders.add('${file.path}: $name');
        }
      }
      expect(offenders, isEmpty, reason: offenders.join('\n'));
    });
  });
}
