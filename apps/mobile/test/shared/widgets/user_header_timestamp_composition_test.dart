// USER HEADER TIMESTAMP — HORIZONTAL COMPOSITION ACCEPTANCE TEST.
//
// The canonical Indonesian relative progression (`baru saja`, …) is longer
// than the strings this header previously rendered, which exposed a latent
// RenderFlex overflow at 320dp x 2.0: the timestamp sat bare (no flex, no
// text strategy) in the identity inner Row. The fix bounds the timestamp as
// compact secondary metadata: parent-level Flexible + single line +
// ellipsis (Metadata compact semantics), leaving identity/name semantics,
// the formatter, and all other TimeAgoWidget consumers untouched.
//
// Matrix: 320/360/412/500 x 1.0/1.3/2.0 over the REAL UserHeaderWidget with
// long identity fixtures competing against canonical timestamp strings.
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/system/shared/domain/services/time_format_service.dart';
import 'package:hishumi/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:hishumi/domains/user/profile/data/profile_providers.dart'
    show avatarCacheServiceProvider;
import 'package:hishumi/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:hishumi/shared/shared.dart';

const List<double> _widths = <double>[320, 360, 412, 500];
const List<double> _scales = <double>[1.0, 1.3, 2.0];

const String _trackedUserId = '123e4567-e89b-12d3-a456-426614174001';
const String _longName =
    'Koi Farm Nusantara Jaya Sentosa Premium Koi Collection Kolam Empat Musim';
const String _longUsername = 'verylongusername_koi_master_indonesia';

const double _surfaceHeight = 720;

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

AuthUser _viewerUser() {
  return AuthUser(
    id: 'viewer-1',
    createdAt: DateTime(2025),
    updatedAt: DateTime(2025),
    email: 'viewer@test.com',
    username: 'viewer',
    isEmailVerified: true,
    roles: const <UserRole>[UserRole.user],
    provider: AuthProvider.email,
  );
}

Widget _harness({
  required Size surface,
  required double scale,
  required Widget subject,
}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(
        () => _FakeAuthController(
          AuthState.authenticated(_viewerUser(), emailVerified: true),
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
              height: _surfaceHeight,
              child: subject,
            ),
          ),
        ),
      ),
    ),
  );
}

Future<void> _pumpAt(
  WidgetTester tester,
  Size surface,
  Widget subject,
) async {
  await tester.binding.setSurfaceSize(surface);
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(
    _harness(surface: surface, scale: 1.0, subject: subject),
  );
  await tester.pump();
}

/// The timestamp Text rendered inside TimeAgoWidget.
Text _timestampText(WidgetTester tester, String expected) {
  final Finder text = find.descendant(
    of: find.byType(TimeAgoWidget),
    matching: find.byType(Text),
  );
  expect(text, findsOneWidget, reason: 'timestamp must be represented');
  final Text widget = tester.widget<Text>(text);
  expect(widget.data, expected);
  return widget;
}

void main() {
  group('UserHeaderWidget timestamp — bounded compact composition', () {
    testWidgets('no overflow with baru saja across the full matrix', (
      tester,
    ) async {
      for (final double width in _widths) {
        for (final double scale in _scales) {
          final Size surface = Size(width, _surfaceHeight);
          final DateTime createdAt = DateTime.now();
          await tester.binding.setSurfaceSize(surface);
          addTearDown(() => tester.binding.setSurfaceSize(null));
          await tester.pumpWidget(
            _harness(
              surface: surface,
              scale: scale,
              subject: UserHeaderWidget(
                userId: _trackedUserId,
                name: _longName,
                username: _longUsername,
                createdAt: createdAt,
              ),
            ),
          );
          await tester.pump();

          expect(
            tester.takeException(),
            isNull,
            reason: 'header overflow at ${width}dp @scale $scale',
          );

          final Rect rect = tester.getRect(find.byType(UserHeaderWidget));
          expect(rect.left, greaterThanOrEqualTo(-0.5));
          expect(
            rect.right,
            lessThanOrEqualTo(surface.width + 0.5),
            reason: 'header wider than surface at ${width}dp @scale $scale',
          );

          // Identity remains represented.
          expect(find.text(_longName), findsOneWidget);
          expect(find.text('@$_longUsername'), findsOneWidget);

          // Timestamp remains represented, single-line, ellipsis strategy.
          final String expected = const TimeFormatService().formatTimeAgo(
            createdAt,
          );
          expect(expected, 'baru saja');
          final Text timestamp = _timestampText(tester, expected);
          expect(timestamp.maxLines, 1);
          expect(timestamp.overflow, TextOverflow.ellipsis);
        }
      }
    });

    testWidgets('longer relative values compete without overflow', (
      tester,
    ) async {
      final DateTime createdAt = DateTime.now().subtract(
        const Duration(minutes: 59),
      );
      const expected = '59 menit lalu';

      for (final double width in _widths) {
        for (final double scale in _scales) {
          final Size surface = Size(width, _surfaceHeight);
          await tester.binding.setSurfaceSize(surface);
          addTearDown(() => tester.binding.setSurfaceSize(null));
          await tester.pumpWidget(
            _harness(
              surface: surface,
              scale: scale,
              subject: UserHeaderWidget(
                userId: _trackedUserId,
                name: _longName,
                username: _longUsername,
                createdAt: createdAt,
              ),
            ),
          );
          await tester.pump();

          expect(
            tester.takeException(),
            isNull,
            reason: 'header overflow at ${width}dp @scale $scale',
          );
          expect(find.text('@$_longUsername'), findsOneWidget);
          final Text timestamp = _timestampText(tester, expected);
          expect(timestamp.maxLines, 1);
          expect(timestamp.overflow, TextOverflow.ellipsis);

          // At the tightest cell the timestamp must genuinely truncate
          // rather than push the row out of bounds.
          if (width == 320 && scale == 2.0) {
            final RenderParagraph paragraph = tester.renderObject<RenderParagraph>(
              find.descendant(
                of: find.byType(TimeAgoWidget),
                matching: find.byType(RichText),
              ),
            );
            expect(
              paragraph.didExceedMaxLines,
              isTrue,
              reason: 'timestamp must truncate (not overflow) at 320dp @2.0',
            );
          }
        }
      }
    });

    testWidgets('other TimeAgoWidget consumers keep unbounded behavior', (
      tester,
    ) async {
      // profile_reviews_tab passes no maxLines: the widget must preserve
      // its legacy unbounded Text when the caller declares no strategy.
      await _pumpAt(
        tester,
        const Size(412, _surfaceHeight),
        TimeAgoWidget.compact(dateTime: _fixedCreatedAt),
      );
      final Text text = tester.widget<Text>(
        find.descendant(
          of: find.byType(TimeAgoWidget),
          matching: find.byType(Text),
        ),
      );
      expect(text.maxLines, isNull);
      expect(text.overflow, isNull);
    });
  });
}

final DateTime _fixedCreatedAt = DateTime(2026, 3, 10, 12, 0, 0);
