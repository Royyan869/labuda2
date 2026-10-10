import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/features/home/home.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/shared/widgets/empty_state.dart';

/// A feed that has completed with zero items → Home renders its first-use
/// empty state (never loading, never error).
class _EmptyFeedNotifier extends FeedNotifier {
  @override
  FeedState build() => const FeedState();
}

Widget _app({
  required Size size,
  double keyboardInset = 0,
  GlobalKey<NavigatorState>? navigatorKey,
}) {
  return ProviderScope(
    overrides: [feedProvider.overrideWith(_EmptyFeedNotifier.new)],
    child: MaterialApp(
      theme: AppTheme.lightTheme,
      navigatorKey: navigatorKey,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: MediaQuery(
        data: MediaQueryData(
          size: size,
          viewInsets: EdgeInsets.only(bottom: keyboardInset),
        ),
        child: const Scaffold(body: HomeScreen()),
      ),
    ),
  );
}

void _setSurface(WidgetTester tester, Size size) {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
}

void main() {
  testWidgets('empty state is centred when there is room, no overflow', (
    tester,
  ) async {
    const size = Size(412, 915);
    _setSurface(tester, size);

    await tester.pumpWidget(_app(size: size));
    await tester.pump();

    expect(tester.takeException(), isNull);
    expect(find.byType(EmptyState), findsOneWidget);
    expect(find.text('Buat Konten'), findsOneWidget);

    final empty = tester.getRect(find.byType(EmptyState));
    final viewport = tester.getRect(find.byType(SingleChildScrollView));
    expect((empty.center.dy - viewport.center.dy).abs() < 2.0, isTrue);
  });

  testWidgets(
    'no RenderFlex overflow at 360x640 with viewInsets.bottom = 300',
    (tester) async {
      const size = Size(360, 640);
      _setSurface(tester, size);

      await tester.pumpWidget(_app(size: size, keyboardInset: 300));
      await tester.pump();

      expect(tester.takeException(), isNull);
      expect(find.byType(EmptyState), findsOneWidget);
      // Secondary action stays reachable (pinned outside the scrolling state).
      expect(find.text('Buat Konten'), findsOneWidget);
    },
  );

  testWidgets('primary action stays reachable by scrolling when cramped', (
    tester,
  ) async {
    const size = Size(360, 640);
    _setSurface(tester, size);

    await tester.pumpWidget(_app(size: size, keyboardInset: 300));
    await tester.pump();
    expect(tester.takeException(), isNull);

    final primary = find.byType(FilledButton);
    expect(primary, findsOneWidget);
    await tester.ensureVisible(primary);
    await tester.pump();
    expect(tester.takeException(), isNull);
  });

  testWidgets('narrow width (text wrapping) stays overflow-free', (
    tester,
  ) async {
    const size = Size(320, 640);
    _setSurface(tester, size);

    await tester.pumpWidget(_app(size: size, keyboardInset: 240));
    await tester.pump();

    expect(tester.takeException(), isNull);
    expect(find.byType(EmptyState), findsOneWidget);
    expect(find.text('Buat Konten'), findsOneWidget);
  });

  testWidgets(
    'push Search then pop Home over the real empty state has no exception',
    (tester) async {
      const size = Size(412, 915);
      _setSurface(tester, size);
      final navigatorKey = GlobalKey<NavigatorState>();

      await tester.pumpWidget(
        _app(size: size, keyboardInset: 300, navigatorKey: navigatorKey),
      );
      await tester.pump();
      expect(tester.takeException(), isNull);

      navigatorKey.currentState!.push(
        MaterialPageRoute<void>(
          builder: (_) => Scaffold(
            appBar: AppBar(title: const Text('Search')),
            body: const SizedBox.expand(),
          ),
        ),
      );
      for (var i = 0; i < 12; i++) {
        await tester.pump(const Duration(milliseconds: 30));
        expect(tester.takeException(), isNull);
      }

      navigatorKey.currentState!.pop();
      for (var i = 0; i < 12; i++) {
        await tester.pump(const Duration(milliseconds: 30));
        expect(tester.takeException(), isNull);
      }
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
      expect(find.byType(EmptyState), findsOneWidget);
      expect(find.text('Buat Konten'), findsOneWidget);
    },
  );
}
