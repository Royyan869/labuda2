/// Chat Data Providers - Riverpod providers for chat data layer
///
/// This file provides all data dependencies for the chat feature using pure Riverpod.
/// Replaces the GetIt-based ChatApiDI dependency injection.
///
/// MIGRATION STATUS: Migrated from chat_api_di.dart (GetIt) to Riverpod
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_room_event_dto.dart';
import 'package:labuda/domains/chat/chat/data/remote/chat_api_datasource.dart';
import 'package:labuda/domains/chat/chat/data/repositories/chat_repository_impl.dart';
import 'package:labuda/domains/chat/chat/domain/repositories/chat_repository.dart';

// =============================================================================
// DATASOURCE PROVIDERS
// =============================================================================

/// Chat API Datasource Provider
final chatApiDatasourceProvider = Provider<ChatApiDatasource>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  final logger = ref.watch(loggerServiceProvider);
  return ChatApiDatasource(apiClient, logger: logger);
});

// =============================================================================
// REPOSITORY PROVIDERS
// =============================================================================

/// Chat Repository Provider
///
/// Provides the API implementation of ChatRepository.
/// This replaces the GetIt-based ChatApiDI.chatRepository.
///
/// MIGRATION: Previously accessed via ChatApiDI.chatRepository or the generic `sl` accessor
final chatRepositoryProvider = Provider<ChatRepository>((ref) {
  final apiDatasource = ref.watch(chatApiDatasourceProvider);
  final webSocketService = ref.watch(webSocketServiceProvider);
  final logger = ref.watch(loggerServiceProvider);
  return ChatRepositoryImpl(
    apiDatasource: apiDatasource,
    webSocketService: webSocketService,
    logger: logger,
  );
});

/// Parsed chat room event stream provider.
///
/// Exposes the repository gateway without wiring any list-state merge logic yet.
final chatRoomEventsProvider = StreamProvider.autoDispose<ChatRoomEventDto>((
  ref,
) {
  final repository = ref.watch(chatRepositoryProvider);
  return repository.watchChatRoomEvents();
});
