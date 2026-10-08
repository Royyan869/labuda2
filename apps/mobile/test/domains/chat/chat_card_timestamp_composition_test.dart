// CHAT LIST TIMESTAMP — HORIZONTAL COMPOSITION ACCEPTANCE TEST.
// Real ChatCard matrix proof for canonical relative timestamp authority.
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat_card.dart';
import 'package:labuda/domains/system/shared/domain/services/time_format_service.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show avatarCacheServiceProvider;
import 'package:labuda/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart'
    show currentUserIdProvider;
import 'package:labuda/shared/governance/content_lifecycle.dart';

const List<double> _widths = <double>[320, 360, 412, 500];
const List<double> _scales = <double>[1.0, 1.3, 2.0];
const String _currentUserId = 'aaaaaaaa-1111-1111-1111-111111111111';
const String _otherUserId = 'bbbbbbbb-2222-2222-2222-222222222222';
const String _longName =
    'Koi Farm Nusantara Jaya Sentosa Premium Collection Indonesia';
const double _surfaceHeight = 900;

final DateTime _now = DateTime.now();
final List<({DateTime updatedAt, String expected})> _fixtures = [
  (updatedAt: _now, expected: 'baru saja'),
  (
    updatedAt: _now.subtract(const Duration(minutes: 59)),
    expected: '59 menit lalu',
  ),
  (
    updatedAt: _now.subtract(const Duration(days: 360)),
    expected: '12 bulan lalu',
  ),
  (
    updatedAt: _now.subtract(const Duration(days: 1095)),
    expected: '3 tahun lalu',
  ),
];

class _FakeAuthController extends AuthController {
  @override
  AuthState build() => const AuthState.unauthenticated();
}

class _NoOpDatasource extends Fake implements UserApiDatasource {}

class _NoOpAvatarCacheService extends AvatarCacheService {
  _NoOpAvatarCacheService() : super(datasource: _NoOpDatasource());

  @override
  Future<String?> getUserAvatarUrl(String userId) async => null;
}

Chat _chat(DateTime updatedAt) => Chat(
  id: 'chat-1',
  participantIds: const <String>[_currentUserId, _otherUserId],
  participantNames: const <String, String>{_otherUserId: _longName},
  participantAvatars: const <String, String?>{_otherUserId: null},
  createdAt: _now.subtract(const Duration(days: 2)),
  updatedAt: updatedAt,
  lastMessage: null,
  participantLifecycles: const <String, ContentLifecycle>{
    _otherUserId: ContentLifecycle.active,
  },
);

Widget _harness({
  required Size surface,
  required double scale,
  required Chat chat,
}) {
  return ProviderScope(
    overrides: [
      currentUserIdProvider.overrideWith((ref) => _currentUserId),
      authControllerProvider.overrideWith(() => _FakeAuthController()),
      avatarCacheServiceProvider
          .overrideWith((ref) => _NoOpAvatarCacheService()),
    ],
    child: MaterialApp(
      theme: AppTheme.lightTheme,
      home: MediaQuery(
        data: MediaQueryData(
          size: surface,
          textScaler: TextScaler.linear(scale),
        ),
        child: Scaffold(
          body: SizedBox(
            width: surface.width,
            height: _surfaceHeight,
            child: SingleChildScrollView(
              child: ChatCard(chat: chat, onTap: () {}),
            ),
          ),
        ),
      ),
    ),
  );
}

bool _underFlexible(WidgetTester tester, Finder finder) => find
    .ancestor(of: finder, matching: find.byType(Flexible))
    .evaluate()
    .isNotEmpty;

void _expectInside(WidgetTester tester, Size surface, Finder finder) {
  final Rect card = tester.getRect(find.byType(ChatCard));
  final Rect timestamp = tester.getRect(finder);
  expect(timestamp.left, greaterThanOrEqualTo(card.left - 0.5));
  expect(timestamp.right, lessThanOrEqualTo(card.right + 0.5));
  expect(timestamp.right, lessThanOrEqualTo(surface.width + 0.5));
}

void main() {
  testWidgets('ChatCard timestamp survives full composition matrix', (
    tester,
  ) async {
    for (final fixture in _fixtures) {
      expect(
        const TimeFormatService().formatTimeAgo(fixture.updatedAt),
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
              chat: _chat(fixture.updatedAt),
            ),
          );
          await tester.pump();

          expect(
            tester.takeException(),
            isNull,
            reason:
                'ChatCard overflow at ${fixture.expected} ${width}dp x $scale',
          );
          final Finder timestampFinder = find.text(fixture.expected);
          expect(timestampFinder, findsOneWidget);
          expect(find.text('@$_longName'), findsOneWidget);

          final Text timestamp = tester.widget<Text>(timestampFinder);
          expect(timestamp.data, fixture.expected);
          expect(timestamp.maxLines, 1);
          expect(timestamp.overflow, TextOverflow.ellipsis);
          expect(_underFlexible(tester, timestampFinder), isTrue);
          _expectInside(tester, surface, timestampFinder);
        }
      }
    }
  });
}
