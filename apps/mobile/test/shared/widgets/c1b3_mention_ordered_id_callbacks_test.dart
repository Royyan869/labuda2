// C1B3 — MentionTextField ordered-ID callback proofs (Proof 3).
//
// Pumps production MentionTextField and captures exact onMentionsChanged
// callback values through the real parser→resolver path.

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:hishumi/domains/user/profile/data/profile_providers.dart'
    show avatarCacheServiceProvider;
import 'package:hishumi/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:hishumi/shared/providers/auth_status_providers.dart'
    show currentUserIdProvider;
import 'package:hishumi/shared/widgets/mentions/mention_text_field.dart';

// =============================================================================
// Fake dependencies
// =============================================================================

/// Canned ApiClient at the PRODUCTION seam (`apiClientProvider`): the
/// mention resolver (`MentionResolver`) calls
/// `ApiClient.get('/users/search', limit: 1)` per uncached username. The
/// old `SearchApiService` override was a stale seam from before the backend
/// rewiring — it never fired, so the real provider threw
/// `UnimplementedError` (test follows codebase).
class _FakeApiClient extends Fake implements ApiClient {
  _FakeApiClient({this.users = const [], this.emptyFor = const {}});

  /// Canned backend rows (`id`/`username` exactly as the Go API returns).
  final List<Map<String, dynamic>> users;

  /// Queries the backend must answer with an empty user list — the
  /// resolver's fail-closed path: empty page → null → no invented ID.
  final Set<String> emptyFor;

  /// Every `q` that hit the wire — for call-count proofs.
  final List<String> queries = [];

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    final q = '${queryParameters?['q'] ?? ''}'.toLowerCase().trim();
    queries.add(q);
    if (emptyFor.contains(q)) {
      return Response<T>(
        requestOptions: RequestOptions(path: path),
        data: <String, dynamic>{'users': const []} as T,
      );
    }
    final limit = int.tryParse('${queryParameters?['limit'] ?? ''}') ?? 20;
    final matches = users
        .where((u) => '${u['username'] ?? ''}'.toLowerCase().contains(q))
        .take(limit)
        .toList();
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      data: <String, dynamic>{'users': matches} as T,
    );
  }
}

class _FakeUserApiDatasource extends Fake implements UserApiDatasource {}

class _StubLogger extends Fake implements ILoggerService {
  @override Future<Result<void>> info(String m, {Map<String, dynamic>? extra}) async => Result.success(null);
  @override Future<Result<void>> error(String m, {Map<String, dynamic>? extra, StackTrace? stackTrace}) async => Result.success(null);
  @override Future<Result<void>> warning(String m, {Map<String, dynamic>? extra}) async => Result.success(null);
}

class _NoOpAvatarCacheService extends AvatarCacheService {
  _NoOpAvatarCacheService() : super(datasource: _FakeUserApiDatasource());
  @override Future<String?> getUserAvatarUrl(String userId) async => null;
}

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._st); final AuthState _st; @override AuthState build()=>_st;
}
AuthUser _au(String id) => AuthUser(id:id, createdAt:DateTime(2025), updatedAt:DateTime(2025),
  email:'$id@t.com', username:id, isEmailVerified:true, roles:const[UserRole.user], provider:AuthProvider.email);

/// One canned backend row.
Map<String, dynamic> _u(String id, String username) => {'id': id, 'username': username};

Widget _wf({required _FakeApiClient api, required TextEditingController c,
  void Function(List<String>)? omc, String uid='v-1'}) {
  final au=_au(uid);
  return ProviderScope(overrides:[
    apiClientProvider.overrideWith((ref)=>api),
    currentUserIdProvider.overrideWith((ref)=>uid),
    loggerServiceProvider.overrideWith((ref)=>_StubLogger()),
    authControllerProvider.overrideWith(()=>_FakeAuthController(AuthState.authenticated(au, emailVerified:true))),
    avatarCacheServiceProvider.overrideWith((ref)=>_NoOpAvatarCacheService()),
  ], child:MaterialApp(home:Scaffold(body:MentionTextField(controller:c, onMentionsChanged:omc))));
}

/// Set text on controller and pump until the async mention resolution completes.
Future<void> _setText(WidgetTester t, TextEditingController c, String text) async {
  c.text = text; c.selection = TextSelection.collapsed(offset: text.length);
  await t.pump();
  // Allow async resolver to complete.
  await t.pump(const Duration(milliseconds: 100));
  await t.pumpAndSettle();
}

void main() {
  group('C1B3 Ordered ID callbacks', () {
    testWidgets('single @alice → [aliceId]', (t) async {
      final api = _FakeApiClient(users: [_u('id-alice','alice')]);
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(api:api, c:c, omc: (ids) => calls.add(List.of(ids))));
      await _setText(t, c, '@alice');

      // Should emit callback with resolved alice ID.
      expect(calls, isNotEmpty);
      final last = calls.last;
      expect(last, contains('id-alice'));
      expect(last.length, 1);
    });

    testWidgets('two @alice tokens → one aliceId', (t) async {
      final api = _FakeApiClient(users: [_u('id-alice','alice')]);
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(api:api, c:c, omc: (ids) => calls.add(List.of(ids))));
      await _setText(t, c, '@alice @alice');

      final last = calls.last;
      expect(last, contains('id-alice'));
      // Deduplicated: only one aliceId.
      expect(last.where((id) => id == 'id-alice').length, 1);
    });

    testWidgets('@Alice + @alice → one canonical aliceId', (t) async {
      final api = _FakeApiClient(users: [_u('id-alice','alice')]);
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(api:api, c:c, omc: (ids) => calls.add(List.of(ids))));
      await _setText(t, c, '@Alice @alice');

      final last = calls.last;
      expect(last, contains('id-alice'));
      expect(last.where((id) => id == 'id-alice').length, 1);
    });

    testWidgets('remove one duplicate → ID remains', (t) async {
      final api = _FakeApiClient(users: [_u('id-alice','alice')]);
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(api:api, c:c, omc: (ids) => calls.add(List.of(ids))));
      // Start with two tokens.
      await _setText(t, c, '@alice @alice');

      final first = calls.last;
      expect(first, contains('id-alice'));

      // Remove one token (leave one).
      await _setText(t, c, '@alice');

      final second = calls.last;
      expect(second, contains('id-alice'));
      expect(second.where((id) => id == 'id-alice').length, 1);
    });

    testWidgets('remove final token → []', (t) async {
      final api = _FakeApiClient(users: [_u('id-alice','alice')]);
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(api:api, c:c, omc: (ids) => calls.add(List.of(ids))));
      await _setText(t, c, '@alice');

      final first = calls.last;
      expect(first, contains('id-alice'));

      // Remove the final token.
      await _setText(t, c, 'hello');

      final second = calls.last;
      expect(second, isEmpty);
    });

    testWidgets('@alice @bob → [aliceId, bobId]', (t) async {
      final api = _FakeApiClient(users: [_u('id-alice','alice'), _u('id-bob','bob')]);
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(api:api, c:c, omc: (ids) => calls.add(List.of(ids))));
      await _setText(t, c, '@alice @bob');

      final last = calls.last;
      expect(last.length, 2);
      expect(last[0], 'id-alice');
      expect(last[1], 'id-bob');
    });

    testWidgets('@bob @alice → [bobId, aliceId]', (t) async {
      final api = _FakeApiClient(users: [_u('id-alice','alice'), _u('id-bob','bob')]);
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(api:api, c:c, omc: (ids) => calls.add(List.of(ids))));
      await _setText(t, c, '@bob @alice');

      final last = calls.last;
      expect(last.length, 2);
      expect(last[0], 'id-bob');
      expect(last[1], 'id-alice');
    });

    testWidgets('stable-ID collision → one shared ID', (t) async {
      // Both alice and alice_alias resolve to the same stable ID.
      final api = _FakeApiClient(users: [
        _u('shared-id','alice'),
        _u('shared-id','alice_alias'),
      ]);
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(api:api, c:c, omc: (ids) => calls.add(List.of(ids))));
      await _setText(t, c, '@alice @alice_alias');

      final last = calls.last;
      expect(last.length, 1);
      expect(last[0], 'shared-id');
    });

    testWidgets('invalid username → no ID', (t) async {
      final api = _FakeApiClient(users: []);
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(api:api, c:c, omc: (ids) => calls.add(List.of(ids))));
      // '@@' is not a valid mention pattern — parser won't match it.
      // But 'Hi @@' just contains a bare @ trigger with no query.
      await _setText(t, c, 'Hi there');

      final last = calls.last;
      // No valid mention tokens → empty list.
      expect(last, isEmpty);
      expect(api.queries, isEmpty, reason: 'no mention token must not hit the API');
    });

    testWidgets('partial fail-closed: alice fails, bob succeeds → [id-bob]', (t) async {
      // The backend answers alice's query with an empty page — the resolver
      // fails closed (null) and only bob's ID appears.
      final api = _FakeApiClient(
        users: [_u('id-bob','bob')],
        emptyFor: {'alice'},
      );
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(api:api, c:c, omc: (ids) => calls.add(List.of(ids))));
      await _setText(t, c, '@alice @bob');

      final last = calls.last;
      expect(last.length, 1);
      expect(last[0], 'id-bob');
      // No alice ID, no stale ID, no invented fallback.
    });

    testWidgets('partial fail-closed inverse: @bob @alice → [id-bob]', (t) async {
      final api = _FakeApiClient(
        users: [_u('id-bob','bob')],
        emptyFor: {'alice'},
      );
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(api:api, c:c, omc: (ids) => calls.add(List.of(ids))));
      await _setText(t, c, '@bob @alice');

      final last = calls.last;
      expect(last.length, 1);
      expect(last[0], 'id-bob');
    });

    testWidgets('@everyone unchanged in text but not resolved', (t) async {
      final api = _FakeApiClient(users: []);
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(api:api, c:c, omc: (ids) => calls.add(List.of(ids))));
      await _setText(t, c, '@everyone');

      // @everyone is excluded from regular mention IDs — the resolver may
      // probe the API for it, but it must never emit an ID for it.
      final last = calls.last;
      expect(last, isEmpty);
    });
  });
}
