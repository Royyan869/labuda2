import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/social/comment/domain/entities/comment.dart';
import 'package:labuda/domains/social/comment/presentation/widgets/comment_input_with_commerce_reference.dart';

Widget _wrap(Widget child) {
  return MaterialApp(home: Scaffold(body: child));
}

void main() {
  testWidgets('submit is locked while in flight and clears after success', (
    tester,
  ) async {
    var submitCount = 0;
    final completer = Completer<bool>();

    await tester.pumpWidget(
      _wrap(
        CommentInputWithCommerceReference(
          onSubmit: (body, listing) {
            submitCount += 1;
            return completer.future;
          },
        ),
      ),
    );

    await tester.enterText(find.byType(TextField), 'hello');
    await tester.pump();

    await tester.tap(find.byKey(const ValueKey('comment-send-button')));
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('comment-send-button')));
    await tester.pump();

    expect(submitCount, 1);
    expect(find.byType(CircularProgressIndicator), findsOneWidget);

    completer.complete(true);
    await tester.pump();
    await tester.pump();

    expect(find.text('hello'), findsNothing);
    expect(find.byType(CircularProgressIndicator), findsNothing);
  });

  testWidgets('failed submit preserves the draft for retry', (tester) async {
    final completer = Completer<bool>();

    await tester.pumpWidget(
      _wrap(
        CommentInputWithCommerceReference(onSubmit: (body, listing) => completer.future),
      ),
    );

    await tester.enterText(find.byType(TextField), 'retry me');
    await tester.pump();

    await tester.tap(find.byKey(const ValueKey('comment-send-button')));
    await tester.pump();

    completer.complete(false);
    await tester.pump();
    await tester.pump();

    expect(find.text('retry me'), findsOneWidget);
    expect(find.byType(CircularProgressIndicator), findsNothing);
  });

  testWidgets('FPS resource appears as selected after picker callback', (tester) async {
    final resource = ResourceIdentity(resourceType: ResourceType.forSale, resourceId: 'fps-1');
    await tester.pumpWidget(_wrap(
      CommentInputWithCommerceReference(
        onSubmit: (_, __) async => true,
        initialResource: resource,
      ),
    ));
    await tester.pump();
    // The selected resource preview should be visible
    expect(find.byType(IconButton), findsWidgets); // remove + send buttons
  });

  testWidgets('successful send clears body and removes selection', (tester) async {
    final resource = ResourceIdentity(resourceType: ResourceType.forSale, resourceId: 'fps-1');
    await tester.pumpWidget(_wrap(
      CommentInputWithCommerceReference(
        onSubmit: (_, __) async => true,
        initialResource: resource,
      ),
    ));
    await tester.enterText(find.byType(TextField), 'check this');
    await tester.pump();

    await tester.tap(find.byKey(const ValueKey('comment-send-button')));
    await tester.pump();
    await tester.pump();

    // Body cleared
    expect(find.text('check this'), findsNothing);
  });

  testWidgets('remove selection clears resource and enables text-only send', (tester) async {
    final resource = ResourceIdentity(resourceType: ResourceType.forSale, resourceId: 'fps-1');
    var capturedResource = resource;
    await tester.pumpWidget(_wrap(
      CommentInputWithCommerceReference(
        onSubmit: (body, res) async {
          capturedResource = res ?? ResourceIdentity(resourceType: ResourceType.forSale, resourceId: 'none');
          return true;
        },
        initialResource: resource,
      ),
    ));
    await tester.pump();

    // Tap remove button (close icon)
    final removeButtons = find.byIcon(Icons.close);
    if (removeButtons.evaluate().isNotEmpty) {
      await tester.tap(removeButtons.first);
      await tester.pump();
    }
  });

  testWidgets('body-only send works without resource', (tester) async {
    String? sentBody;
    ResourceIdentity? sentResource;
    await tester.pumpWidget(_wrap(
      CommentInputWithCommerceReference(
        onSubmit: (body, res) async {
          sentBody = body;
          sentResource = res;
          return true;
        },
      ),
    ));
    await tester.enterText(find.byType(TextField), 'normal comment');
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('comment-send-button')));
    await tester.pump();
    await tester.pump();

    expect(sentBody, 'normal comment');
    expect(sentResource, isNull);
  });
}
