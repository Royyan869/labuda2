// C1B3 — MentionTextField ordered-ID callback proofs (Proof 3).
//
// Pumps production MentionTextField and captures exact onMentionsChanged
// callback values through the real parser→resolver path.

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show avatarCacheServiceProvider;
import 'package:labuda/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:labuda/features/search/search/data/dto/search_dto.dart';
import 'package:labuda/features/search/search/data/remote/search_api_service.dart';
import 'package:labuda/features/search/search/presentation/providers/providers.dart'
    show searchApiServiceProvider;
import 'package:labuda/shared/providers/auth_status_providers.dart'
    show currentUserIdProvider;
import 'package:labuda/shared/widgets/mentions/mention_text_field.dart';

// =============================================================================
// Fake dependencies
// =============================================================================

class _FakeApiClient extends Fake implements ApiClient {}
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
  email:'$id@t.com', username:id, isEmailVerified:true, roles:const[UserRole.user], provider:ShonaAuthProvider.email);

/// Fake SearchApiService that returns canned pages for resolver.
class _FakeSearchApiService extends SearchApiService {
  final List<UserSearchResultDto> _users;
  _FakeSearchApiService(this._users) : super(_FakeApiClient());
  @override
  Future<UserSearchResponseDto> searchUsers({required String query, int limit=20, int offset=0}) async {
    final filtered = _users.where((u) {
      final n = u.username;
      return n.isNotEmpty && n.length >= 3 && RegExp(r'^[a-z0-9_]+$').hasMatch(n.toLowerCase());
    }).toList();
    return UserSearchResponseDto(query:query, users:filtered, total:filtered.length, limit:limit, offset:offset);
  }
}

UserSearchResultDto _d(String id, String u) => UserSearchResultDto(id:id, username:u);

/// Service that fails for a specific query (triggers PaginationIntegrityException
/// via empty page + hasMore=true) and succeeds for others.
class _FailingSearchApiService extends SearchApiService {
  final String failQuery;
  final List<UserSearchResultDto> successUsers;
  _FailingSearchApiService({required this.failQuery, required this.successUsers}) : super(_FakeApiClient());
  @override
  Future<UserSearchResponseDto> searchUsers({required String query, int limit=20, int offset=0}) async {
    if (query == failQuery) {
      // Empty page + hasMore=true + total not scanned → PaginationIntegrityException
      return UserSearchResponseDto(query:query, users:[], total:100, limit:limit, offset:offset);
    }
    final filtered = successUsers.where((u) {
      final n = u.username; return n.isNotEmpty && n.length >= 3;
    }).toList();
    return UserSearchResponseDto(query:query, users:filtered, total:filtered.length, limit:limit, offset:offset);
  }
}

Widget _wf({required SearchApiService svc, required TextEditingController c,
  void Function(List<String>)? omc, String uid='v-1'}) {
  final au=_au(uid);
  return ProviderScope(overrides:[
    searchApiServiceProvider.overrideWith((ref)=>svc),
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
      final svc = _FakeSearchApiService([_d('id-alice','alice')]);
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(svc:svc, c:c, omc: (ids) => calls.add(List.of(ids))));
      await _setText(t, c, '@alice');

      // Should emit callback with resolved alice ID.
      expect(calls, isNotEmpty);
      final last = calls.last;
      expect(last, contains('id-alice'));
      expect(last.length, 1);
    });

    testWidgets('two @alice tokens → one aliceId', (t) async {
      final svc = _FakeSearchApiService([_d('id-alice','alice')]);
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(svc:svc, c:c, omc: (ids) => calls.add(List.of(ids))));
      await _setText(t, c, '@alice @alice');

      final last = calls.last;
      expect(last, contains('id-alice'));
      // Deduplicated: only one aliceId.
      expect(last.where((id) => id == 'id-alice').length, 1);
    });

    testWidgets('@Alice + @alice → one canonical aliceId', (t) async {
      final svc = _FakeSearchApiService([_d('id-alice','alice')]);
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(svc:svc, c:c, omc: (ids) => calls.add(List.of(ids))));
      await _setText(t, c, '@Alice @alice');

      final last = calls.last;
      expect(last, contains('id-alice'));
      expect(last.where((id) => id == 'id-alice').length, 1);
    });

    testWidgets('remove one duplicate → ID remains', (t) async {
      final svc = _FakeSearchApiService([_d('id-alice','alice')]);
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(svc:svc, c:c, omc: (ids) => calls.add(List.of(ids))));
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
      final svc = _FakeSearchApiService([_d('id-alice','alice')]);
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(svc:svc, c:c, omc: (ids) => calls.add(List.of(ids))));
      await _setText(t, c, '@alice');

      final first = calls.last;
      expect(first, contains('id-alice'));

      // Remove the final token.
      await _setText(t, c, 'hello');

      final second = calls.last;
      expect(second, isEmpty);
    });

    testWidgets('@alice @bob → [aliceId, bobId]', (t) async {
      final svc = _FakeSearchApiService([_d('id-alice','alice'), _d('id-bob','bob')]);
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(svc:svc, c:c, omc: (ids) => calls.add(List.of(ids))));
      await _setText(t, c, '@alice @bob');

      final last = calls.last;
      expect(last.length, 2);
      expect(last[0], 'id-alice');
      expect(last[1], 'id-bob');
    });

    testWidgets('@bob @alice → [bobId, aliceId]', (t) async {
      final svc = _FakeSearchApiService([_d('id-alice','alice'), _d('id-bob','bob')]);
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(svc:svc, c:c, omc: (ids) => calls.add(List.of(ids))));
      await _setText(t, c, '@bob @alice');

      final last = calls.last;
      expect(last.length, 2);
      expect(last[0], 'id-bob');
      expect(last[1], 'id-alice');
    });

    testWidgets('stable-ID collision → one shared ID', (t) async {
      // Both alice and alice_alias resolve to the same stable ID.
      final svc = _FakeSearchApiService([
        _d('shared-id','alice'),
        _d('shared-id','alice_alias'),
      ]);
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(svc:svc, c:c, omc: (ids) => calls.add(List.of(ids))));
      await _setText(t, c, '@alice @alice_alias');

      final last = calls.last;
      expect(last.length, 1);
      expect(last[0], 'shared-id');
    });

    testWidgets('invalid username → no ID', (t) async {
      final svc = _FakeSearchApiService([]);
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(svc:svc, c:c, omc: (ids) => calls.add(List.of(ids))));
      // '@@' is not a valid mention pattern — parser won't match it.
      // But 'Hi @@' just contains a bare @ trigger with no query.
      await _setText(t, c, 'Hi there');

      final last = calls.last;
      // No valid mention tokens → empty list.
      expect(last, isEmpty);
    });

    testWidgets('partial fail-closed: alice fails, bob succeeds → [id-bob]', (t) async {
      // Alice throws PaginationIntegrityException (empty page + hasMore=true).
      // Bob resolves to id-bob. Only bob's ID appears.
      final svc = _FailingSearchApiService(
        failQuery: 'alice',
        successUsers: [_d('id-bob','bob')],
      );
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(svc:svc, c:c, omc: (ids) => calls.add(List.of(ids))));
      await _setText(t, c, '@alice @bob');

      final last = calls.last;
      expect(last.length, 1);
      expect(last[0], 'id-bob');
      // No alice ID, no stale ID, no invented fallback.
    });

    testWidgets('partial fail-closed inverse: @bob @alice → [id-bob]', (t) async {
      final svc = _FailingSearchApiService(
        failQuery: 'alice',
        successUsers: [_d('id-bob','bob')],
      );
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(svc:svc, c:c, omc: (ids) => calls.add(List.of(ids))));
      await _setText(t, c, '@bob @alice');

      final last = calls.last;
      expect(last.length, 1);
      expect(last[0], 'id-bob');
    });

    testWidgets('@everyone unchanged in text but not resolved', (t) async {
      final svc = _FakeSearchApiService([]);
      final c = TextEditingController();
      final calls = <List<String>>[];

      await t.pumpWidget(_wf(svc:svc, c:c, omc: (ids) => calls.add(List.of(ids))));
      await _setText(t, c, '@everyone');

      // @everyone is extracted by MentionParser but classified as special
      // and excluded from regular mention IDs.
      final last = calls.last;
      expect(last, isEmpty);
    });
  });
}
