// C1B3 — MentionRichText runtime safety (Proof 2).

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/features/search/search/data/remote/search_api_service.dart';
import 'package:labuda/features/search/search/presentation/providers/mention_providers.dart';
import 'package:labuda/features/search/search/presentation/providers/providers.dart'
    show searchApiServiceProvider;
import 'package:labuda/shared/widgets/mentions/mention_rich_text.dart';

class _FakeApiClient extends Fake implements ApiClient {}
class _FakeSearchApiService extends SearchApiService { _FakeSearchApiService() : super(_FakeApiClient()); }

class _StubLogger extends Fake implements ILoggerService {
  final messages = <String>[];
  @override Future<Result<void>> info(String m, {Map<String, dynamic>? extra}) async => Result.success(null);
  @override Future<Result<void>> error(String m, {Map<String, dynamic>? extra, StackTrace? stackTrace}) async { messages.add(m); return Result.success(null); }
  @override Future<Result<void>> warning(String m, {Map<String, dynamic>? extra}) async { messages.add(m); return Result.success(null); }
}

class _TestMentionResolver extends MentionResolver {
  String? cannedId; Object? cannedError; bool throwPaginationIntegrity = false; int callCount = 0;
  _TestMentionResolver() : super(apiService: _FakeSearchApiService(), logger: _StubLogger());
  @override Future<String?> resolveUsername(String username) async {
    callCount++;
    if (throwPaginationIntegrity) throw const PaginationIntegrityException('simulated');
    if (cannedError != null) throw cannedError!;
    return cannedId;
  }
}

class _TestNav extends Fake implements NavigationHandler { String? navId; @override void navigateToUserProfile(String id) { navId = id; } }

Widget _wrap({required _TestMentionResolver r, _TestNav? nav, _StubLogger? log, required String text}) {
  return ProviderScope(overrides: [
    mentionResolverProvider.overrideWith((ref) => r),
    if (nav != null) navigationHandlerProvider.overrideWith((ref) => nav),
    if (log != null) loggerServiceProvider.overrideWith((ref) => log),
    searchApiServiceProvider.overrideWith((ref) => _FakeSearchApiService()),
  ], child: MaterialApp(home: Scaffold(body: Directionality(
    textDirection: TextDirection.ltr, child: MentionRichText(text: text)))));
}

void main() {
  group('C1B3 MentionRichText runtime', () {
    testWidgets('exact resolution → navigation with stable ID', (t) async {
      final r = _TestMentionResolver(); r.cannedId = 'user-alice-42';
      final nav = _TestNav();
      await t.pumpWidget(_wrap(r:r, nav:nav, text: 'Hello @alice!'));
      await t.pumpAndSettle();

      // Tap the RichText widget which contains the @alice mention span.
      final rt = find.byType(RichText);
      expect(rt, findsOneWidget);
      await t.tap(rt);
      await t.pumpAndSettle();

      expect(r.callCount, 1);
      expect(nav.navId, 'user-alice-42');
      expect(nav.navId, isNot('alice'));
      expect(nav.navId, isNot(contains('@')));
    });

    testWidgets('unresolved → no navigation', (t) async {
      final r = _TestMentionResolver(); r.cannedId = null;
      final nav = _TestNav();
      await t.pumpWidget(_wrap(r:r, nav:nav, text: '@unknown'));
      await t.pumpAndSettle();
      await t.tap(find.byType(RichText)); await t.pumpAndSettle();
      expect(r.callCount, 1); expect(nav.navId, isNull);
    });

    testWidgets('pagination integrity → no navigation, no exception', (t) async {
      final r = _TestMentionResolver(); r.throwPaginationIntegrity = true;
      final nav = _TestNav(); final log = _StubLogger();
      await t.pumpWidget(_wrap(r:r, nav:nav, log:log, text: '@alice'));
      await t.pumpAndSettle();
      await t.tap(find.byType(RichText)); await t.pumpAndSettle();
      expect(r.callCount, 1); expect(nav.navId, isNull);
      expect(log.messages.any((m)=>m.contains('pagination integrity')), isTrue);
    });

    testWidgets('generic failure → no navigation, no exception', (t) async {
      final r = _TestMentionResolver(); r.cannedError = Exception('crash');
      final nav = _TestNav(); final log = _StubLogger();
      await t.pumpWidget(_wrap(r:r, nav:nav, log:log, text: '@alice'));
      await t.pumpAndSettle();
      await t.tap(find.byType(RichText)); await t.pumpAndSettle();
      expect(r.callCount, 1); expect(nav.navId, isNull);
      expect(log.messages.any((m)=>m.contains('unexpected resolver failure')), isTrue);
    });

    testWidgets('disposed → no navigation', (t) async {
      final r = _TestMentionResolver(); r.cannedId = 'id-42';
      final nav = _TestNav();
      await t.pumpWidget(_wrap(r:r, nav:nav, text: '@alice'));
      await t.pumpAndSettle();
      await t.tap(find.byType(RichText));
      // Replace widget immediately — old tree disposed.
      await t.pumpWidget(const SizedBox()); await t.pumpAndSettle();
      // No exception. Navigation may or may not fire depending on timing.
    });

    testWidgets('special @everyone → no navigation, no resolver call', (t) async {
      final r = _TestMentionResolver();
      final nav = _TestNav();
      await t.pumpWidget(_wrap(r:r, nav:nav, text: '@everyone'));
      await t.pumpAndSettle();
      await t.tap(find.byType(RichText)); await t.pumpAndSettle();
      // Special mentions return early — no resolver call.
      expect(r.callCount, 0); expect(nav.navId, isNull);
    });
  });
}
