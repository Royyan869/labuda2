import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
// PHASE 7-8 CUTOVER: Using chat_refactor screens
import 'package:labuda/domains/chat/chat/chat.dart';
import 'package:labuda/core/src/router/route_paths.dart';
import 'base_module.dart';

/// Chat Module - Routes dan dependencies untuk fitur chat
/// PHASE 7-8 CUTOVER: Now using chat_refactor screens
class ChatModule implements BaseModule {
  @override
  String get moduleName => 'Chat';

  @override
  List<GoRoute> get routes => _chatRoutes;

  late final List<GoRoute> _chatRoutes;

  @override
  Future<void> initialize() async {
    // Chat module dependencies sudah di-setup di core/di/di.dart
    // Initialize routes
    _chatRoutes = _buildRoutes();
  }

  @override
  void dispose() {
    // Clean up resources if needed
  }

  List<GoRoute> _buildRoutes() {
    return [
      // Chat list route - PHASE 7-8: Using chat_refactor
      GoRoute(
        path: RoutePaths.chat,
        name: RouteNames.chat,
        builder: (context, state) => const ChatListScreen(),
      ),

      // New chat route - Using chat_refactor screen
      GoRoute(
        path: RoutePaths.newChat,
        name: RouteNames.newChat,
        builder: (context, state) => const NewChatScreen(),
      ),

      // Chat conversation route - PHASE 7-8: Using chat_refactor
      GoRoute(
        path: RoutePaths.chatConversation,
        name: RouteNames.chatConversation,
        builder: (context, state) {
          final conversationId = state.pathParameters['conversationId'];

          if (conversationId == null || conversationId.isEmpty) {
            return const Scaffold(
              body: Center(child: Text('Invalid conversation ID')),
            );
          }

          // Extract initialMessage from extra data if provided
          final extra = state.extra as Map<String, dynamic>?;
          final initialMessage = extra?['initialMessage'] as String?;

          return ChatDetailScreen(
            chatId: conversationId,
            initialMessage: initialMessage,
          );
        },
      ),
    ];
  }

  @override
  void registerRoutes(List<GoRoute> routes) {
    routes.addAll(_chatRoutes);
  }
}
