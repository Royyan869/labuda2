// ============================================================================
// FEED REPOSITORY LIFETIME AND CURSOR AUTHORITY PROOF
//
// These tests prove that the production provider graph correctly retains
// HomeRepositoryImpl and its _nextCursor for the FeedNotifier's lifecycle.
//
// No test-only provider pins (container.listen, keepAlive, etc.) are used.
// ============================================================================

import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/features/home/home.dart';
import 'package:labuda/shared/services/logger_service.dart';

// ============================================================================
// Canonical HTTP fixture builder (same envelope as cross-boundary tests)
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

Map<String, dynamic> _contentItem({
  required String id,
  required String body,
}) {
  return <String, dynamic>{
    'feed_item_kind': 'content',
    'id': id,
    'status': 'active',
    'body': body,
    'created_at': '2026-08-05T10:00:00Z',
    'updated_at': '2026-08-05T10:00:00Z',
    'author': <String, dynamic>{
      'id': 'author-1',
      'username': 'alice',
      'avatar_url': 'https://example.com/avatar.jpg',
      'lifecycle': 'active',
    },
    'media': <Map<String, dynamic>>[],
  };
}

// ============================================================================
// Fake HTTP adapter — records query parameters and returns canned responses
// ============================================================================

class _CannedResponse {
  final int statusCode;
  final Map<String, dynamic>? body;
  const _CannedResponse({required this.statusCode, this.body});
}

class _RecordingAdapter implements HttpClientAdapter {
  final List<_CannedResponse> _responses;
  int _callCount = 0;
  final List<Map<String, dynamic>> capturedParams = [];

  _RecordingAdapter(List<_CannedResponse> responses) : _responses = responses;

  ResponseBody _genericSuccess() {
    return ResponseBody.fromString(
      jsonEncode(<String, dynamic>{
        'success': true,
        'data': <String, dynamic>{},
        'timestamp': '2026-08-05T00:00:00Z',
      }),
      200,
      headers: {'content-type': ['application/json']},
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

    capturedParams.add(Map<String, dynamic>.from(options.queryParameters));

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
      headers: {'content-type': ['application/json']},
    );
  }

  @override
  void close({bool force = false}) {}

  int get requestCount => _callCount;
}

ApiClient _fakeApiClient(_RecordingAdapter adapter) {
  final client = ApiClient.testing();
  client.dio.httpClientAdapter = adapter;
  return client;
}

// ============================================================================
// Fake auth
// ============================================================================

class _FakeAuthController extends AuthController {
  @override
  AuthState build() => const AuthStateUnauthenticated();
}

// ============================================================================
// Build ProviderContainer with real feed pipeline and fake transport
// ============================================================================

ProviderContainer _buildContainer(_RecordingAdapter adapter) {
  final container = ProviderContainer(
    overrides: [
      apiClientProvider.overrideWithValue(_fakeApiClient(adapter)),
      authControllerProvider.overrideWith(_FakeAuthController.new),
      loggerServiceProvider.overrideWithValue(LoggerService.instance),
    ],
  );
  addTearDown(container.dispose);
  return container;
}

/// Process microtasks so the Future.microtask(loadFeed) chain resolves.
/// Dio with a synchronous fake adapter still needs several microtask
/// cycles to complete the full async chain (ApiClient.get → dio.get →
/// adapter.fetch → response parsing → datasource → repository → notifier).
Future<void> _settleMicrotasks() async {
  for (int i = 0; i < 8; i++) {
    await Future<void>.delayed(Duration.zero);
  }
}

// ============================================================================
// Tests
// ============================================================================

void main() {
  // ==========================================================================
  // 1. Cursor survives provider idle period
  // ==========================================================================
  group('cursor survival', () {
    test('_nextCursor retained between loadFeed and loadMore', () async {
      final adapter = _RecordingAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_contentItem(id: 'a-1', body: 'Content A')],
            nextCursor: 'cursor-X',
            hasMore: true,
          ),
        ),
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_contentItem(id: 'b-1', body: 'Content B')],
            hasMore: false,
          ),
        ),
      ]);

      final container = _buildContainer(adapter);

      // Access feedProvider — this creates the FeedNotifier, which in
      // build() calls Future.microtask(loadFeed). The initial load sets
      // _nextCursor = 'cursor-X' inside HomeRepositoryImpl.
      container.listen(feedProvider, (_, __) {});
      await _settleMicrotasks();

      // Verify initial load sent no cursor.
      expect(adapter.capturedParams.length, 1);
      expect(adapter.capturedParams[0]['cursor'], isNull);
      expect(adapter.capturedParams[0]['limit'], 20);

      final stateAfterLoad = container.read(feedProvider);
      expect(stateAfterLoad.items, hasLength(1));
      expect(stateAfterLoad.items[0].id, 'a-1');
      expect(stateAfterLoad.hasReachedMax, isFalse);

      // Allow the provider container to settle. In production, this is the
      // gap between user scrolls. homeRepositoryProvider must survive this.
      await _settleMicrotasks();

      // Trigger loadMore. Must use cursor-X from the retained repository.
      await container.read(feedProvider.notifier).loadMore();
      await _settleMicrotasks();

      // The loadMore request MUST contain cursor-X.
      expect(adapter.capturedParams.length, 2);
      expect(adapter.capturedParams[1]['cursor'], 'cursor-X');
      expect(adapter.capturedParams[1]['limit'], 20);

      // Items appended.
      final stateAfterMore = container.read(feedProvider);
      expect(stateAfterMore.items, hasLength(2));
      expect(stateAfterMore.items[0].id, 'a-1');
      expect(stateAfterMore.items[1].id, 'b-1');
    });
  });

  // ==========================================================================
  // 2. No first-page duplication
  // ==========================================================================
  group('no page duplication', () {
    test('loadMore appends unique items, does not replay page 1', () async {
      final adapter = _RecordingAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_contentItem(id: 'a-1', body: 'Content A')],
            nextCursor: 'cursor-X',
            hasMore: true,
          ),
        ),
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_contentItem(id: 'b-1', body: 'Content B')],
            nextCursor: 'cursor-Y',
            hasMore: false,
          ),
        ),
      ]);

      final container = _buildContainer(adapter);
      container.listen(feedProvider, (_, __) {});
      await _settleMicrotasks();

      await container.read(feedProvider.notifier).loadMore();
      await _settleMicrotasks();

      final state = container.read(feedProvider);
      // Must have exactly 2 items: [A, B], no duplicates.
      expect(state.items, hasLength(2));
      expect(state.items.map((e) => e.id).toList(), ['a-1', 'b-1']);
      expect(state.hasReachedMax, isTrue);

      // Page 2 request used cursor from page 1.
      expect(adapter.capturedParams[1]['cursor'], 'cursor-X');
      expect(adapter.capturedParams[1]['limit'], 20);
    });
  });

  // ==========================================================================
  // 3. Refresh resets cursor intentionally
  // ==========================================================================
  group('refresh cursor reset', () {
    test('refresh sends no cursor, next loadMore uses new cursor', () async {
      final adapter = _RecordingAdapter([
        // Page 1.
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_contentItem(id: 'a-1', body: 'Page 1')],
            nextCursor: 'cursor-X',
            hasMore: true,
          ),
        ),
        // Refresh request (no cursor → fresh page 1).
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_contentItem(id: 'b-1', body: 'Refreshed')],
            nextCursor: 'cursor-Y',
            hasMore: true,
          ),
        ),
        // loadMore after refresh — uses cursor-Y.
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_contentItem(id: 'c-1', body: 'Page 2')],
            hasMore: false,
          ),
        ),
      ]);

      final container = _buildContainer(adapter);
      container.listen(feedProvider, (_, __) {});
      await _settleMicrotasks();

      // Initial load used no cursor.
      expect(adapter.capturedParams[0]['cursor'], isNull);

      // Refresh: must send no cursor (starts fresh).
      await container.read(feedProvider.notifier).refresh();
      await _settleMicrotasks();

      expect(adapter.capturedParams[1]['cursor'], isNull);
      final stateAfterRefresh = container.read(feedProvider);
      expect(stateAfterRefresh.items[0].id, 'b-1');

      // loadMore after refresh: must use cursor-Y.
      await container.read(feedProvider.notifier).loadMore();
      await _settleMicrotasks();

      expect(adapter.capturedParams[2]['cursor'], 'cursor-Y');
      final stateAfterMore = container.read(feedProvider);
      expect(stateAfterMore.items, hasLength(2));
    });
  });

  // ==========================================================================
  // 4. Feed invalidation resets lifecycle
  // ==========================================================================
  group('feed invalidation resets lifecycle', () {
    test('new feedProvider instance starts with no cursor', () async {
      final adapter = _RecordingAdapter([
        // First lifecycle: page 1 with cursor-X.
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_contentItem(id: 'a-1', body: 'Lifecycle 1')],
            nextCursor: 'cursor-X',
            hasMore: true,
          ),
        ),
        // Second lifecycle: fresh page 1 (no old cursor).
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_contentItem(id: 'b-1', body: 'Lifecycle 2')],
            nextCursor: 'cursor-Z',
            hasMore: true,
          ),
        ),
      ]);

      // First lifecycle.
      ProviderContainer container1 = _buildContainer(adapter);
      container1.listen(feedProvider, (_, __) {});
      await _settleMicrotasks();

      expect(adapter.capturedParams[0]['cursor'], isNull);
      final state1 = container1.read(feedProvider);
      expect(state1.items[0].id, 'a-1');

      // Dispose the first container — FeedNotifier and repository destroyed.
      container1.dispose();
      await _settleMicrotasks();

      // Second lifecycle: fresh container, fresh providers.
      // Override the api auth logger again since the adapter is shared.
      final container2 = ProviderContainer(
        overrides: [
          apiClientProvider.overrideWithValue(_fakeApiClient(adapter)),
          authControllerProvider.overrideWith(_FakeAuthController.new),
          loggerServiceProvider.overrideWithValue(LoggerService.instance),
        ],
      );
      addTearDown(container2.dispose);

      container2.listen(feedProvider, (_, __) {});
      await _settleMicrotasks();

      // Second lifecycle must NOT reuse cursor-X.
      expect(adapter.capturedParams[1]['cursor'], isNull);
      final state2 = container2.read(feedProvider);
      expect(state2.items[0].id, 'b-1');
    });
  });

  // ==========================================================================
  // 5. No test-only pin
  // ==========================================================================
  group('no test-only pin', () {
    test('cursor survives without container.listen on repository', () async {
      // This is the key test: we NEVER call
      // container.listen(homeRepositoryProvider, ...).
      // The production ref.watch in FeedNotifier.build() must keep
      // the repository alive.

      final adapter = _RecordingAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_contentItem(id: 'a-1', body: 'Item A')],
            nextCursor: 'cursor-X',
            hasMore: true,
          ),
        ),
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_contentItem(id: 'b-1', body: 'Item B')],
            hasMore: false,
          ),
        ),
      ]);

      final container = _buildContainer(adapter);
      container.listen(feedProvider, (_, __) {});
      await _settleMicrotasks();

      // Let providers settle — no manual pin on homeRepositoryProvider.
      await _settleMicrotasks();

      // loadMore must still use cursor-X.
      await container.read(feedProvider.notifier).loadMore();
      await _settleMicrotasks();

      expect(adapter.capturedParams[1]['cursor'], 'cursor-X');
    });
  });
}
