// HORIZONTAL COMPOSITION — ACCEPTANCE GATE.
//
// Contract: lib/shared/widgets/docs/horizontal_composition_contract.md
//
// The contract is a RULE over existing Flutter primitives, not a widget:
//   * bounded/fixed leading & trailing content stays non-flexible;
//   * dynamic textual/contentual content receives the remaining width through
//     Expanded/Flexible;
//   * dynamic text declares an explicit text strategy (ellipsis / wrap /
//     Wrap / bounded FittedBox).
//
// This suite proves the contract by BEHAVIOUR, modeled after the marketplace
// card layout authority test: representative canonical compositions are pumped
// across a WIDTH x TEXT-SCALE matrix with deliberately long dynamic content,
// and must produce NO Flutter layout exception and stay within the surface.
//
// It deliberately does NOT classify every Row in lib and does NOT assert
// "every Row must contain Expanded". A Row of bounded children (Icon + short
// label, Text + Spacer + bounded trailing, bounded badges) is canonical as-is.
//
// A single planted negative control proves the detector has teeth: an
// unconstrained two-dynamic-end Row IS caught. The negative widget is isolated
// inside this test and never enters production.
//
// NotificationItemWidget was found by this gate overflowing at 320dp x 1.3
// (its timestamp Row). It has since been converged to the contract and is now
// a passing canonical subject here; its focused regression gate lives in
// test/domains/system/notification/notification_item_widget_layout_test.dart.
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart' hide NotificationEntity;
import 'package:hishumi/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_primitives.dart';
import 'package:hishumi/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';
import 'package:hishumi/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:hishumi/domains/system/notification/presentation/widgets/notification_item_widget.dart';
import 'package:hishumi/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:hishumi/domains/user/profile/data/profile_providers.dart'
    show avatarCacheServiceProvider;
import 'package:hishumi/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:hishumi/shared/models/seller_identity_data.dart';
import 'package:hishumi/shared/shared.dart';

// ============================================================================
// Matrix
// ============================================================================

const List<double> _widths = <double>[320, 360, 412, 500];
const List<double> _scales = <double>[1.0, 1.3, 2.0];

/// Long, dynamic, realistic worst-case content.
const String _longStoreName =
    'Koi Farm Nusantara Jaya Sentosa Premium Koi Collection Kolam Empat Musim';
const String _longUsername = 'verylongusername_koi_master_indonesia';
const String _longChipLabel =
    'Kohaku Gin Rin Kanoko Premium Super Jumbo 48cm';
const String _longCardTitle =
    'Sankei Kohaku Gin Rin Kanoko Premium Kolam Empat Musim 48cm Sertifikat';
const String _longBadge = 'Sertifikat + Vaksin Lengkap';
const String _longLabel = 'Metode Pembayaran Yang Dipilih Pembeli';
const String _longValue =
    'Transfer Bank Virtual Account BCA — dikonfirmasi otomatis oleh sistem';

// ============================================================================
// Identity provider harness (mirrors seller_identity_view_test)
// ============================================================================

const String _trackedUserId = '123e4567-e89b-12d3-a456-426614174001';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);
  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _NoOpDatasource extends Fake implements UserApiDatasource {}

class _NoOpAvatarCacheService extends AvatarCacheService {
  _NoOpAvatarCacheService() : super(datasource: _NoOpDatasource());

  @override
  Future<String?> getUserAvatarUrl(String userId) async => null;
}

AuthUser _authUser(String id, String username) {
  return AuthUser(
    id: id,
    createdAt: DateTime(2025),
    updatedAt: DateTime(2025),
    email: '$username@test.com',
    username: username,
    isEmailVerified: true,
    roles: const <UserRole>[UserRole.user],
    provider: AuthProvider.email,
  );
}

/// A bounded horizontal slot + vertical scroll (so a vertical overflow can
/// never masquerade as a horizontal one) under a fixed surface and text scale.
Widget _harness({
  required Size surface,
  required double scale,
  required Widget subject,
}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(
        () => _FakeAuthController(
          AuthState.authenticated(
            _authUser('viewer-1', 'viewer'),
            emailVerified: true,
          ),
        ),
      ),
      avatarCacheServiceProvider.overrideWith(
        (_) => _NoOpAvatarCacheService(),
      ),
      userOnlineStatusProvider(
        _trackedUserId,
      ).overrideWith((ref) => Stream<bool>.value(false)),
    ],
    child: MaterialApp(
      theme: AppTheme.lightTheme,
      home: MediaQuery(
        data: MediaQueryData(
          size: surface,
          textScaler: TextScaler.linear(scale),
        ),
        child: Scaffold(
          body: Center(
            child: SizedBox(
              width: surface.width - 24,
              child: SingleChildScrollView(child: subject),
            ),
          ),
        ),
      ),
    ),
  );
}

Future<void> _expectNoOverflow(
  WidgetTester tester, {
  required Size surface,
  required double scale,
  required Widget subject,
  required Type type,
}) async {
  await tester.binding.setSurfaceSize(surface);
  addTearDown(() => tester.binding.setSurfaceSize(null));

  await tester.pumpWidget(
    _harness(surface: surface, scale: scale, subject: subject),
  );
  await tester.pump();

  final Object? exception = tester.takeException();
  expect(
    exception,
    isNull,
    reason:
        'horizontal overflow: $type at ${surface.width}dp @scale $scale '
        '($exception)',
  );

  final Finder finder = find.byType(type);
  expect(finder, findsOneWidget, reason: '$type did not render');
  final Rect rect = tester.getRect(finder);
  expect(rect.left, greaterThanOrEqualTo(-0.5), reason: '$type left of surface');
  expect(
    rect.right,
    lessThanOrEqualTo(surface.width + 0.5),
    reason: '$type wider than surface at ${surface.width}dp @scale $scale',
  );
}

// ============================================================================
// Canonical subjects
// ============================================================================

Widget _sellerIdentity() => const SizedBox(
  child: SellerIdentityView(
    identity: SellerIdentityData(
      userId: _trackedUserId,
      username: _longUsername,
      storeName: _longStoreName,
      avatarUrl: 'https://example.com/avatar.jpg',
      storeImageUrl: 'https://example.com/store.jpg',
      isSeller: true,
    ),
    size: 48,
  ),
);

Widget _userHeader() => UserHeaderWidget(
  userId: _trackedUserId,
  name: _longStoreName,
  username: _longUsername,
  createdAt: DateTime.now(),
  size: UserHeaderSize.medium,
);

Widget _detailChip() => const DetailChipWidget(
  icon: Icons.straighten,
  label: _longChipLabel,
  color: Colors.blue,
);

Widget _notificationItem() => NotificationItemWidget(
  notification: NotificationEntity(
    id: 'n-1',
    userId: 'u-1',
    type: NotificationType.orderCreated,
    title: 'Pesanan baru $_longStoreName menunggu konfirmasi penjual segera',
    body: 'Pembeli telah membuat pesanan $_longValue dan menunggu konfirmasi.',
    isRead: false,
    createdAt: DateTime.now().subtract(const Duration(minutes: 59)),
  ),
  onTap: () {},
);

Widget _marketplaceCard() => CommerceMarketplaceCardShell(
  media: const CommerceMarketplaceCardMedia(
    imageUrl: null,
    fallback: SizedBox.shrink(),
  ),
  title: _longCardTitle,
  value: const CommerceMarketplaceCardValue(value: 'Rp 1.500.000.000'),
  badges: const <Widget>[
    CommerceMarketplaceCardBadge(label: _longBadge),
    CommerceMarketplaceCardBadge(label: _longBadge),
    CommerceMarketplaceCardBadge(label: 'Dipromosikan'),
  ],
);

Widget _labelValueHorizontal() => const CommerceDetailLabelValue(
  label: _longLabel,
  value: _longValue,
  layout: CommerceDetailValueLayout.horizontal,
);

Widget _labelValueAuto() => const CommerceDetailLabelValue(
  label: _longLabel,
  value: _longValue,
  layout: CommerceDetailValueLayout.auto,
);

// ============================================================================
// Tests
// ============================================================================

void main() {
  group('HORIZONTAL COMPOSITION — canonical compositions survive the matrix', () {
    testWidgets('SellerIdentityView: fixed avatar + dynamic textual body', (
      tester,
    ) async {
      for (final double width in _widths) {
        for (final double scale in _scales) {
          await _expectNoOverflow(
            tester,
            surface: Size(width, 640),
            scale: scale,
            subject: _sellerIdentity(),
            type: SellerIdentityView,
          );
        }
      }
    });

    testWidgets('UserHeaderWidget: fixed avatar + dynamic textual body', (
      tester,
    ) async {
      for (final double width in _widths) {
        for (final double scale in _scales) {
          await _expectNoOverflow(
            tester,
            surface: Size(width, 640),
            scale: scale,
            subject: _userHeader(),
            type: UserHeaderWidget,
          );
        }
      }
    });

    testWidgets('DetailChipWidget: icon + dynamic text', (tester) async {
      for (final double width in _widths) {
        for (final double scale in _scales) {
          await _expectNoOverflow(
            tester,
            surface: Size(width, 640),
            scale: scale,
            subject: _detailChip(),
            type: DetailChipWidget,
          );
        }
      }
    });

    testWidgets('NotificationItemWidget: fixed icon + dynamic body/trailing', (
      tester,
    ) async {
      for (final double width in _widths) {
        for (final double scale in _scales) {
          await _expectNoOverflow(
            tester,
            surface: Size(width, 640),
            scale: scale,
            subject: _notificationItem(),
            type: NotificationItemWidget,
          );
        }
      }
    });

    testWidgets(
      'CommerceMarketplaceCardShell: dynamic title + variable badges (Wrap)',
      (tester) async {
        for (final double width in _widths) {
          for (final double scale in _scales) {
            await _expectNoOverflow(
              tester,
              surface: Size(width, 640),
              scale: scale,
              subject: _marketplaceCard(),
              type: CommerceMarketplaceCardShell,
            );
          }
        }
      },
    );

    testWidgets(
      'CommerceDetailLabelValue: existing label/value (horizontal + auto)',
      (tester) async {
        for (final double width in _widths) {
          for (final double scale in _scales) {
            await _expectNoOverflow(
              tester,
              surface: Size(width, 640),
              scale: scale,
              subject: _labelValueHorizontal(),
              type: CommerceDetailLabelValue,
            );
            await _expectNoOverflow(
              tester,
              surface: Size(width, 640),
              scale: scale,
              subject: _labelValueAuto(),
              type: CommerceDetailLabelValue,
            );
          }
        }
      },
    );
  });

  // ==========================================================================
  // TEST TEETH — the planted negative control
  // ==========================================================================
  testWidgets(
    'the gate has teeth: an unconstrained two-dynamic-end Row IS caught',
    (tester) async {
      const Size surface = Size(320, 320);
      await tester.binding.setSurfaceSize(surface);
      addTearDown(() => tester.binding.setSurfaceSize(null));

      // Deliberately invalid: two unbounded dynamic Texts, no Expanded/Flexible,
      // inside the bounded slot. Isolated to the test — never production.
      await tester.pumpWidget(
        _harness(
          surface: surface,
          scale: 1.0,
          subject: const Row(
            children: <Widget>[
              Text('Sankei Kohaku Gin Rin Kanoko Premium Kolam Empat Musim 48cm'),
              Text('Rp 1.500.000.000 total pembayaran segera dikonfirmasi'),
            ],
          ),
        ),
      );
      await tester.pump();

      expect(
        tester.takeException(),
        isNotNull,
        reason:
            'an unconstrained two-dynamic-end Row must be caught by the gate — '
            'otherwise the canonical cases would only be passing by accident',
      );
    },
  );
}
