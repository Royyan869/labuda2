import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/widgets/popup_more_options_button.dart';

/// Shared-widget behavioral contract for PopupMoreOptionsButton Delete action.
///
/// Canonical rule: an optional action MUST NOT be rendered when its callback
/// is null.  This test locks that invariant for the Delete item so that a
/// future regression cannot reintroduce a visible-but-dead Delete across any
/// content type.
void main() {
  group('PopupMoreOptionsButton Delete rendering', () {
    testWidgets('hides Delete when onDelete is null (even for creator)', (
      tester,
    ) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Builder(
              builder: (context) {
                return PopupMoreOptionsButton(
                  isCreator: true,
                  isDeleting: false,
                  contentType: PopupMoreOptionsContentType.auction,
                  onDelete: null,
                );
              },
            ),
          ),
        ),
      );

      // Open the popup menu.
      await tester.tap(find.byIcon(Icons.more_vert));
      await tester.pumpAndSettle();

      // Delete MUST NOT be present.
      expect(find.text('Delete'), findsNothing);
      expect(find.byIcon(Icons.delete_outline), findsNothing);
    });

    testWidgets('shows Delete and invokes callback when onDelete is non-null', (
      tester,
    ) async {
      var called = false;

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Builder(
              builder: (context) {
                return PopupMoreOptionsButton(
                  isCreator: true,
                  isDeleting: false,
                  contentType: PopupMoreOptionsContentType.auction,
                  onDelete: () => called = true,
                );
              },
            ),
          ),
        ),
      );

      // Open the popup menu.
      await tester.tap(find.byIcon(Icons.more_vert));
      await tester.pumpAndSettle();

      // Delete MUST be visible.
      expect(find.text('Delete'), findsOneWidget);

      // Tap Delete.
      await tester.tap(find.text('Delete'));
      await tester.pumpAndSettle();

      // Callback MUST have been invoked exactly once.
      expect(called, isTrue);
    });
  });
}
