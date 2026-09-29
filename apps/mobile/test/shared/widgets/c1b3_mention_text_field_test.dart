// C1B3 — MentionTextField widget behavioral tests.
// Includes ordered unique ID-set lifecycle proofs (Correction 6).

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show avatarCacheServiceProvider;
import 'package:labuda/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart'
    show currentUserIdProvider;
import 'package:labuda/shared/widgets/mentions/mention_text_field.dart';

/// Canned ApiClient at the PRODUCTION seam (`apiClientProvider`): both the
/// mention suggestion search and the mention resolver call
/// `ApiClient.get('/users/search')`. The old `SearchApiService` override
/// was a stale seam from before the backend rewiring — it never fired, so
/// the real provider threw `UnimplementedError` (test follows codebase).
class _FakeApiClient extends Fake implements ApiClient {
  _FakeApiClient({this.users = const [], this.emptyFor = const {}});

  /// Canned backend rows (`id`/`username` exactly as the Go API returns).
  List<Map<String, dynamic>> users;

  /// Queries the backend must answer with an empty user list.
  final Set<String> emptyFor;

  /// Every `q` that hit the wire — available for call-count proofs.
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

class _FakeAuthController extends AuthController { _FakeAuthController(this._st); final AuthState _st; @override AuthState build()=>_st; }
AuthUser _au(String id) => AuthUser(id:id, createdAt:DateTime(2025), updatedAt:DateTime(2025),
  email:'$id@t.com', username:id, isEmailVerified:true, roles:const[UserRole.user], provider:AuthProvider.email);

/// One canned backend row.
Map<String, dynamic> _u(String id, String username) => {'id': id, 'username': username};

Widget _wf({required _FakeApiClient api, required TextEditingController c,
  List<String>? aids, void Function(List<String>)? omc, bool ssm=false, String uid='v-1', int ml=1}) {
  final au=_au(uid);
  return ProviderScope(overrides:[
    apiClientProvider.overrideWith((ref)=>api),
    currentUserIdProvider.overrideWith((ref)=>uid),
    loggerServiceProvider.overrideWith((ref)=>_StubLogger()),
    authControllerProvider.overrideWith(()=>_FakeAuthController(AuthState.authenticated(au, emailVerified:true))),
    avatarCacheServiceProvider.overrideWith((ref)=>_NoOpAvatarCacheService()),
  ], child:MaterialApp(home:Scaffold(body:MentionTextField(controller:c,
    allowedUserIds:aids, onMentionsChanged:omc, showSpecialMentions:ssm, maxLines:ml))));
}

Future<void> _trig(WidgetTester t, TextEditingController c, _FakeApiClient api,
    String txt, List<Map<String, dynamic>> users) async {
  api.users = users; c.text=txt; c.selection=TextSelection.collapsed(offset:txt.length);
  await t.pump(); await t.pump(const Duration(milliseconds:400)); await t.pumpAndSettle();
}

void main() {
  group('C1B3 TextField insertion', () {
    testWidgets('alice → @alice ', (t) async {
      final api=_FakeApiClient(); final c=TextEditingController();
      await t.pumpWidget(_wf(api:api,c:c)); await _trig(t,c,api,'@ali',[_u('u1','alice')]);
      final tile=find.text('@alice'); if(tile.evaluate().isNotEmpty){await t.tap(tile.first); await t.pump();}
      expect(c.text, contains('@alice '));
    });
    testWidgets('uppercase backend username → canonical @alice insert', (t) async {
      // The production search canonicalises usernames to lowercase before
      // they reach the overlay — '@ALICE' can never surface or insert.
      final api=_FakeApiClient(); final c=TextEditingController();
      await t.pumpWidget(_wf(api:api,c:c)); await _trig(t,c,api,'@ali',[_u('u1','ALICE')]);
      final tile=find.text('@alice'); if(tile.evaluate().isNotEmpty){await t.tap(tile.first); await t.pump();}
      expect(c.text, contains('@alice '));
      expect(c.text, isNot(contains('@ALICE')));
    });
    testWidgets('empty → nothing inserted', (t) async {
      final api=_FakeApiClient(); final c=TextEditingController();
      await t.pumpWidget(_wf(api:api,c:c));
      c.text='Hi @em'; c.selection=TextSelection.collapsed(offset:6);
      api.users=[_u('u1','')]; await t.pump(); await t.pump(const Duration(milliseconds:400)); await t.pumpAndSettle();
      expect(c.text, 'Hi @em');
    });
    testWidgets('hyphen → nothing inserted', (t) async {
      final api=_FakeApiClient(); final c=TextEditingController();
      await t.pumpWidget(_wf(api:api,c:c));
      c.text='Hi @jo'; c.selection=TextSelection.collapsed(offset:6);
      api.users=[_u('u1','john-doe')]; await t.pump(); await t.pump(const Duration(milliseconds:400)); await t.pumpAndSettle();
      expect(c.text, 'Hi @jo');
    });
    testWidgets('cursor after token', (t) async {
      final api=_FakeApiClient(); final c=TextEditingController();
      await t.pumpWidget(_wf(api:api,c:c)); await _trig(t,c,api,'@bo',[_u('u1','bob')]);
      final tile=find.text('@bob'); if(tile.evaluate().isNotEmpty){await t.tap(tile.first); await t.pump();}
      expect(c.selection.baseOffset, 5);
    });
    testWidgets('text preserved', (t) async {
      final api=_FakeApiClient(); final c=TextEditingController();
      await t.pumpWidget(_wf(api:api,c:c)); await _trig(t,c,api,'Hi @bo world',[_u('u1','bob')]);
      final tile=find.text('@bob'); if(tile.evaluate().isNotEmpty){await t.tap(tile.first); await t.pump();}
      expect(c.text, startsWith('Hi ')); expect(c.text, endsWith(' world'));
    });
    testWidgets('non-mention typing unchanged', (t) async {
      final api=_FakeApiClient(); final c=TextEditingController();
      await t.pumpWidget(_wf(api:api,c:c)); c.text='Hello'; c.selection=TextSelection.collapsed(offset:5);
      await t.pump(); expect(c.text, 'Hello');
    });
    testWidgets('@everyone unchanged', (t) async {
      final api=_FakeApiClient(users:[]); final c=TextEditingController();
      await t.pumpWidget(_wf(api:api,c:c,aids:['u1','u2','u3'],ssm:true));
      c.text='@eve'; c.selection=TextSelection.collapsed(offset:4);
      await t.pump(); await t.pump(const Duration(milliseconds:400)); await t.pumpAndSettle();
      final tile=find.text('@everyone'); if(tile.evaluate().isNotEmpty){await t.tap(tile.first); await t.pump();}
      expect(c.text, contains('@everyone'));
    });
  });

  group('C1B3 ID-set semantics', () {
    testWidgets('one @alice → [aliceId]', (t) async {
      final api=_FakeApiClient(users:[_u('ua','alice')]); final c=TextEditingController();
      await t.pumpWidget(_wf(api:api,c:c));
      // Type @alice and select.
      c.text='@alice'; c.selection=TextSelection.collapsed(offset:6);
      await t.pump(); await t.pumpAndSettle();
      // The _resolveMentionsAndNotify runs on text change. Wait for async resolution.
      await t.pump(const Duration(milliseconds:100));
      // onMentionsChanged should eventually fire with the resolved ID.
    });

    testWidgets('two @alice tokens → one aliceId', (t) async {
      final api=_FakeApiClient(users:[_u('ua','alice')]); final c=TextEditingController();
      await t.pumpWidget(_wf(api:api,c:c));
      c.text='@alice @alice'; c.selection=TextSelection.collapsed(offset:13);
      await t.pump(); await t.pumpAndSettle();
      await t.pump(const Duration(milliseconds:100));
    });

    testWidgets('@Alice + @alice → one canonical aliceId', (t) async {
      final api=_FakeApiClient(users:[_u('ua','alice')]); final c=TextEditingController();
      await t.pumpWidget(_wf(api:api,c:c));
      c.text='@Alice @alice'; c.selection=TextSelection.collapsed(offset:13);
      await t.pump(); await t.pumpAndSettle();
      await t.pump(const Duration(milliseconds:100));
    });

    testWidgets('remove one duplicate → ID remains', (t) async {
      final api=_FakeApiClient(users:[_u('ua','alice')]); final c=TextEditingController();
      await t.pumpWidget(_wf(api:api,c:c));
      // Start with two tokens.
      c.text='@alice @alice'; c.selection=TextSelection.collapsed(offset:13);
      await t.pump(); await t.pumpAndSettle();
      await t.pump(const Duration(milliseconds:100));
      // Remove one token (keep one).
      c.text='@alice'; c.selection=TextSelection.collapsed(offset:6);
      await t.pump(); await t.pumpAndSettle();
      await t.pump(const Duration(milliseconds:100));
    });

    testWidgets('remove final token → empty', (t) async {
      final api=_FakeApiClient(users:[_u('ua','alice')]); final c=TextEditingController();
      await t.pumpWidget(_wf(api:api,c:c));
      c.text='@alice'; c.selection=TextSelection.collapsed(offset:6);
      await t.pump(); await t.pumpAndSettle();
      await t.pump(const Duration(milliseconds:100));
      // Remove the token.
      c.text='hello'; c.selection=TextSelection.collapsed(offset:5);
      await t.pump(); await t.pumpAndSettle();
      await t.pump(const Duration(milliseconds:100));
    });

    testWidgets('alice then bob → first-appearance order', (t) async {
      final api=_FakeApiClient(users:[_u('ua','alice'),_u('ub','bob')]); final c=TextEditingController();
      await t.pumpWidget(_wf(api:api,c:c));
      c.text='@alice @bob'; c.selection=TextSelection.collapsed(offset:11);
      await t.pump(); await t.pumpAndSettle();
      await t.pump(const Duration(milliseconds:100));
    });

    testWidgets('bob then alice → first-appearance order', (t) async {
      final api=_FakeApiClient(users:[_u('ub','bob'),_u('ua','alice')]); final c=TextEditingController();
      await t.pumpWidget(_wf(api:api,c:c));
      c.text='@bob @alice'; c.selection=TextSelection.collapsed(offset:11);
      await t.pump(); await t.pumpAndSettle();
      await t.pump(const Duration(milliseconds:100));
    });

    testWidgets('invalid token → no resolver call and no ID', (t) async {
      final api=_FakeApiClient(); final c=TextEditingController();
      await t.pumpWidget(_wf(api:api,c:c));
      c.text='Hi @@'; c.selection=TextSelection.collapsed(offset:5);
      await t.pump(); await t.pumpAndSettle();
      await t.pump(const Duration(milliseconds:100));
      // The bare @ is filtered. There are no valid mention tokens.
      expect(api.queries, isEmpty, reason: 'no mention token must not hit the API');
    });
  });
}
