import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/domains/social/share/domain/entities/share_destination.dart';
import 'package:labuda/domains/social/share/domain/entities/share_target.dart';
import 'package:labuda/domains/social/share/presentation/providers/share_composer_state.dart';
import 'package:labuda/domains/social/share/presentation/widgets/share_as_post_dialog.dart';
import 'package:labuda/domains/social/share/presentation/widgets/share_bottom_sheet.dart';
import 'package:labuda/domains/social/share/presentation/widgets/share_to_chat_dialog.dart';
import 'package:labuda/features/search/search/domain/entities/user_search.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/new_chat_user_search_provider.dart';

ProviderScope _wrap(Widget child) {
  final router = GoRouter(
    initialLocation: '/',
    routes: [
      GoRoute(
        path: '/',
        builder: (context, state) => Scaffold(body: child),
      ),
      GoRoute(
        path: '/chat/:chatId',
        builder: (context, state) => const Scaffold(body: SizedBox.shrink()),
      ),
    ],
  );

  return ProviderScope(
    overrides: [
      newChatUserSearchProvider.overrideWith(
        (ref, query) async => const <UserSearch>[],
      ),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

void main() {
  test('share composer state preserves target and destination selection', () {
    const target = ShareTarget(
      id: 'content-1',
      type: ExternalShareType.content,
      title: 'Shared content',
      description: 'Description',
    );

    final state = ShareComposerState(target: target);
    final updated = state.copyWith(
      selectedDestination: ShareDestinationType.sendToChat,
    );

    expect(updated.target, target);
    expect(updated.selectedDestination, ShareDestinationType.sendToChat);
  });

  testWidgets(
    'share sheet reuses one optional text controller across chat and feed',
    (tester) async {
      const target = ShareTarget(
        id: 'content-1',
        type: ExternalShareType.content,
        title: 'Shared content',
        description: 'Description',
      );

      await tester.pumpWidget(
        _wrap(
          Builder(
            builder: (context) {
              return TextButton(
                onPressed: () {
                  unawaited(
                    ShareBottomSheet.show(
                      context: context,
                      target: target,
                      canSharePost: true,
                    ),
                  );
                },
                child: const Text('open share'),
              );
            },
          ),
        ),
      );

      await tester.tap(find.text('open share'));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Send to Chat'));
      await tester.pumpAndSettle();

      await tester.enterText(find.byType(TextField).at(1), 'Shared note');
      await tester.pump();

      await tester.tap(find.text('Cancel'));
      await tester.pumpAndSettle();

      await tester.tap(find.text('To Feed'));
      await tester.pumpAndSettle();

      final captionField = tester.widget<TextField>(find.byType(TextField));
      expect(captionField.controller?.text, 'Shared note');

      await tester.tap(find.text('Cancel'));
      await tester.pumpAndSettle();
    },
  );

  testWidgets('share to chat still opens with the canonical target', (
    tester,
  ) async {
    const target = ShareTarget(
      id: 'content-2',
      type: ExternalShareType.content,
      title: 'Shared content',
      description: 'Description',
    );

    await tester.pumpWidget(
      _wrap(
        Builder(
          builder: (context) {
            return TextButton(
              onPressed: () {
                unawaited(
                  ShareToChatDialog.show(context: context, target: target),
                );
              },
              child: const Text('open chat'),
            );
          },
        ),
      ),
    );

    await tester.tap(find.text('open chat'));
    await tester.pumpAndSettle();

    expect(find.text('Send to Chat'), findsOneWidget);
    expect(find.text('Recipient'), findsOneWidget);

    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();
  });

  testWidgets('share to feed dialog still accepts the canonical target', (
    tester,
  ) async {
    const target = ShareTarget(
      id: 'content-3',
      type: ExternalShareType.content,
      title: 'Shared content',
      description: 'Description',
    );

    await tester.pumpWidget(
      _wrap(
        Builder(
          builder: (context) {
            return TextButton(
              onPressed: () {
                unawaited(
                  ShareAsPostDialog.show(context: context, target: target),
                );
              },
              child: const Text('open feed'),
            );
          },
        ),
      ),
    );

    await tester.tap(find.text('open feed'));
    await tester.pumpAndSettle();

    expect(find.text('Share to Feed'), findsOneWidget);
    final captionField = tester.widget<TextField>(find.byType(TextField));
    expect(captionField.controller, isNotNull);

    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();
  });
}
