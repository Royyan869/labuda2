import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/features/home/presentation/widgets/main_app_bar.dart';

class _FakeAuthController extends AuthController {
  @override
  AuthState build() => const AuthState.unauthenticated();
}

class _Navigation extends Fake implements NavigationHandler {
  final calls = <String>[];
  VoidCallback? onSearch;

  @override
  void navigateToSearch() {
    calls.add('/search');
    onSearch?.call();
  }

  @override
  void navigateToSignIn() => calls.add('sign-in');

  @override
  void navigateToSavedItems() => calls.add('saved');

  @override
  void navigateToMyBids() => calls.add('bids');

  @override
  void navigateToChat() => calls.add('chat');

  @override
  void navigateToNotifications() => calls.add('notifications');
}

class _SearchPage extends StatelessWidget {
  const _SearchPage();

  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: AppBar(title: const Text('Search')),
    body: const SizedBox.expand(),
  );
}

/// Production-shaped host: real AppTheme (centerTitle:true) + a Scaffold drawer,
/// mirroring MainScreen. This is what caught the missing auto-implied leading.
Widget _host({
  required _Navigation navigation,
  required PreferredSizeWidget appBar,
  Widget? body,
}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(_FakeAuthController.new),
      navigationHandlerProvider.overrideWithValue(navigation),
    ],
    child: MaterialApp(
      theme: AppTheme.lightTheme,
      home: Scaffold(
        drawer: const Drawer(),
        appBar: appBar,
        body: body ?? const SizedBox.expand(),
      ),
    ),
  );
}

void main() {
  for (final width in [360.0, 393.0, 412.0]) {
    testWidgets('MainAppBar geometry at $width dp', (tester) async {
      final navigation = _Navigation();
      tester.view.physicalSize = Size(width, 800);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      await tester.pumpWidget(
        _host(navigation: navigation, appBar: const MainAppBar()),
      );
      await tester.pump();

      // Exactly six action buttons: the Scaffold drawer must NOT inject a
      // second, zero-width auto-implied leading hamburger.
      final buttons = find.byType(IconButton);
      expect(buttons, findsNWidgets(6));
      expect(tester.takeException(), isNull);

      final buttonHosts = tester
          .widgetList<SizedBox>(find.byType(SizedBox))
          .where((box) => box.width == 56 && box.height == 56)
          .toList();
      expect(buttonHosts, hasLength(6));
      final rects = buttonHosts
          .map((box) => tester.getRect(find.byWidget(box)))
          .toList();
      expect(
        rects.every(
          (rect) =>
              (rect.width - 56).abs() < 0.01 && (rect.height - 56).abs() < 0.01,
        ),
        isTrue,
      );
      expect(rects.every((rect) => rect.top.abs() < 0.01), isTrue);
      expect(find.byType(OverflowBar), findsNothing);
      expect(rects.last.right <= width, isTrue);

      final gaps = [
        for (var i = 0; i < rects.length - 1; i++)
          rects[i + 1].left - rects[i].right,
      ];
      expect(gaps.every((gap) => (gap - gaps.first).abs() < 0.01), isTrue);
      final spacerRects = tester
          .widgetList<Spacer>(find.byType(Spacer))
          .map((spacer) => tester.getRect(find.byWidget(spacer)))
          .toList();
      expect(spacerRects, hasLength(5));
      expect(
        spacerRects.every((rect) => (rect.width - gaps.first).abs() < 0.01),
        isTrue,
      );

      await tester.tap(find.byIcon(Icons.search));
      expect(navigation.calls, contains('/search'));
    });
  }

  testWidgets('hamburger glyph stays compact (24dp) inside a 56dp button', (
    tester,
  ) async {
    final navigation = _Navigation();
    await tester.pumpWidget(
      _host(navigation: navigation, appBar: const MainAppBar()),
    );
    await tester.pump();

    final menuIcon = find.byIcon(Icons.menu);
    expect(menuIcon, findsOneWidget);
    final iconSize = tester.getSize(menuIcon);
    expect(iconSize, const Size(24, 24));

    final menuButton = tester.getRect(
      find.ancestor(of: menuIcon, matching: find.byType(IconButton)),
    );
    expect(menuButton.size, const Size(56, 56));
    // No second (auto-implied) hamburger painted at the left edge.
    expect(find.byIcon(Icons.menu), findsOneWidget);
  });

  testWidgets('push Search then pop Home has no layout exception', (
    tester,
  ) async {
    final navigation = _Navigation();
    final navigatorKey = GlobalKey<NavigatorState>();
    navigation.onSearch = () => navigatorKey.currentState!.push(
      MaterialPageRoute<void>(builder: (_) => const _SearchPage()),
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authControllerProvider.overrideWith(_FakeAuthController.new),
          navigationHandlerProvider.overrideWithValue(navigation),
        ],
        child: MaterialApp(
          theme: AppTheme.lightTheme,
          navigatorKey: navigatorKey,
          home: Scaffold(
            drawer: const Drawer(),
            appBar: const MainAppBar(),
            body: const SizedBox.expand(),
          ),
        ),
      ),
    );
    await tester.pump();

    await tester.tap(find.byIcon(Icons.search));
    for (var i = 0; i < 12; i++) {
      await tester.pump(const Duration(milliseconds: 30));
      expect(tester.takeException(), isNull);
    }
    expect(find.text('Search'), findsOneWidget);

    navigatorKey.currentState!.pop();
    for (var i = 0; i < 12; i++) {
      await tester.pump(const Duration(milliseconds: 30));
      expect(tester.takeException(), isNull);
    }
    // Let the pop transition finish so the Search route is gone, then confirm
    // Home is intact and still renders exactly its six actions.
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
    expect(find.byIcon(Icons.menu), findsOneWidget);
    expect(find.byType(IconButton), findsNWidgets(6));
  });
}
