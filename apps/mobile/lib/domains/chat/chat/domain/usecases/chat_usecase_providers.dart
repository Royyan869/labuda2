import 'package:labuda/domains/chat/chat/data/chat_providers.dart';
import 'package:labuda/domains/chat/chat/domain/usecases/get_chat_usecase.dart';
import 'package:labuda/domains/chat/chat/domain/usecases/get_messages_usecase.dart';
import 'package:labuda/domains/chat/chat/domain/usecases/send_message_usecase.dart';
import 'package:labuda/domains/chat/chat/domain/usecases/mark_messages_read_usecase.dart';
import 'package:labuda/domains/chat/chat/domain/usecases/manage_presence_usecase.dart';
import 'package:labuda/domains/chat/chat/domain/usecases/link_order_to_chat_usecase.dart';
import 'package:labuda/domains/chat/chat/domain/usecases/get_or_create_commerce_chat_usecase.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Chat UseCase Providers
///
/// Provides usecase instances with repository injection.

// Get Chat UseCase Provider
final getChatUseCaseProvider = Provider<GetChatUseCase>((ref) {
  final repository = ref.watch(chatRepositoryProvider);
  return GetChatUseCase(repository);
});

// Get Messages UseCase Provider
final getMessagesUseCaseProvider = Provider<GetMessagesUseCase>((ref) {
  final repository = ref.watch(chatRepositoryProvider);
  return GetMessagesUseCase(repository);
});

// Send Message UseCase Provider
final sendMessageUseCaseProvider = Provider<SendMessageUseCase>((ref) {
  final repository = ref.watch(chatRepositoryProvider);
  return SendMessageUseCase(repository);
});

// Mark Messages Read UseCase Provider
final markMessagesReadUseCaseProvider = Provider<MarkMessagesAsReadUseCase>((
  ref,
) {
  final repository = ref.watch(chatRepositoryProvider);
  return MarkMessagesAsReadUseCase(repository);
});

// Manage Presence UseCase Provider
final managePresenceUseCaseProvider = Provider<ManagePresenceUseCase>((ref) {
  final repository = ref.watch(chatRepositoryProvider);
  return ManagePresenceUseCase(repository);
});

// Link Order to Chat UseCase Provider
final linkOrderToChatUseCaseProvider = Provider<LinkOrderToChatUseCase>((ref) {
  final repository = ref.watch(chatRepositoryProvider);
  return LinkOrderToChatUseCase(repository);
});

// Get or Create Commerce Chat UseCase Provider
final getOrCreateCommerceChatUseCaseProvider =
    Provider<GetOrCreateCommerceChatUseCase>((ref) {
      final repository = ref.watch(chatRepositoryProvider);
      return GetOrCreateCommerceChatUseCase(repository);
    });
