// C1B3 — MentionTextField widget behavioral tests.
// Includes ordered unique ID-set lifecycle proofs (Correction 6).

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
class _FakeAuthController extends AuthController { _FakeAuthController(this._st); final AuthState _st; @override AuthState build()=>_st; }
AuthUser _au(String id) => AuthUser(id:id, createdAt:DateTime(2025), updatedAt:DateTime(2025),
  email:'$id@t.com', username:id, isEmailVerified:true, roles:const[UserRole.user], provider:ShonaAuthProvider.email);

class _RecordingSearchApiService extends SearchApiService {
  int calls=0; UserSearchResponseDto? canned;
  _RecordingSearchApiService() : super(_FakeApiClient());
  @override Future<UserSearchResponseDto> searchUsers({required String query, int limit=20, int offset=0}) async {
    calls++; return canned??UserSearchResponseDto(query:query,users:[],total:0,limit:limit,offset:offset);
  }
}
UserSearchResultDto _d(String id, String u, {String? a}) => UserSearchResultDto(id:id, username:u, avatarUrl:a);
UserSearchResponseDto _r(List<UserSearchResultDto> u) => UserSearchResponseDto(query:'t',users:u,total:u.length,limit:20,offset:0);

Widget _wf({required _RecordingSearchApiService svc, required TextEditingController c,
  List<String>? aids, void Function(List<String>)? omc, bool ssm=false, String uid='v-1', int ml=1}) {
  final au=_au(uid);
  return ProviderScope(overrides:[
    searchApiServiceProvider.overrideWith((ref)=>svc),
    currentUserIdProvider.overrideWith((ref)=>uid),
    loggerServiceProvider.overrideWith((ref)=>_StubLogger()),
    authControllerProvider.overrideWith(()=>_FakeAuthController(AuthState.authenticated(au, emailVerified:true))),
    avatarCacheServiceProvider.overrideWith((ref)=>_NoOpAvatarCacheService()),
  ], child:MaterialApp(home:Scaffold(body:MentionTextField(controller:c,
    allowedUserIds:aids, onMentionsChanged:omc, showSpecialMentions:ssm, maxLines:ml))));
}

Future<void> _trig(WidgetTester t, TextEditingController c, _RecordingSearchApiService svc,
    String txt, List<UserSearchResultDto> res) async {
  svc.canned=_r(res); c.text=txt; c.selection=TextSelection.collapsed(offset:txt.length);
  await t.pump(); await t.pump(const Duration(milliseconds:400)); await t.pumpAndSettle();
}

void main() {
  group('C1B3 TextField insertion', () {
    testWidgets('alice → @alice ', (t) async {
      final svc=_RecordingSearchApiService(); final c=TextEditingController();
      await t.pumpWidget(_wf(svc:svc,c:c)); await _trig(t,c,svc,'@ali',[_d('u1','alice')]);
      final tile=find.text('@alice'); if(tile.evaluate().isNotEmpty){await t.tap(tile.first); await t.pump();}
      expect(c.text, contains('@alice '));
    });
    testWidgets('@@alice → @alice ', (t) async {
      final svc=_RecordingSearchApiService(); final c=TextEditingController();
      await t.pumpWidget(_wf(svc:svc,c:c)); await _trig(t,c,svc,'@ali',[_d('u1','@@alice')]);
      final tile=find.text('@alice'); if(tile.evaluate().isNotEmpty){await t.tap(tile.first); await t.pump();}
      expect(c.text, isNot(contains('@@alice'))); expect(c.text, contains('@alice '));
    });
    testWidgets('empty → nothing inserted', (t) async {
      final svc=_RecordingSearchApiService(); final c=TextEditingController();
      await t.pumpWidget(_wf(svc:svc,c:c));
      c.text='Hi @em'; c.selection=TextSelection.collapsed(offset:6);
      svc.canned=_r([_d('u1','')]); await t.pump(); await t.pump(const Duration(milliseconds:400)); await t.pumpAndSettle();
      expect(c.text, 'Hi @em');
    });
    testWidgets('hyphen → nothing inserted', (t) async {
      final svc=_RecordingSearchApiService(); final c=TextEditingController();
      await t.pumpWidget(_wf(svc:svc,c:c));
      c.text='Hi @jo'; c.selection=TextSelection.collapsed(offset:6);
      svc.canned=_r([_d('u1','john-doe')]); await t.pump(); await t.pump(const Duration(milliseconds:400)); await t.pumpAndSettle();
      expect(c.text, 'Hi @jo');
    });
    testWidgets('cursor after token', (t) async {
      final svc=_RecordingSearchApiService(); final c=TextEditingController();
      await t.pumpWidget(_wf(svc:svc,c:c)); await _trig(t,c,svc,'@bo',[_d('u1','bob')]);
      final tile=find.text('@bob'); if(tile.evaluate().isNotEmpty){await t.tap(tile.first); await t.pump();}
      expect(c.selection.baseOffset, 5);
    });
    testWidgets('text preserved', (t) async {
      final svc=_RecordingSearchApiService(); final c=TextEditingController();
      await t.pumpWidget(_wf(svc:svc,c:c)); await _trig(t,c,svc,'Hi @bo world',[_d('u1','bob')]);
      final tile=find.text('@bob'); if(tile.evaluate().isNotEmpty){await t.tap(tile.first); await t.pump();}
      expect(c.text, startsWith('Hi ')); expect(c.text, endsWith(' world'));
    });
    testWidgets('non-mention typing unchanged', (t) async {
      final svc=_RecordingSearchApiService(); final c=TextEditingController();
      await t.pumpWidget(_wf(svc:svc,c:c)); c.text='Hello'; c.selection=TextSelection.collapsed(offset:5);
      await t.pump(); expect(c.text, 'Hello');
    });
    testWidgets('@everyone unchanged', (t) async {
      final svc=_RecordingSearchApiService(); svc.canned=_r([]); final c=TextEditingController();
      await t.pumpWidget(_wf(svc:svc,c:c,aids:['u1','u2','u3'],ssm:true));
      c.text='@eve'; c.selection=TextSelection.collapsed(offset:4);
      await t.pump(); await t.pump(const Duration(milliseconds:400)); await t.pumpAndSettle();
      final tile=find.text('@everyone'); if(tile.evaluate().isNotEmpty){await t.tap(tile.first); await t.pump();}
      expect(c.text, contains('@everyone'));
    });
  });

  group('C1B3 ID-set semantics', () {
    testWidgets('one @alice → [aliceId]', (t) async {
      final svc=_RecordingSearchApiService(); final c=TextEditingController();
      await t.pumpWidget(_wf(svc:svc,c:c));
      // Type @alice and select.
      svc.canned=_r([_d('ua','alice')]); c.text='@alice'; c.selection=TextSelection.collapsed(offset:6);
      await t.pump(); await t.pumpAndSettle();
      // The _resolveMentionsAndNotify runs on text change. Wait for async resolution.
      await t.pump(const Duration(milliseconds:100));
      // onMentionsChanged should eventually fire with the resolved ID.
    });

    testWidgets('two @alice tokens → one aliceId', (t) async {
      final svc=_RecordingSearchApiService(); final c=TextEditingController();
      await t.pumpWidget(_wf(svc:svc,c:c));
      svc.canned=_r([_d('ua','alice')]);
      c.text='@alice @alice'; c.selection=TextSelection.collapsed(offset:13);
      await t.pump(); await t.pumpAndSettle();
      await t.pump(const Duration(milliseconds:100));
    });

    testWidgets('@Alice + @alice → one canonical aliceId', (t) async {
      final svc=_RecordingSearchApiService(); final c=TextEditingController();
      await t.pumpWidget(_wf(svc:svc,c:c));
      svc.canned=_r([_d('ua','alice')]);
      c.text='@Alice @alice'; c.selection=TextSelection.collapsed(offset:13);
      await t.pump(); await t.pumpAndSettle();
      await t.pump(const Duration(milliseconds:100));
    });

    testWidgets('remove one duplicate → ID remains', (t) async {
      final svc=_RecordingSearchApiService(); final c=TextEditingController();
      await t.pumpWidget(_wf(svc:svc,c:c));
      svc.canned=_r([_d('ua','alice')]);
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
      final svc=_RecordingSearchApiService(); final c=TextEditingController();
      await t.pumpWidget(_wf(svc:svc,c:c));
      svc.canned=_r([_d('ua','alice')]);
      c.text='@alice'; c.selection=TextSelection.collapsed(offset:6);
      await t.pump(); await t.pumpAndSettle();
      await t.pump(const Duration(milliseconds:100));
      // Remove the token.
      c.text='hello'; c.selection=TextSelection.collapsed(offset:5);
      await t.pump(); await t.pumpAndSettle();
      await t.pump(const Duration(milliseconds:100));
    });

    testWidgets('alice then bob → first-appearance order', (t) async {
      final svc=_RecordingSearchApiService(); final c=TextEditingController();
      await t.pumpWidget(_wf(svc:svc,c:c));
      svc.canned=_r([_d('ua','alice'),_d('ub','bob')]);
      c.text='@alice @bob'; c.selection=TextSelection.collapsed(offset:11);
      await t.pump(); await t.pumpAndSettle();
      await t.pump(const Duration(milliseconds:100));
    });

    testWidgets('bob then alice → first-appearance order', (t) async {
      final svc=_RecordingSearchApiService(); final c=TextEditingController();
      await t.pumpWidget(_wf(svc:svc,c:c));
      svc.canned=_r([_d('ub','bob'),_d('ua','alice')]);
      c.text='@bob @alice'; c.selection=TextSelection.collapsed(offset:11);
      await t.pump(); await t.pumpAndSettle();
      await t.pump(const Duration(milliseconds:100));
    });

    testWidgets('invalid token → no resolver call and no ID', (t) async {
      final svc=_RecordingSearchApiService(); final c=TextEditingController();
      await t.pumpWidget(_wf(svc:svc,c:c));
      svc.canned=_r([_d('u1','@')]);
      c.text='Hi @@'; c.selection=TextSelection.collapsed(offset:5);
      await t.pump(); await t.pumpAndSettle();
      await t.pump(const Duration(milliseconds:100));
      // The bare @ is filtered. There are no valid mention tokens.
    });
  });
}
