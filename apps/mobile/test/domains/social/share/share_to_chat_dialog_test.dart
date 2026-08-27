import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/src/auth/app_role.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_resource_occurrence_request.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/new_chat_user_search_provider.dart';
import 'package:labuda/domains/social/share/domain/entities/share_destination.dart';
import 'package:labuda/domains/social/share/domain/entities/share_target.dart';
import 'package:labuda/domains/social/share/presentation/widgets/share_to_chat_dialog.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/features/search/search/domain/entities/user_search.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart';

const _chatId = '11111111-1111-1111-1111-111111111111';
const _currentUserId = '22222222-2222-2222-2222-222222222222';
const _recipientUserId = '33333333-3333-3333-3333-333333333333';

class _FakeAuthController extends AuthController {
  @override
  AuthState build() {
    final now = DateTime.utc(2026, 8, 9, 8);
    final user = AuthUser(
      id: _currentUserId,
      createdAt: now,
      updatedAt: now,
      email: 'me@example.com',
      username: 'sender',
      isEmailVerified: true,
      accountStatus: AccountStatus.active,
      hasSellerProfile: false,
      hasMarketAuthority: false,
      sellerSubscriptionStatus: 'none',
      roles: const [UserRole.user],
      provider: ShonaAuthProvider.email,
      lifecycle: ContentLifecycle.active,
    );
    return AuthState.authenticated(user, emailVerified: true);
  }
}

class _FakeChatListNotifier extends ChatList {
  int getOrCreateCalls = 0;
  List<String>? lastParticipantIds;

  @override
  ChatListState build() => const ChatListState();

  @override
  Future<Chat?> getOrCreateChat({
    required String userId,
    required String otherUserId,
  }) async {
    getOrCreateCalls += 1;
    lastParticipantIds = [userId, otherUserId];
    final now = DateTime.utc(2026, 8, 9, 8);
    return Chat(
      id: _chatId,
      participantIds: [userId, otherUserId],
      participantNames: const {
        _currentUserId: 'sender',
        _recipientUserId: 'recipient',
      },
      participantAvatars: const {},
      participantLifecycles: const {
        _recipientUserId: ContentLifecycle.active,
      },
      createdAt: now,
      status: ChatStatus.active,
    );
  }
}

class _FakeChatDetailNotifier extends ChatDetail {
  int sendCalls = 0;
  String? lastSenderId;
  String? lastSenderName;
  String? lastContent;
  MessageType? lastType;
  ChatResourceOccurrenceRequest? lastResourceOccurrence;

  @override
  ChatDetailState build(String chatId) {
    return ChatDetailState(
      chat: Chat(
        id: chatId,
        participantIds: const [_currentUserId, _recipientUserId],
        participantNames: const {
          _currentUserId: 'sender',
          _recipientUserId: 'recipient',
        },
        participantAvatars: const {},
        participantLifecycles: const {
          _recipientUserId: ContentLifecycle.active,
        },
        createdAt: DateTime.utc(2026, 8, 9),
        status: ChatStatus.active,
      ),
    );
  }

  @override
  Future<Message?> sendMessage({
    required String senderId,
    required String senderName,
    required String content,
    MessageType type = MessageType.text,
    List<String> mediaUrls = const [],
    List<String> mediaAssetIds = const [],
    String? replyToId,
    String? replyToMessageId,
    List<String> mentionedUserIds = const [],
    String? idempotencyKey,
    ChatResourceOccurrenceRequest? resourceOccurrence,
    Map<String, dynamic>? workflowAttachment,
  }) async {
    sendCalls += 1;
    lastSenderId = senderId;
    lastSenderName = senderName;
    lastContent = content;
    lastType = type;
    lastResourceOccurrence = resourceOccurrence;
    return Message(
      id: 'sent-message',
      chatId: _chatId,
      senderId: senderId,
      senderName: senderName,
      content: content,
      createdAt: DateTime.utc(2026, 8, 9, 9),
      status: MessageStatus.sent,
      mentionedUserIds: const [],
      deletedBy: const [],
    );
  }
}

ProviderScope _wrap({
  required Widget child,
  required _FakeChatListNotifier chatListNotifier,
  required _FakeChatDetailNotifier chatDetailNotifier,
}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(_FakeAuthController.new),
      currentUserIdProvider.overrideWith((ref) => _currentUserId),
      chatListProvider.overrideWith(() => chatListNotifier),
      chatDetailProvider(_chatId).overrideWith(() => chatDetailNotifier),
      newChatUserSearchProvider.overrideWith((ref, query) async {
        if (query.trim().isEmpty) return <UserSearch>[];
        return const [
          UserSearch(
            userId: _recipientUserId,
            username: 'recipient',
          ),
        ];
      }),
    ],
    child: MaterialApp(
      home: Scaffold(
        body: Builder(
          builder: (context) {
            return child;
          },
        ),
      ),
    ),
  );
}

void main() {
  test('share-to-chat destinations expose the internal chat entry', () {
    expect(ShareDestination.internalDestinations, contains(ShareDestination.sendToChat));
  });

  test('share-to-chat resource mapping is identity-only', () {
    final cases = <(ShareTarget, ChatResourceOccurrenceResourceType)>[
      (
        const ShareTarget(
          id: _currentUserId,
          type: ExternalShareType.profile,
          title: 'Profile',
          description: '',
        ),
        ChatResourceOccurrenceResourceType.profile,
      ),
      (
        const ShareTarget(
          id: _recipientUserId,
          type: ExternalShareType.content,
          title: 'Content',
          description: '',
        ),
        ChatResourceOccurrenceResourceType.content,
      ),
      (
        const ShareTarget(
          id: _chatId,
          type: ExternalShareType.listing,
          title: 'Listing',
          description: '',
        ),
        ChatResourceOccurrenceResourceType.fixedPriceSale,
      ),
      (
        const ShareTarget(
          id: _chatId,
          type: ExternalShareType.auction,
          title: 'Auction',
          description: '',
        ),
        ChatResourceOccurrenceResourceType.auction,
      ),
    ];

    for (final entry in cases) {
      final request = buildShareToChatRequest(entry.$1);
      expect(request.operation, ChatResourceOccurrenceOperation.shareToChat);
      expect(request.resourceType, entry.$2);
      expect(request.resourceId, entry.$1.id);
      expect(
        request.toJson().keys.toSet(),
        {'operation', 'resource_type', 'resource_id'},
      );
    }
  });

  testWidgets('share-to-chat composer sends optional text and resource', (
    tester,
  ) async {
    final chatListNotifier = _FakeChatListNotifier();
    final chatDetailNotifier = _FakeChatDetailNotifier();
    const target = ShareTarget(
      id: _recipientUserId,
      type: ExternalShareType.content,
      title: 'Shared content',
      description: 'Description',
    );

    await tester.pumpWidget(
      _wrap(
        chatListNotifier: chatListNotifier,
        chatDetailNotifier: chatDetailNotifier,
        child: Builder(
          builder: (context) {
            return TextButton(
              onPressed: () {
                unawaited(
                  ShareToChatDialog.show(context: context, target: target),
                );
              },
              child: const Text('open'),
            );
          },
        ),
      ),
    );

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    await tester.enterText(find.byType(TextField).first, 'recipient');
    await tester.pumpAndSettle();
    final recipientTile = find.ancestor(
      of: find.text('@recipient'),
      matching: find.byType(ListTile),
    );
    await tester.ensureVisible(recipientTile);
    await tester.tap(recipientTile);
    await tester.pumpAndSettle();

    await tester.enterText(find.byType(TextField).at(1), 'Hello there');
    await tester.pump();

    await tester.tap(find.text('Send'));
    await tester.pumpAndSettle();

    expect(chatListNotifier.getOrCreateCalls, 1);
    expect(
      chatListNotifier.lastParticipantIds,
      containsAllInOrder([_currentUserId, _recipientUserId]),
    );
    expect(chatDetailNotifier.sendCalls, 1);
    expect(chatDetailNotifier.lastSenderId, _currentUserId);
    expect(chatDetailNotifier.lastSenderName, 'sender');
    expect(chatDetailNotifier.lastContent, 'Hello there');
    expect(chatDetailNotifier.lastType, MessageType.text);
    expect(
      chatDetailNotifier.lastResourceOccurrence,
      buildShareToChatRequest(target),
    );
    expect(find.text('Send to Chat'), findsNothing);
  });

  testWidgets('share-to-chat cancel sends nothing', (tester) async {
    final chatListNotifier = _FakeChatListNotifier();
    final chatDetailNotifier = _FakeChatDetailNotifier();
    const target = ShareTarget(
      id: _recipientUserId,
      type: ExternalShareType.profile,
      title: 'Shared profile',
      description: 'Description',
    );

    await tester.pumpWidget(
      _wrap(
        chatListNotifier: chatListNotifier,
        chatDetailNotifier: chatDetailNotifier,
        child: Builder(
          builder: (context) {
            return TextButton(
              onPressed: () {
                unawaited(
                  ShareToChatDialog.show(context: context, target: target),
                );
              },
              child: const Text('open'),
            );
          },
        ),
      ),
    );

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();

    expect(chatListNotifier.getOrCreateCalls, 0);
    expect(chatDetailNotifier.sendCalls, 0);
    expect(find.text('Send to Chat'), findsNothing);
  });

  testWidgets('share-to-chat resource-only send uses system message', (
    tester,
  ) async {
    final chatListNotifier = _FakeChatListNotifier();
    final chatDetailNotifier = _FakeChatDetailNotifier();
    const target = ShareTarget(
      id: _recipientUserId,
      type: ExternalShareType.auction,
      title: 'Shared auction',
      description: 'Description',
    );

    await tester.pumpWidget(
      _wrap(
        chatListNotifier: chatListNotifier,
        chatDetailNotifier: chatDetailNotifier,
        child: Builder(
          builder: (context) {
            return TextButton(
              onPressed: () {
                unawaited(
                  ShareToChatDialog.show(context: context, target: target),
                );
              },
              child: const Text('open'),
            );
          },
        ),
      ),
    );

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    await tester.enterText(find.byType(TextField).first, 'recipient');
    await tester.pumpAndSettle();
    final recipientTile = find.ancestor(
      of: find.text('@recipient'),
      matching: find.byType(ListTile),
    );
    await tester.ensureVisible(recipientTile);
    await tester.tap(recipientTile);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Send'));
    await tester.pumpAndSettle();

    expect(chatDetailNotifier.lastContent, isEmpty);
    expect(chatDetailNotifier.lastType, MessageType.system);
    expect(
      chatDetailNotifier.lastResourceOccurrence,
      buildShareToChatRequest(target),
    );
  });
}
