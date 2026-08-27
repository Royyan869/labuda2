library;

/// Application Layer for Support Module
/// Riverpod Notifier - handles business logic from UseCases
/// Application layer - bergantung pada domain dan data layer

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/support/domain/domain.dart';
import 'package:labuda/domains/system/support/data/datasources/support_api_datasource.dart';
import 'package:labuda/domains/system/support/data/repositories/support_repository_api.dart';

part 'support_notifier.g.dart';

// ============================================
// DATASOURCE PROVIDER
// ============================================

/// Support API Datasource Provider
/// Internal provider for data layer
final _supportApiDatasourceProvider = Provider<SupportApiDatasource>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  final logger = ref.watch(loggerServiceProvider);
  return SupportApiDatasource(apiClient, logger: logger);
});

// ============================================
// REPOSITORY PROVIDER
// ============================================

/// Provider for SupportRepository
/// Provides the API implementation directly.
/// This follows CLIENT_MIGRATION_STANDARD.md - no UnimplementedError override pattern.
@riverpod
SupportRepository supportRepository(Ref ref) {
  final datasource = ref.watch(_supportApiDatasourceProvider);
  final logger = ref.watch(loggerServiceProvider);
  return SupportRepositoryApi(datasource: datasource, logger: logger);
}

// ============================================
// SINGLE TICKET PROVIDER
// ============================================

/// Single ticket provider
@riverpod
Future<SupportTicket?> supportTicket(Ref ref, String ticketId) async {
  final repository = ref.watch(supportRepositoryProvider);
  final result = await repository.getTicket(ticketId);

  if (result.isSuccess) {
    return result.data;
  }

  return null;
}

// ============================================
// ACTION PROVIDERS
// ============================================

/// Create support chat action
@riverpod
Future<String?> supportCreateChat(
  Ref ref, {
  required String userId,
  required String userName,
  String? userAvatar,
  required SupportCategory category,
  SupportPriority priority = SupportPriority.medium,
  String? description,
  String? linkedOrderId,
}) async {
  final repository = ref.watch(supportRepositoryProvider);

  final result = await repository.createSupportChat(
    userId: userId,
    userName: userName,
    userAvatar: userAvatar,
    category: category,
    priority: priority,
    description: description,
    linkedOrderId: linkedOrderId,
  );

  if (result.isSuccess) {
    return result.data;
  }

  return null;
}

/// Reopen ticket action
@riverpod
Future<bool> supportReopenTicket(Ref ref, ReopenTicketRequest request) async {
  final repository = ref.watch(supportRepositoryProvider);

  final result = await repository.reopenTicket(request);

  return result.isSuccess;
}
