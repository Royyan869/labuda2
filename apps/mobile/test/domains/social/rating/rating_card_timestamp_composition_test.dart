// RATING CARD TIMESTAMP — HORIZONTAL COMPOSITION ACCEPTANCE TEST.
//
// The timestamp is canonical relative metadata. It competes with the footer
// action through Spacer, so it is flex-bounded and truncated rather than
// changing formatter output.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/rating/domain/entities/rating_entity.dart';
import 'package:labuda/domains/social/rating/presentation/widgets/rating_card.dart';
import 'package:labuda/domains/system/shared/domain/services/time_format_service.dart';

const List<double> _widths = <double>[320, 360, 412, 500];
const List<double> _scales = <double>[1.0, 1.3, 2.0];
const String _longBuyerName =
    'Koi Farm Nusantara Jaya Sentosa Premium Collection';
const String _longComment =
    'Sankei Kohaku Gin Rin Kanoko Premium review content intentionally long for composition audit';
const double _surfaceHeight = 1000;

final DateTime _now = DateTime.now();
final List<({DateTime createdAt, String expected})> _relativeFixtures = [
  (createdAt: _now, expected: 'baru saja'),
  (
    createdAt: _now.subtract(const Duration(minutes: 59)),
    expected: '59 menit lalu',
  ),
  (
    createdAt: _now.subtract(const Duration(days: 360)),
    expected: '12 bulan lalu',
  ),
  (
    createdAt: _now.subtract(const Duration(days: 1095)),
    expected: '3 tahun lalu',
  ),
];

Rating _rating(DateTime createdAt) => Rating(
  id: 'rating-1',
  orderId: 'order-1',
  buyerId: 'buyer-1',
  sellerId: 'seller-1',
  ratingValue: 5,
  comment: _longComment,
  createdAt: createdAt,
);

Widget _harness({
  required Size surface,
  required double scale,
  required Widget subject,
}) {
  return MaterialApp(
    theme: AppTheme.lightTheme,
    home: MediaQuery(
      data: MediaQueryData(
        size: surface,
        textScaler: TextScaler.linear(scale),
      ),
      child: Scaffold(
        body: SingleChildScrollView(child: subject),
      ),
    ),
  );
}

void _expectInsideSurface(WidgetTester tester, Size surface, Finder finder) {
  final Rect rect = tester.getRect(finder);
  expect(rect.left, greaterThanOrEqualTo(-0.5), reason: 'left of surface');
  expect(
    rect.right,
    lessThanOrEqualTo(surface.width + 0.5),
    reason: 'timestamp wider than surface at ${surface.width}dp',
  );
}

void main() {
  group('RatingCard timestamp — bounded compact composition', () {
    testWidgets('canonical relative strings survive full matrix', (
      tester,
    ) async {
      for (final ({DateTime createdAt, String expected}) fixture
          in _relativeFixtures) {
        expect(
          const TimeFormatService().formatTimeAgo(fixture.createdAt),
          fixture.expected,
        );

        for (final double width in _widths) {
          for (final double scale in _scales) {
            final Size surface = Size(width, _surfaceHeight);
            await tester.binding.setSurfaceSize(surface);
            addTearDown(() => tester.binding.setSurfaceSize(null));
            await tester.pumpWidget(
              _harness(
                surface: surface,
                scale: scale,
                subject: RatingCard(
                  rating: _rating(fixture.createdAt),
                  buyerName: _longBuyerName,
                ),
              ),
            );
            await tester.pump();

            expect(
              tester.takeException(),
              isNull,
              reason:
                  'rating timestamp overflow at ${fixture.expected} '
                  '${width}dp x $scale',
            );
            expect(find.text(fixture.expected), findsOneWidget);
            expect(find.text(_longBuyerName), findsOneWidget);

            final Text timestamp = tester.widget<Text>(
              find.text(fixture.expected),
            );
            expect(timestamp.data, fixture.expected);
            expect(timestamp.maxLines, 1);
            expect(timestamp.overflow, TextOverflow.ellipsis);
            _expectInsideSurface(
              tester,
              surface,
              find.text(fixture.expected),
            );
          }
        }
      }
    });

    testWidgets('tightest cell truncates timestamp, not formatter output', (
      tester,
    ) async {
      final DateTime createdAt = _now.subtract(
        const Duration(minutes: 59),
      );
      const expected = '59 menit lalu';
      expect(const TimeFormatService().formatTimeAgo(createdAt), expected);

      const Size surface = Size(320, _surfaceHeight);
      await tester.binding.setSurfaceSize(surface);
      addTearDown(() => tester.binding.setSurfaceSize(null));
      await tester.pumpWidget(
        _harness(
          surface: surface,
          scale: 2.0,
          subject: RatingCard(
            rating: _rating(createdAt),
            buyerName: _longBuyerName,
          ),
        ),
      );
      await tester.pump();

      expect(tester.takeException(), isNull);
      final Text timestamp = tester.widget<Text>(find.text(expected));
      expect(timestamp.data, expected);
      expect(timestamp.maxLines, 1);
      expect(timestamp.overflow, TextOverflow.ellipsis);
      _expectInsideSurface(tester, surface, find.text(expected));
    });
  });
}
