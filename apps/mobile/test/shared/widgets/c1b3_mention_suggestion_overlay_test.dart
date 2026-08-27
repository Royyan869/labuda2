// C1B3 — MentionSuggestionOverlay widget behavioral tests.

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
import 'package:labuda/features/search/search/domain/entities/user_search.dart';
import 'package:labuda/features/search/search/presentation/providers/providers.dart'
    show searchApiServiceProvider;
import 'package:labuda/shared/providers/auth_status_providers.dart'
    show currentUserIdProvider;
import 'package:labuda/shared/widgets/mentions/mention_suggestion_overlay.dart';
import 'package:labuda/shared/widgets/hybrid_avatar.dart';

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

Widget _w({required _RecordingSearchApiService svc, required String q, List<String>? aids,
  required Function(UserSearch) onSel, bool ssm=false, String uid='v-1'}) {
  final au=_au(uid);
  return ProviderScope(overrides:[
    searchApiServiceProvider.overrideWith((ref)=>svc),
    currentUserIdProvider.overrideWith((ref)=>uid),
    loggerServiceProvider.overrideWith((ref)=>_StubLogger()),
    authControllerProvider.overrideWith(()=>_FakeAuthController(AuthState.authenticated(au, emailVerified:true))),
    avatarCacheServiceProvider.overrideWith((ref)=>_NoOpAvatarCacheService()),
  ], child:MaterialApp(home:Scaffold(body:MentionSuggestionOverlay(query:q,
    allowedUserIds:aids, onUserSelected:onSel, onDismiss:(){}, showSpecialMentions:ssm))));
}

void main() {
  group('C1B3 Overlay', () {
    testWidgets('lowercase canonical display', (t) async {
      final svc=_RecordingSearchApiService(); svc.canned=_r([_d('u1','alice')]);
      await t.pumpWidget(_w(svc:svc,q:'ali',onSel:(_){})); await t.pumpAndSettle();
      expect(find.text('@alice'), findsAtLeast(1));
    });
    testWidgets('defensive uppercase input → lowercase canonical', (t) async {
      final svc=_RecordingSearchApiService(); svc.canned=_r([_d('u1','Alice')]);
      await t.pumpWidget(_w(svc:svc,q:'ali',onSel:(_){})); await t.pumpAndSettle();
      expect(find.text('@alice'), findsAtLeast(1));
    });
    testWidgets('raw @alice', (t) async {
      final svc=_RecordingSearchApiService(); svc.canned=_r([_d('u1','@alice')]);
      await t.pumpWidget(_w(svc:svc,q:'ali',onSel:(_){})); await t.pumpAndSettle();
      expect(find.text('@alice'), findsAtLeast(1)); expect(find.text('@@alice'), findsNothing);
    });
    testWidgets('multi-@ @@alice', (t) async {
      final svc=_RecordingSearchApiService(); svc.canned=_r([_d('u1','@@alice')]);
      await t.pumpWidget(_w(svc:svc,q:'ali',onSel:(_){})); await t.pumpAndSettle();
      expect(find.text('@alice'), findsAtLeast(1)); expect(find.text('@@alice'), findsNothing);
    });
    testWidgets('too-short → no row', (t) async {
      final svc=_RecordingSearchApiService(); svc.canned=_r([_d('u1','ab')]);
      await t.pumpWidget(_w(svc:svc,q:'ab',onSel:(_){})); await t.pumpAndSettle();
      expect(find.text('No users found'), findsOneWidget);
    });
    testWidgets('hyphen → no row', (t) async {
      final svc=_RecordingSearchApiService(); svc.canned=_r([_d('u1','john-doe')]);
      await t.pumpWidget(_w(svc:svc,q:'john',onSel:(_){})); await t.pumpAndSettle();
      expect(find.text('No users found'), findsOneWidget);
    });
    testWidgets('period → no row', (t) async {
      final svc=_RecordingSearchApiService(); svc.canned=_r([_d('u1','john.doe')]);
      await t.pumpWidget(_w(svc:svc,q:'john',onSel:(_){})); await t.pumpAndSettle();
      expect(find.text('No users found'), findsOneWidget);
    });
    testWidgets('numeric-only retained', (t) async {
      final svc=_RecordingSearchApiService(); svc.canned=_r([_d('u1','12345')]);
      await t.pumpWidget(_w(svc:svc,q:'123',onSel:(_){})); await t.pumpAndSettle();
      expect(find.text('@12345'), findsAtLeast(1));
    });
    testWidgets('UUID-with-hyphens → no row', (t) async {
      final svc=_RecordingSearchApiService(); svc.canned=_r([_d('u1','550e8400-e29b-41d4-a716-446655440000')]);
      await t.pumpWidget(_w(svc:svc,q:'550e',onSel:(_){})); await t.pumpAndSettle();
      expect(find.text('No users found'), findsOneWidget);
    });
    testWidgets('invalid row cannot be tapped', (t) async {
      final svc=_RecordingSearchApiService(); svc.canned=_r([_d('u1','@')]);
      bool called=false;
      await t.pumpWidget(_w(svc:svc,q:'@',onSel:(_)=>called=true)); await t.pumpAndSettle();
      expect(called, isFalse);
    });
    testWidgets('valid callback carries stable ID', (t) async {
      final svc=_RecordingSearchApiService(); svc.canned=_r([_d('user-42','bob')]);
      UserSearch? tapped;
      await t.pumpWidget(_w(svc:svc,q:'bob',onSel:(u)=>tapped=u)); await t.pumpAndSettle();
      await t.tap(find.text('@bob').first); await t.pump();
      expect(tapped!.userId, 'user-42'); expect(tapped!.userId, isNot(contains('@')));
    });
    testWidgets('avatar correct', (t) async {
      final svc=_RecordingSearchApiService(); svc.canned=_r([_d('user-42','bob',a:'https://i.e/b.jpg')]);
      await t.pumpWidget(_w(svc:svc,q:'bob',onSel:(_){})); await t.pumpAndSettle();
      final a=t.widget<HybridAvatar>(find.byType(HybridAvatar));
      expect(a.userId, 'user-42'); expect(a.username, 'bob');
    });
    testWidgets('@everyone unchanged', (t) async {
      final svc=_RecordingSearchApiService(); svc.canned=_r([]); UserSearch? tapped;
      await t.pumpWidget(_w(svc:svc,q:'eve',aids:['u1','u2','u3'],ssm:true,onSel:(u)=>tapped=u));
      await t.pumpAndSettle();
      expect(find.text('@everyone'), findsOneWidget);
      await t.tap(find.text('@everyone')); await t.pump();
      expect(tapped!.userId, 'everyone');
    });
  });
}
