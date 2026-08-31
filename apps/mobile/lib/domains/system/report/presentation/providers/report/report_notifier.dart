/// Report Notifier
///
/// Riverpod Notifier for report functionality.
/// Replaces UseCase layer - contains business logic orchestration.
library;

import 'package:riverpod/riverpod.dart';

import 'package:labuda/domains/system/report/data/data.dart'
    show ReportRepositoryException;
import 'package:labuda/domains/system/report/domain/entities/entities.dart';
import 'package:labuda/domains/system/report/domain/repositories/report_repository.dart';
import 'package:labuda/domains/system/report/presentation/providers/report_providers.dart';
import 'report_state.dart';

/// Report Actions Notifier - handles user report actions
class ReportActionsNotifier extends Notifier<ReportActionsState> {
  ReportActionsNotifier();

  @override
  ReportActionsState build() {
    return const ReportActionsState();
  }

  // =====================
  // Dependencies
  // =====================

  ReportRepository get _repository => ref.read(reportRepositoryProvider);

  // =====================
  // User Actions
  // =====================

  /// Submit a new report
  Future<bool> submitReport(CreateReportRequest request) async {
    // Guard: block unsupported types before API call
    if (!request.subjectType.isBackendSupported) {
      state = state.copyWith(
        isLoading: false,
        error: 'Fitur laporan ini belum tersedia.',
      );
      return false;
    }

    if (!request.isValid) {
      state = state.copyWith(isLoading: false, error: 'Invalid report request');
      return false;
    }

    state = state.copyWith(isLoading: true, error: null);

    try {
      // Get current user ID from auth
      final userId = ref.read(reportCurrentUserIdProvider);

      if (userId == null) {
        state = state.copyWith(
          isLoading: false,
          error: 'Anda harus login untuk melaporkan',
        );
        return false;
      }

      final report = await _repository.createReport(
        reporterId: userId,
        request: request,
      );

      state = state.copyWith(isLoading: false, lastReport: report);
      return true;
    } on ReportRepositoryException catch (e) {
      state = state.copyWith(isLoading: false, error: e.message);
      return false;
    } catch (e) {
      state = state.copyWith(isLoading: false, error: e.toString());
      return false;
    }
  }

  /// Check if user already reported this target
  Future<bool> hasReported({
    required String targetId,
    required ReportTargetType targetType,
  }) async {
    try {
      final userId = ref.read(reportCurrentUserIdProvider);
      if (userId == null) return false;

      return await _repository.hasUserReported(
        userId: userId,
        targetId: targetId,
        targetType: targetType,
      );
    } catch (_) {
      return false;
    }
  }

  /// Clear error state
  void clearError() {
    state = state.copyWith(error: null);
  }
}

/// Report List Notifier - manages user's submitted reports
class ReportListNotifier extends Notifier<ReportListState> {
  ReportListNotifier();

  @override
  ReportListState build() {
    // Load reports on init
    Future.microtask(() => loadReports());
    return const ReportListState(isLoading: true);
  }

  ReportRepository get _repository => ref.read(reportRepositoryProvider);

  /// Load user's reports
  Future<void> loadReports({bool refresh = false}) async {
    if (refresh) {
      state = state.copyWith(isLoading: true, error: null);
    }

    try {
      final userId = ref.read(reportCurrentUserIdProvider);
      if (userId == null) {
        state = state.copyWith(isLoading: false, reports: []);
        return;
      }

      final reports = await _repository.getReportsByUser(userId: userId);

      state = state.copyWith(reports: reports, isLoading: false);
    } catch (e) {
      state = state.copyWith(isLoading: false, error: e.toString());
    }
  }

  /// Refresh the list
  Future<void> refresh() async {
    await loadReports(refresh: true);
  }
}
