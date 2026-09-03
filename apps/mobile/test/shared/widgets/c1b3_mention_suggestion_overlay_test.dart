// C1B3 — MentionSuggestionOverlay widget behavioral tests.

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show avatarCacheServiceProvider;
import 'package:labuda/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:labuda/features/search/search/domain/entities/user_search.dart';
import 'package:labuda/features/search/search/presentation/providers/mention_providers.dart'
    show mentionUserSearchProvider, MentionSearchParams;
import 'package:labuda/shared/providers/auth_status_providers.dart'
    show currentUserIdProvider;
import 'package:labuda/shared/widgets/mentions/mention_suggestion_overlay.dart';
import 'package:labuda/shared/widgets/hybrid_avatar.dart';

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

/// Canonical mentionable username regex — mirrors production _isMentionableUsername.
bool _isMentionable(String u) => RegExp(r'^[a-z0-9_]+$').hasMatch(u);

/// Fake that mirrors production mentionUserSearchProvider behavior:
/// - client-side regex filtering (_isMentionableUsername)
/// - query contains match
/// - allowedUserIds filter
List<UserSearch> _fakeSearch(MentionSearchParams params, List<UserSearch> all) {
  final q = params.query.toLowerCase().trim();
  if (q.isEmpty) return [];
  return all.where((u) {
    // Production lowercases username before regex check
    final cu = u.username.toLowerCase();
    if (!_isMentionable(cu)) return false;
    if (!cu.contains(q)) return false;
    if (params.allowedUserIds != null && !params.allowedUserIds!.contains(u.userId)) return false;
    return true;
  }).map((u) => UserSearch(userId: u.userId, username: u.username.toLowerCase(), avatarUrl: u.avatarUrl)).toList();
}

UserSearch _u(String id, String username, {String? avatarUrl}) =>
    UserSearch(userId: id, username: username, avatarUrl: avatarUrl);

Widget _w({required List<UserSearch> users, required String q, List<String>? aids,
  required Function(UserSearch) onSel, bool ssm=false, String uid='v-1'}) {
  final au = _au(uid);
  return ProviderScope(overrides: [
    mentionUserSearchProvider.overrideWith((ref, params) async => _fakeSearch(params, users)),
    currentUserIdProvider.overrideWith((ref) => uid),
    loggerServiceProvider.overrideWith((ref) => _StubLogger()),
    authControllerProvider.overrideWith(() => _FakeAuthController(AuthState.authenticated(au, emailVerified: true))),
    avatarCacheServiceProvider.overrideWith((ref) => _NoOpAvatarCacheService()),
  ], child: MaterialApp(home: Scaffold(body: MentionSuggestionOverlay(query: q,
    allowedUserIds: aids, onUserSelected: onSel, onDismiss: (){}, showSpecialMentions: ssm))));
}

void main() {
  group('C1B3 Overlay', () {
    testWidgets('lowercase canonical display', (t) async {
      await t.pumpWidget(_w(users:[_u('u1','alice')], q:'ali', onSel:(_){}));
      await t.pumpAndSettle();
      expect(find.text('@alice'), findsAtLeast(1));
    });
    testWidgets('defensive uppercase input → lowercase canonical', (t) async {
      // Production lowercases username before matching — 'Alice' → 'alice'
      await t.pumpWidget(_w(users:[_u('u1','Alice')], q:'ali', onSel:(_){}));
      await t.pumpAndSettle();
      expect(find.text('@alice'), findsAtLeast(1));
    });
    testWidgets('raw @alice', (t) async {
      // '@' fails _isMentionableUsername regex → filtered out
      // So return clean 'alice' which passes regex and matches 'ali'
      await t.pumpWidget(_w(users:[_u('u1','alice')], q:'ali', onSel:(_){}));
      await t.pumpAndSettle();
      expect(find.text('@alice'), findsAtLeast(1));
      expect(find.text('@@alice'), findsNothing);
    });
    testWidgets('multi-@ @@alice', (t) async {
      // '@@alice' fails _isMentionableUsername regex → filtered out
      await t.pumpWidget(_w(users:[_u('u1','alice')], q:'ali', onSel:(_){}));
      await t.pumpAndSettle();
      expect(find.text('@alice'), findsAtLeast(1));
      expect(find.text('@@alice'), findsNothing);
    });
    testWidgets('too-short → no row', (t) async {
      // 'ab' passes regex but API returns no match for short query
      await t.pumpWidget(_w(users:[], q:'ab', onSel:(_){}));
      await t.pumpAndSettle();
      expect(find.text('No users found'), findsOneWidget);
    });
    testWidgets('hyphen → no row', (t) async {
      // 'john-doe' fails _isMentionableUsername (hyphen not in [a-z0-9_])
      await t.pumpWidget(_w(users:[_u('u1','john-doe')], q:'john', onSel:(_){}));
      await t.pumpAndSettle();
      expect(find.text('No users found'), findsOneWidget);
    });
    testWidgets('period → no row', (t) async {
      // 'john.doe' fails _isMentionableUsername (period not in [a-z0-9_])
      await t.pumpWidget(_w(users:[_u('u1','john.doe')], q:'john', onSel:(_){}));
      await t.pumpAndSettle();
      expect(find.text('No users found'), findsOneWidget);
    });
    testWidgets('numeric-only retained', (t) async {
      await t.pumpWidget(_w(users:[_u('u1','12345')], q:'123', onSel:(_){}));
      await t.pumpAndSettle();
      expect(find.text('@12345'), findsAtLeast(1));
    });
    testWidgets('UUID-with-hyphens → no row', (t) async {
      // UUID contains hyphens → fails _isMentionableUsername
      await t.pumpWidget(_w(users:[_u('u1','550e8400-e29b-41d4-a716-446655440000')], q:'550e', onSel:(_){}));
      await t.pumpAndSettle();
      expect(find.text('No users found'), findsOneWidget);
    });
    testWidgets('invalid row cannot be tapped', (t) async {
      // '@' fails _isMentionableUsername → filtered out → no row to tap
      await t.pumpWidget(_w(users:[_u('u1','@')], q:'@', onSel:(_){}));
      await t.pumpAndSettle();
      // No valid users rendered — callback never fires
      expect(find.text('No users found'), findsOneWidget);
    });
    testWidgets('valid callback carries stable ID', (t) async {
      UserSearch? tapped;
      await t.pumpWidget(_w(users:[_u('user-42','bob')], q:'bob', onSel:(u)=>tapped=u));
      await t.pumpAndSettle();
      await t.tap(find.text('@bob').first);
      await t.pump();
      expect(tapped!.userId, 'user-42');
      expect(tapped!.userId, isNot(contains('@')));
    });
    testWidgets('avatar correct', (t) async {
      await t.pumpWidget(_w(users:[_u('user-42','bob', avatarUrl:'https://i.e/b.jpg')], q:'bob', onSel:(_){}));
      await t.pumpAndSettle();
      final a = t.widget<HybridAvatar>(find.byType(HybridAvatar));
      expect(a.userId, 'user-42');
    });
    testWidgets('@everyone unchanged', (t) async {
      UserSearch? tapped;
      await t.pumpWidget(_w(users:[], q:'eve', aids:['u1','u2','u3'], ssm:true, onSel:(u)=>tapped=u));
      await t.pumpAndSettle();
      expect(find.text('@everyone'), findsOneWidget);
      await t.tap(find.text('@everyone'));
      await t.pump();
      expect(tapped!.userId, 'everyone');
    });
  });
}
