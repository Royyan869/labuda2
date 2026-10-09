import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/features/home/presentation/widgets/main_app_bar.dart';

class _FakeAuthController extends AuthController {
  @override
  AuthState build() => const AuthState.unauthenticated();
}

class _Navigation extends Fake implements NavigationHandler {
  final calls = <String>[];

  @override
  void navigateToSearch() => calls.add('/search');

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

void main() {
  for (final width in [360.0, 393.0, 412.0]) {
    testWidgets('MainAppBar geometry at $width dp', (tester) async {
      final navigation = _Navigation();
      tester.view.physicalSize = Size(width, 800);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authControllerProvider.overrideWith(_FakeAuthController.new),
            navigationHandlerProvider.overrideWithValue(navigation),
          ],
          child: SizedBox(
            width: width,
            height: 800,
            child: const MaterialApp(home: Scaffold(appBar: MainAppBar())),
          ),
        ),
      );
      await tester.pump();

      final buttons = tester
          .widgetList<IconButton>(find.byType(IconButton))
          .toList();
      expect(buttons, hasLength(6));
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
}
