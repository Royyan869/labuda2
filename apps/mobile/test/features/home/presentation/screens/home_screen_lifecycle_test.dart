import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/auction.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/domain/repositories/like_repository.dart';
import 'package:labuda/domains/social/like/presentation/providers/like_notifier.dart';
import 'package:labuda/features/home/home.dart';
import 'package:labuda/shared/services/logger_service.dart';
import 'package:visibility_detector/visibility_detector.dart';

// ============================================================================
// Canonical HTTP fixture / fake transport
// Pipeline: _FakeFeedHttpAdapter → ApiClient → apiClientProvider
//           → FeedApiDatasource → FeedNotifier → HomeScreen
// ============================================================================

Map<String, dynamic> _feedEnvelope({
  required List<Map<String, dynamic>> items,
  String? nextCursor,
  bool hasMore = false,
}) {
  return <String, dynamic>{
    'data': <String, dynamic>{
      'data': items,
      'next_cursor': nextCursor,
      'has_more': hasMore,
    },
  };
}

class _CannedResponse {
  final int statusCode;
  final Map<String, dynamic>? body;
  const _CannedResponse({required this.statusCode, this.body});
}

class _FakeFeedHttpAdapter implements HttpClientAdapter {
  final List<_CannedResponse> _responses;
  int _callCount = 0;

  _FakeFeedHttpAdapter(List<_CannedResponse> responses)
    : _responses = responses;

  ResponseBody _genericSuccess() {
    return ResponseBody.fromString(
      jsonEncode(<String, dynamic>{
        'success': true,
        'data': <String, dynamic>{},
        'timestamp': '2026-08-05T00:00:00Z',
      }),
      200,
      headers: {
        'content-type': ['application/json'],
      },
    );
  }

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<List<int>>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (!options.path.contains('feed') && !options.path.contains('/feed')) {
      return _genericSuccess();
    }

    if (_callCount >= _responses.length) {
      final body = _feedEnvelope(
        items: <Map<String, dynamic>>[],
        hasMore: false,
      );
      return ResponseBody.fromString(jsonEncode(body), 200, headers: {
        'content-type': ['application/json'],
      });
    }

    final canned = _responses[_callCount];
    _callCount++;

    if (canned.body == null) {
      throw DioException(
        requestOptions: options,
        type: DioExceptionType.connectionError,
        message: 'Simulated network error',
      );
    }

    return ResponseBody.fromString(
      jsonEncode(canned.body),
      canned.statusCode,
      headers: {
        'content-type': ['application/json'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

ApiClient _fakeApiClient(_FakeFeedHttpAdapter adapter) {
  final client = ApiClient(logger: null);
  client.dio.httpClientAdapter = adapter;
  return client;
}

class _FakeAuthenticatedAuthController extends AuthController {
  @override
  AuthState build() => AuthState.authenticated(
    AuthUser(
      id: 'user-1',
      email: 'test@example.com',
      username: 'testuser',
      isEmailVerified: true,
      createdAt: DateTime.utc(2026, 1, 1),
      updatedAt: DateTime.utc(2026, 1, 1),
      roles: const [UserRole.user],
      provider: ShonaAuthProvider.email,
    ),
    emailVerified: true,
  );
}

class _FakeAuctionRepository implements AuctionRepository {
  @override
  Stream<List<Auction>> watchActiveAuctions({int limit = 50}) {
    return Stream.value(const <Auction>[]);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeLikeRepository implements LikeRepository {
  @override
  Future<Result<bool>> toggleLike({
    required String targetId,
    required LikeTargetType targetType,
    required String userId,
  }) async {
    return Result.success(false);
  }

  @override
  Future<Result<LikeStats>> getLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) async {
    return Result.success(
      LikeStats(
        targetId: targetId,
        targetType: targetType,
        totalLikes: 0,
        isLikedByCurrentUser: false,
      ),
    );
  }

  @override
  Future<Result<bool>> hasUserLiked({
    required String targetId,
    required LikeTargetType targetType,
    required String userId,
  }) async {
    return Result.success(false);
  }

  @override
  Stream<LikeStats> watchLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) {
    return Stream.value(
      LikeStats(
        targetId: targetId,
        targetType: targetType,
        totalLikes: 0,
        isLikedByCurrentUser: false,
      ),
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Future<void> _capturePrints(
  Future<void> Function() body,
  List<String> lines,
) async {
  await runZoned(
    body,
    zoneSpecification: ZoneSpecification(
      print: (self, parent, zone, line) {
        lines.add(line);
      },
    ),
  );
}

Widget _wrapHomeHarness(GoRouter router, _FakeFeedHttpAdapter adapter) {
  return ProviderScope(
    overrides: [
      apiClientProvider.overrideWithValue(_fakeApiClient(adapter)),
      authControllerProvider.overrideWith(
        _FakeAuthenticatedAuthController.new,
      ),
      auctionRepositoryProvider.overrideWithValue(_FakeAuctionRepository()),
      likeRepositoryProvider.overrideWithValue(_FakeLikeRepository()),
      loggerServiceProvider.overrideWithValue(LoggerService.instance),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

// NB: idle_nav_trace.dart was removed — idle-nav trace is disabled in tests.
const kIdleNavTraceEnabled = false;

void main() {
  setUp(() {
    VisibilityDetectorController.instance.updateInterval = Duration.zero;
  });

  testWidgets(
    'HomeScreen mounts and disposes without invalid lifecycle lookups',
    (tester) async {
      final lines = <String>[];
      final adapter = _FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(items: <Map<String, dynamic>>[], hasMore: false),
        ),
      ]);
      final router = GoRouter(
        initialLocation: '/home',
        routes: [
          GoRoute(
            path: '/home',
            builder: (context, state) => const Scaffold(body: HomeScreen()),
          ),
          GoRoute(
            path: '/other',
            builder: (context, state) =>
                const Scaffold(body: SizedBox.shrink()),
          ),
        ],
      );

      await _capturePrints(() async {
        await tester.pumpWidget(_wrapHomeHarness(router, adapter));
        await tester.pumpAndSettle();
        expect(tester.takeException(), isNull);

        router.go('/other');
        await tester.pumpAndSettle();
        expect(tester.takeException(), isNull);
      }, lines);

      if (kIdleNavTraceEnabled) {
        expect(
          lines.any(
            (line) =>
                line.contains('event=HOME_WIDGET_INIT') &&
                line.contains('route=unknown'),
          ),
          isTrue,
        );
        expect(
          lines.any(
            (line) =>
                line.contains('event=HOME_WIDGET_ROUTE_ATTACHED') &&
                line.contains('route=/home'),
          ),
          isTrue,
        );
        expect(
          lines.any(
            (line) =>
                line.contains('event=HOME_WIDGET_BUILD') &&
                line.contains('route=/home'),
          ),
          isTrue,
        );
        expect(
          lines.any(
            (line) =>
                line.contains('event=HOME_WIDGET_DISPOSE') &&
                line.contains('route=/home'),
          ),
          isTrue,
        );
      } else {
        expect(lines, isEmpty);
      }
    },
  );
}
