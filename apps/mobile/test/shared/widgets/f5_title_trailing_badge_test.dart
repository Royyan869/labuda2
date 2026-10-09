// F5 — TITLE + TRAILING BADGE/HEADER COMPOSITION REGRESSION SUITE.
//
// Business truth: title/badge share one row when width permits; the title
// (compressible chrome) yields via bounded flex + ellipsis; business-
// meaningful badges stay fully readable (never ellipsized); two-data rows
// recompose vertically when horizontal cannot fit.
//
// Matrix: 320 / 360 / 412 / 500 x text scale 1.0 / 1.3 / 2.0 with realistic
// long Indonesian titles and actual production badge strings.
// Hosts too heavy to pump (refund list section, seller tab, feed, checkout
// section) are covered by pattern-identical reconstructions + call-site pins.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:intl/date_symbol_data_local.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/order/presentation/widgets/order_widgets.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/personal_information_section.dart';
import 'package:labuda/generated/app_localizations.dart';

const List<double> _widths = <double>[320, 360, 412, 500];
const List<double> _scales = <double>[1.0, 1.3, 2.0];

Widget _harness({
  required Size surface,
  required double scale,
  required Widget subject,
}) {
  return MaterialApp(
    theme: AppTheme.lightTheme,
    locale: const Locale('id'),
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    home: MediaQuery(
      data: MediaQueryData(size: surface, textScaler: TextScaler.linear(scale)),
      child: Scaffold(
        body: Center(
          child: SizedBox(
            width: surface.width,
            child: SingleChildScrollView(child: subject),
          ),
        ),
      ),
    ),
  );
}

Future<void> _pump(
  WidgetTester tester, {
  required double width,
  required double scale,
  required Widget subject,
  double height = 800,
}) async {
  final Size surface = Size(width, height);
  await tester.binding.setSurfaceSize(surface);
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(
    _harness(surface: surface, scale: scale, subject: subject),
  );
  await tester.pump();
}

void _expectClean(
  WidgetTester tester, {
  required String label,
  required double width,
  required double scale,
}) {
  final Object? exception = tester.takeException();
  expect(
    exception,
    isNull,
    reason: 'layout exception: $label at ${width}dp @scale $scale '
        '($exception)',
  );
}

Rect _rectOf(WidgetTester tester, String text) =>
    tester.getRect(find.text(text));

void _expectInside(
  WidgetTester tester, {
  required String text,
  required Rect bounds,
}) {
  final Rect rect = _rectOf(tester, text);
  expect(
    rect.left >= bounds.left - 0.5 && rect.right <= bounds.right + 0.5,
    isTrue,
    reason: '"$text" left its card ($rect vs $bounds)',
  );
}

/// Badge/status text must exist and must NOT carry ellipsis (business-
/// meaningful content is never truncated to save a row).
void _expectBadgeIntact(WidgetTester tester, String badgeText) {
  expect(find.text(badgeText), findsOneWidget);
  final Text widget = tester.widget<Text>(find.text(badgeText));
  expect(
    widget.overflow,
    isNot(TextOverflow.ellipsis),
    reason: 'badge "$badgeText" must never be ellipsized',
  );
  expect(widget.maxLines, isNull, reason: 'badge "$badgeText" needs no cap');
}

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const ShippingInfo _shippingInfo = ShippingInfo(
  recipientName: 'Pembeli Nusantara',
  phone: '081234567890',
  address: 'Jl. Kolam Koi No. 48, Blimbing, Malang',
  method: ShippingMethod.bus,
  shippingCost: 25000,
);

RefundRequest _refund(RefundStatus status) => RefundRequest(
  id: 'refund-1',
  orderId: 'order-1',
  buyerId: 'buyer-1',
  sellerId: 'seller-1',
  reason: RefundReason.itemDamaged,
  status: status,
  refundAmount: 750000,
  createdAt: DateTime(2026, 5, 18, 9, 0),
);

Order _order({required PaymentStatus? paymentStatus}) => Order(
  id: 'order-1',
  buyerId: 'buyer-1',
  sellerId: 'seller-1',
  items: const [],
  status: OrderStatus.paid,
  paymentMethodCode: 'bank_transfer',
  paymentStatus: paymentStatus,
  shippingInfo: _shippingInfo,
  pricing: const OrderPricing(
    subtotal: 1250000,
    shippingCost: 25000,
    totalPayableAmount: 1280000,
    totalBeforeCoinsAmount: 1275000,
  ),
  createdAt: DateTime(2026, 5, 16, 10, 0),
  paidAt: DateTime(2026, 5, 17, 14, 30),
  source: OrderSource.forSale,
);

Order _overdueOrder({int daysOverdue = 45}) => Order(
  id: 'order-od-1',
  buyerId: 'buyer-1',
  sellerId: 'seller-1',
  items: const [],
  status: OrderStatus.paid,
  paymentStatus: PaymentStatus.paid,
  shippingInfo: _shippingInfo,
  pricing: const OrderPricing(
    subtotal: 1250000,
    shippingCost: 25000,
    totalPayableAmount: 1280000,
    totalBeforeCoinsAmount: 1275000,
  ),
  isOverdue: true,
  overdueTier: 'critical_overdue',
  overdueDays: daysOverdue,
  readyToShipBy: DateTime(2026, 4, 2),
  createdAt: DateTime(2026, 5, 16, 10, 0),
  paidAt: DateTime(2026, 5, 17, 14, 30),
  source: OrderSource.forSale,
);

// ---------------------------------------------------------------------------
// Fixed-pattern reconstructions (NOT production — mirror the converged
// production rows for hosts too heavy to pump; production conformance is
// pinned by call-site tests below).
// ---------------------------------------------------------------------------

/// Pattern-identical adaptive measure (mirrors the F5-local production
/// helpers; test-local copy since production helpers are file-private).
bool _fitsReconSingleLine({
  required BuildContext context,
  required double maxWidth,
  required String title,
  required String badge,
  required TextStyle? titleStyle,
  required TextStyle? badgeStyle,
  required double fixedExtrasWidth,
}) {
  if (!maxWidth.isFinite) {
    return false;
  }
  final TextDirection direction = Directionality.of(context);
  final TextScaler scaler = MediaQuery.textScalerOf(context);

  double singleLineWidth(String text, TextStyle? style) {
    final TextPainter painter = TextPainter(
      text: TextSpan(text: text, style: style),
      textDirection: direction,
      textScaler: scaler,
      maxLines: 1,
    )..layout();
    return painter.width;
  }

  const double safetyMargin = 2;
  return singleLineWidth(title, titleStyle) +
          fixedExtrasWidth +
          singleLineWidth(badge, badgeStyle) +
          safetyMargin <=
      maxWidth;
}

// ---------------------------------------------------------------------------
// Fixed-pattern reconstructions (NOT production — mirror the converged
// production rows for hosts too heavy to pump; production conformance is
// pinned by call-site tests below).
// ---------------------------------------------------------------------------

/// profile_about_tab seller header, converged: adaptive title + intact badge.
Widget _sellerStatusRow() => Builder(
  builder: (BuildContext context) {
    final ColorScheme scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(16),
      child: LayoutBuilder(
        builder: (BuildContext context, BoxConstraints constraints) {
          final TextStyle titleStyle = context.typeRoles.bodyDense.copyWith(
            fontWeight: FontWeight.w500,
            color: scheme.onSurfaceVariant,
          );
          final TextStyle badgeStyle = context.typeRoles.labelMicro.copyWith(
            fontWeight: FontWeight.w600,
          );
          final bool fits = _fitsReconSingleLine(
            context: context,
            maxWidth: constraints.maxWidth,
            title: 'Status Penjual',
            badge: 'Bukan Penjual',
            titleStyle: titleStyle,
            badgeStyle: badgeStyle,
            fixedExtrasWidth: 20 + 8 + 24,
          );
          final Widget badge = Container(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
            decoration: BoxDecoration(
              color: scheme.primary.withValues(alpha: 0.15),
              borderRadius: BorderRadius.circular(12),
            ),
            child: Text(
              'Bukan Penjual',
              style: context.typeRoles.labelMicro.copyWith(
                fontWeight: FontWeight.w600,
                color: scheme.primary,
              ),
              softWrap: true,
            ),
          );
          if (fits) {
            return Row(
              children: [
                Icon(
                  Icons.person_outline,
                  color: scheme.onSurfaceVariant,
                  size: 20,
                ),
                const SizedBox(width: 8),
                Text('Status Penjual', style: titleStyle),
                const Spacer(),
                badge,
              ],
            );
          }
          return Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                'Status Penjual',
                style: titleStyle,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
              const SizedBox(height: 4),
              badge,
            ],
          );
        },
      ),
    );
  },
);

/// feed_renderers promo row, converged: adaptive static marker + intact
/// live countdown.
Widget _feedPromoRow() => Builder(
  builder: (BuildContext context) {
    final ColorScheme scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(16),
      child: LayoutBuilder(
        builder: (BuildContext context, BoxConstraints constraints) {
          final TextStyle badgeStyle = context.typeRoles.labelMicro.copyWith(
            color: scheme.primary,
            fontWeight: FontWeight.w600,
          );
          final TextStyle countdownStyle = context.typeRoles.labelMicro
              .copyWith(
                color: context.statusColors.warning,
                fontWeight: FontWeight.w600,
              );
          final bool fits = _fitsReconSingleLine(
            context: context,
            maxWidth: constraints.maxWidth,
            title: 'Dipromosikan',
            badge: '45h tersisa',
            titleStyle: badgeStyle,
            badgeStyle: countdownStyle,
            fixedExtrasWidth: 16 + 4 + 16 + 16,
          );
          Widget countdown() => Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
            decoration: BoxDecoration(
              color: context.statusColors.warning.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(4),
            ),
            child: Text(
              '45h tersisa',
              style: context.typeRoles.labelMicro.copyWith(
                color: context.statusColors.warning,
                fontWeight: FontWeight.w600,
              ),
              softWrap: true,
            ),
          );
          const Widget marker = Text('Dipromosikan');
          if (fits) {
            return Row(
              children: [
                Text('Dipromosikan', style: badgeStyle),
                const Spacer(),
                countdown(),
              ],
            );
          }
          return Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [marker, const SizedBox(height: 4), countdown()],
          );
        },
      ),
    );
  },
);

/// checkout title + Lelang chip, converged: adaptive title + intact chip.
Widget _checkoutTitleRow() => Builder(
  builder: (BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      child: LayoutBuilder(
        builder: (BuildContext context, BoxConstraints constraints) {
          final TextStyle titleStyle = context.typeRoles.titleSection
              .copyWith(fontWeight: FontWeight.bold);
          final TextStyle chipStyle = context.typeRoles.labelMicro.copyWith(
            fontWeight: FontWeight.bold,
          );
          final bool fits = _fitsReconSingleLine(
            context: context,
            maxWidth: constraints.maxWidth,
            title: 'Ringkasan Pesanan',
            badge: 'Lelang',
            titleStyle: titleStyle,
            badgeStyle: chipStyle,
            fixedExtrasWidth: 8 + 16 + 16 + 3,
          );
          Widget chip() => Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
            decoration: BoxDecoration(
              color: context.statusColors.success.withValues(alpha: 0.15),
              borderRadius: BorderRadius.circular(4),
              border: Border.all(
                color: context.statusColors.success.withValues(alpha: 0.4),
              ),
            ),
            child: Text(
              'Lelang',
              style: context.typeRoles.labelMicro.copyWith(
                fontWeight: FontWeight.bold,
                color: context.statusColors.success,
              ),
              softWrap: true,
            ),
          );
          if (fits) {
            return Row(
              children: [
                Text('Ringkasan Pesanan', style: titleStyle),
                const SizedBox(width: 8),
                chip(),
              ],
            );
          }
          return Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                'Ringkasan Pesanan',
                style: titleStyle,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
              const SizedBox(height: 4),
              chip(),
            ],
          );
        },
      ),
    );
  },
);

void main() {
  setUpAll(() async {
    await initializeDateFormatting('id_ID');
  });

  group('F5 refund headers (all statuses, full matrix)', () {
    for (final RefundStatus status in RefundStatus.values) {
      for (final double width in _widths) {
        for (final double scale in _scales) {
          testWidgets(
            '${status.displayName} at ${width}dp @scale $scale',
            (tester) async {
              await _pump(
                tester,
                width: width,
                scale: scale,
                subject: RefundStatusCard(refund: _refund(status)),
              );
              _expectClean(
                tester,
                label: 'refund ${status.displayName}',
                width: width,
                scale: scale,
              );
              final Rect card = tester.getRect(find.byType(RefundStatusCard));
              _expectInside(
                tester,
                text: 'Permintaan Pengembalian',
                bounds: card,
              );
              _expectInside(tester, text: status.displayName, bounds: card);
              _expectBadgeIntact(tester, status.displayName);
            },
          );
        }
      }
    }
  });

  group('F5 payment header (status variants, full matrix)', () {
    for (final PaymentStatus? status in <PaymentStatus?>[
      null,
      PaymentStatus.paid,
      PaymentStatus.refunded,
    ]) {
      for (final double width in _widths) {
        for (final double scale in _scales) {
          testWidgets(
            'payment ${status?.name ?? "no-status"} at ${width}dp @scale $scale',
            (tester) async {
              await _pump(
                tester,
                width: width,
                scale: scale,
                subject: OrderPaymentInfoCard(
                  order: _order(paymentStatus: status),
                ),
              );
              _expectClean(
                tester,
                label: 'payment header ${status?.name}',
                width: width,
                scale: scale,
              );
              final Rect card = tester.getRect(
                find.byType(OrderPaymentInfoCard),
              );
              _expectInside(tester, text: 'Info Pembayaran', bounds: card);
              if (status == PaymentStatus.paid) {
                _expectInside(tester, text: 'LUNAS', bounds: card);
                _expectBadgeIntact(tester, 'LUNAS');
              }
              if (status == PaymentStatus.refunded) {
                // NOTE: production copy carries a typo ('DIKEMBALALIKAN');
                // asserted verbatim — copy changes are out of F5 scope.
                _expectInside(tester, text: 'DIKEMBALALIKAN', bounds: card);
                _expectBadgeIntact(tester, 'DIKEMBALALIKAN');
              }
            },
          );
        }
      }
    }
  });

  group('F5 overdue deadline row (adaptive)', () {
    for (final double width in _widths) {
      for (final double scale in _scales) {
        testWidgets('overdue 45d at ${width}dp @scale $scale', (tester) async {
          await _pump(
            tester,
            width: width,
            scale: scale,
            subject: OrderOverdueInfoCard(order: _overdueOrder()),
          );
          _expectClean(
            tester,
            label: 'overdue deadline',
            width: width,
            scale: scale,
          );
          final Rect card = tester.getRect(find.byType(OrderOverdueInfoCard));
          _expectInside(
            tester,
            text: 'Target siap kirim: 02/04/2026',
            bounds: card,
          );
          _expectInside(tester, text: 'Telat 45 hari', bounds: card);
          _expectBadgeIntact(tester, 'Telat 45 hari');
        });
      }
    }

    testWidgets('vertical fallback occurs when constrained (320dp @2.0)', (
      tester,
    ) async {
      await _pump(
        tester,
        width: 320,
        scale: 2.0,
        subject: OrderOverdueInfoCard(order: _overdueOrder()),
      );
      _expectClean(tester, label: 'overdue stack', width: 320, scale: 2.0);
      final Rect date = _rectOf(tester, 'Target siap kirim: 02/04/2026');
      final Rect late = _rectOf(tester, 'Telat 45 hari');
      expect(
        late.top,
        greaterThanOrEqualTo(date.bottom - 0.5),
        reason: 'late count must recompose below the deadline ($date vs $late)',
      );
    });

    testWidgets('horizontal preserved when it fits (800dp @1.0)', (
      tester,
    ) async {
      await _pump(
        tester,
        width: 800,
        scale: 1.0,
        subject: OrderOverdueInfoCard(order: _overdueOrder()),
      );
      _expectClean(tester, label: 'overdue row', width: 800, scale: 1.0);
      final Rect date = _rectOf(tester, 'Target siap kirim: 02/04/2026');
      final Rect late = _rectOf(tester, 'Telat 45 hari');
      expect(
        (date.top - late.top).abs(),
        lessThanOrEqualTo(2.0),
        reason: 'deadline and late count should share one line ($date vs $late)',
      );
    });
  });

  group('F5 phone verified rows (containment scoped to the F5 row)', () {
    // The section also carries non-F5 bare-title rows (pinned as residue
    // below), so the layout exception is drained and the F5 row itself is
    // proven contained + intact.
    const Set<String> allowedOutside = <String>{
      'Informasi Kontak & Identitas',
      'Contact Information',
    };
    for (final bool verified in <bool>[true, false]) {
      for (final double width in _widths) {
        for (final double scale in _scales) {
          testWidgets(
            'phone verified=$verified at ${width}dp @scale $scale',
            (tester) async {
              final TextEditingController controller =
                  TextEditingController(text: '081234567890');
              addTearDown(controller.dispose);
              await _pump(
                tester,
                width: width,
                scale: scale,
                subject: PersonalInformationSection(
                  onSelectDateOfBirth: () {},
                  email: 'pembeli@nusantara.id',
                  emailVerified: true,
                  phoneController: controller,
                  phoneVerified: verified,
                  onVerifyPhone: () {},
                ),
              );
              tester.takeException();
              final Rect card = tester.getRect(
                find.byType(PersonalInformationSection),
              );
              _expectInside(tester, text: 'Phone Number', bounds: card);
              final String badge = verified ? 'Verified' : 'Unverified';
              // The badge string also appears on the email row; scope to the
              // phone row by vertical proximity to the phone title.
              final Rect title = _rectOf(tester, 'Phone Number');
              final List<Rect> badges = find
                  .text(badge)
                  .evaluate()
                  .map((Element el) {
                    final RenderBox box = el.renderObject! as RenderBox;
                    final Offset at = box.localToGlobal(Offset.zero);
                    return at & box.size;
                  })
                  .toList();
              final Rect phoneBadge = badges.reduce(
                (Rect a, Rect b) =>
                    (a.center.dy - title.center.dy).abs() <
                        (b.center.dy - title.center.dy).abs()
                    ? a
                    : b,
              );
              expect(
                phoneBadge.left >= card.left - 0.5 &&
                    phoneBadge.right <= card.right + 0.5,
                isTrue,
                reason: 'phone badge "$badge" left the card '
                    '($phoneBadge vs $card)',
              );
              for (final Element el in find.byType(Text).evaluate()) {
                final String? data = (el.widget as Text).data;
                if (data == null || data.isEmpty) continue;
                final RenderBox box = el.renderObject! as RenderBox;
                if (!box.hasSize) continue;
                final Offset at = box.localToGlobal(Offset.zero);
                final Rect rect = at & box.size;
                final bool outside =
                    rect.left < card.left - 0.5 ||
                    rect.right > card.right + 0.5;
                if (outside) {
                  expect(
                    allowedOutside.contains(data),
                    isTrue,
                    reason: 'unexpected overflowing text "$data" '
                        '($rect vs $card)',
                  );
                }
              }
            },
          );
        }
      }
    }

    testWidgets(
      'NON-F5 RESIDUE PIN: bare section titles still overflow at 320dp @2.0',
      (tester) async {
        final TextEditingController controller = TextEditingController(
          text: '081234567890',
        );
        addTearDown(controller.dispose);
        await _pump(
          tester,
          width: 320,
          scale: 2.0,
          subject: PersonalInformationSection(
            onSelectDateOfBirth: () {},
            email: 'pembeli@nusantara.id',
            emailVerified: true,
            phoneController: controller,
            phoneVerified: true,
            onVerifyPhone: () {},
          ),
        );
        tester.takeException();
        final Rect card = tester.getRect(
          find.byType(PersonalInformationSection),
        );
        final Rect title = _rectOf(tester, 'Contact Information');
        expect(
          title.right,
          greaterThan(card.right + 0.5),
          reason: 'residue pin: this bare-title row has no trailing element '
              'and is outside F5 scope. If fixed, remove this pin.',
        );
      },
    );
  });

  group('F5 fixed-pattern rows (seller / feed / checkout)', () {
    for (final double width in _widths) {
      for (final double scale in _scales) {
        testWidgets('seller status at ${width}dp @scale $scale', (
          tester,
        ) async {
          await _pump(
            tester,
            width: width,
            scale: scale,
            subject: _sellerStatusRow(),
          );
          _expectClean(
            tester,
            label: 'seller status',
            width: width,
            scale: scale,
          );
          expect(find.text('Bukan Penjual'), findsOneWidget);
        });
        testWidgets('feed promo at ${width}dp @scale $scale', (tester) async {
          await _pump(
            tester,
            width: width,
            scale: scale,
            subject: _feedPromoRow(),
          );
          _expectClean(tester, label: 'feed promo', width: width, scale: scale);
          expect(find.text('45h tersisa'), findsOneWidget);
        });
        testWidgets('checkout title at ${width}dp @scale $scale', (
          tester,
        ) async {
          await _pump(
            tester,
            width: width,
            scale: scale,
            subject: _checkoutTitleRow(),
          );
          _expectClean(
            tester,
            label: 'checkout title',
            width: width,
            scale: scale,
          );
          expect(find.text('Lelang'), findsOneWidget);
        });
      }
    }

    testWidgets('recon rows stack when constrained (320dp @2.0)', (
      tester,
    ) async {
      await _pump(
        tester,
        width: 320,
        scale: 2.0,
        subject: _sellerStatusRow(),
      );
      _expectClean(tester, label: 'seller stack', width: 320, scale: 2.0);
      expect(
        _rectOf(tester, 'Bukan Penjual').top,
        greaterThanOrEqualTo(_rectOf(tester, 'Status Penjual').bottom - 0.5),
      );
      await _pump(
        tester,
        width: 320,
        scale: 2.0,
        subject: _feedPromoRow(),
      );
      _expectClean(tester, label: 'feed stack', width: 320, scale: 2.0);
      expect(
        _rectOf(tester, '45h tersisa').top,
        greaterThanOrEqualTo(_rectOf(tester, 'Dipromosikan').bottom - 0.5),
      );
    });
  });

  group('F5 call-site + negative pins', () {
    String readLib(String relative) => File('lib/$relative').readAsStringSync();

    test('refund + payment headers are adaptive (fit-measure present)', () {
      for (final String path in <String>[
        'domains/commerce/transaction/order/presentation/widgets/order_payment_info_card.dart',
        'domains/commerce/transaction/order/presentation/widgets/order_refund_status_card.dart',
        'domains/commerce/transaction/order/presentation/screens/order_detail/order_refund_list_section.dart',
        'domains/commerce/transaction/order/presentation/widgets/order_overdue_cards.dart',
      ]) {
        final String source = readLib(path);
        expect(
          source.contains('_fitsOrderLabelValueSingleLine') ||
              source.contains('_fitsTitleBadgeSingleLine'),
          isTrue,
          reason: '$path must measure the title+badge fit',
        );
      }
      // Titles are compressible chrome: stacked fallbacks cap them.
      for (final String path in <String>[
        'domains/commerce/transaction/order/presentation/widgets/order_payment_info_card.dart',
        'domains/commerce/transaction/order/presentation/widgets/order_refund_status_card.dart',
        'domains/commerce/transaction/order/presentation/screens/order_detail/order_refund_list_section.dart',
      ]) {
        expect(
          readLib(path).contains('maxLines: 1'),
          isTrue,
          reason: '$path stacked title must stay single-line',
        );
      }
    });

    test('profile/checkout/feed headers are adaptive (fit-measure present)', () {
      for (final String path in <String>[
        'domains/user/profile/presentation/widgets/personal_information_section.dart',
        'domains/user/profile/presentation/widgets/personal_info/phone_verification_field.dart',
        'domains/user/profile/presentation/screens/profile_screen/profile_about_tab.dart',
        'domains/commerce/transaction/checkout/presentation/widgets/checkout_order_summary_section.dart',
        'features/home/presentation/providers/feed_renderers.dart',
      ]) {
        final String source = readLib(path);
        expect(
          source.contains('_fitsTitleBadgeSingleLine'),
          isTrue,
          reason: '$path must measure the title+badge fit',
        );
      }
    });

    test('no prohibited workaround in touched F5 files', () {
      const List<String> paths = <String>[
        'domains/commerce/transaction/order/presentation/widgets/order_payment_info_card.dart',
        'domains/commerce/transaction/order/presentation/widgets/order_refund_status_card.dart',
        'domains/commerce/transaction/order/presentation/screens/order_detail/order_refund_list_section.dart',
        'domains/commerce/transaction/order/presentation/widgets/order_overdue_cards.dart',
        'domains/user/profile/presentation/widgets/personal_information_section.dart',
        'domains/user/profile/presentation/widgets/personal_info/phone_verification_field.dart',
        'domains/user/profile/presentation/screens/profile_screen/profile_about_tab.dart',
        'domains/commerce/transaction/checkout/presentation/widgets/checkout_order_summary_section.dart',
        'features/home/presentation/providers/feed_renderers.dart',
      ];
      for (final String path in paths) {
        final String source = readLib(path);
        expect(
          source.contains('IntrinsicWidth') ||
              source.contains('IntrinsicHeight') ||
              source.contains('FittedBox('),
          isFalse,
          reason: '$path must not carry an overflow workaround',
        );
        expect(
          source.contains('MediaQuery.of(context).size.width') ||
              source.contains('MediaQuery.sizeOf(context).width'),
          isFalse,
          reason: '$path must not use viewport width arithmetic',
        );
      }
    });

    test('no new generic header widget was introduced', () {
      final String lib = readLib(
        'domains/commerce/transaction/order/presentation/widgets/order_widgets_impl.dart',
      );
      expect(lib, isNot(contains('AdaptiveHeader')));
      expect(lib, isNot(contains('ResponsiveHeader')));
      expect(lib, isNot(contains('OverflowHeader')));
    });
  });
}
