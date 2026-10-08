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
// TICKET LIST PROVIDER (canonical list authority)
// ============================================

/// Canonical Support Tickets list authority for the authenticated account.
///
/// ONE provider owns the ticket list; the screen consumes it and never keeps
/// a page-local `FutureBuilder`/`setState` state machine. Refreshing through
/// `ref.refresh` preserves the last-known-good list while in flight and on
/// failure (surfaced inline), so a refresh never collapses to full loading or
/// an empty list.
///
/// Auto-retry is disabled so a failure settles into a deterministic error the
/// screen can render (PageErrorState with no data / inline banner with data)
/// instead of an endless spinner.
final supportTicketsProvider = FutureProvider.autoDispose<List<SupportTicket>>((
  ref,
) async {
  final repository = ref.watch(supportRepositoryProvider);
  final result = await repository.getMyTickets();

  if (result.isError) {
    throw Exception(result.error ?? 'Failed to load support tickets');
  }

  return result.data ?? const <SupportTicket>[];
}, retry: (retryCount, error) => null);

// ============================================
// TICKET ACTIVITY (EVENTS) PROVIDER
// ============================================

/// Canonical read-only activity authority for ONE support ticket.
///
/// The ticket thread watches this provider to render the activity timeline.
/// It reads through the SAME canonical `SupportRepository.getEvents` pipeline
/// that reaches the owner-only `GET /support/tickets/:id/events` contract —
/// there is no second event source.
///
/// Auto-retry is disabled so a failure settles into a deterministic error the
/// timeline section can render (with an explicit retry) instead of an endless
/// spinner. Events are returned in canonical chronology (oldest first) so the
/// timeline reads as the ticket's own story; the backend returns newest-first.
final supportTicketEventsProvider = FutureProvider.autoDispose
    .family<List<SupportEvent>, String>((ref, ticketId) async {
      final repository = ref.watch(supportRepositoryProvider);
      final result = await repository.getEvents(ticketId);

      if (result.isError) {
        throw Exception(result.error ?? 'Failed to load ticket activity');
      }

      final events = [...?result.data];
      events.sort((a, b) => a.createdAt.compareTo(b.createdAt));
      return events;
    }, retry: (retryCount, error) => null);
