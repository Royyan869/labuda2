/// Appeal Notifier
///
/// Riverpod Notifier for appeal functionality.
library;

import 'package:riverpod/riverpod.dart';

import 'package:labuda/domains/system/report/data/data.dart'
    show AppealRepositoryException;
import 'package:labuda/domains/system/report/domain/entities/entities.dart';
import 'package:labuda/domains/system/report/domain/repositories/appeal_repository.dart';
import 'package:labuda/domains/system/report/presentation/providers/report_providers.dart';
import 'appeal_state.dart';

/// Appeal Actions Notifier - handles user appeal actions
class AppealActionsNotifier extends Notifier<AppealActionsState> {
  AppealActionsNotifier();

  @override
  AppealActionsState build() {
    return const AppealActionsState();
  }

  AppealRepository get _repository => ref.read(appealRepositoryProvider);

  /// Submit a new appeal
  Future<bool> submitAppeal(CreateAppealRequest request) async {
    if (!request.isValid) {
      state = state.copyWith(isLoading: false, error: 'Invalid appeal request');
      return false;
    }

    state = state.copyWith(isLoading: true, error: null);

    try {
      final userId = ref.read(reportCurrentUserIdProvider);

      if (userId == null) {
        state = state.copyWith(
          isLoading: false,
          error: 'Anda harus login untuk mengajukan banding',
        );
        return false;
      }

      // Check for existing pending appeal
      final hasPending = await _repository.hasPendingAppeal(
        userId: userId,
        type: request.appealType,
        sourceId: request.sourceId,
      );

      if (hasPending) {
        state = state.copyWith(
          isLoading: false,
          error: 'Anda sudah memiliki banding yang sedang diproses',
        );
        return false;
      }

      final appeal = await _repository.submitAppeal(request);

      state = state.copyWith(isLoading: false, lastAppeal: appeal);
      return true;
    } on AppealRepositoryException catch (e) {
      state = state.copyWith(isLoading: false, error: e.message);
      return false;
    } catch (e) {
      state = state.copyWith(isLoading: false, error: e.toString());
      return false;
    }
  }

  /// Check if user has pending appeal
  Future<bool> hasPendingAppeal({
    required AppealType type,
    String? sourceId,
  }) async {
    try {
      final userId = ref.read(reportCurrentUserIdProvider);
      if (userId == null) return false;

      return await _repository.hasPendingAppeal(
        userId: userId,
        type: type,
        sourceId: sourceId,
      );
    } catch (_) {
      return false;
    }
  }

  /// Cancel an appeal
  Future<bool> cancelAppeal(String appealId) async {
    state = state.copyWith(isLoading: true);

    try {
      await _repository.cancelAppeal(appealId);
      state = state.copyWith(isLoading: false);
      return true;
    } catch (e) {
      state = state.copyWith(isLoading: false, error: e.toString());
      return false;
    }
  }

  /// Clear error state
  void clearError() {
    state = state.copyWith(error: null);
  }
}

/// User Appeal List Notifier - manages user's appeals
class UserAppealListNotifier extends Notifier<AppealListState> {
  UserAppealListNotifier();

  @override
  AppealListState build() {
    // Load appeals on init
    Future.microtask(() => loadAppeals());
    return const AppealListState(isLoading: true);
  }

  AppealRepository get _repository => ref.read(appealRepositoryProvider);

  /// Load user's appeals
  Future<void> loadAppeals({bool refresh = false}) async {
    if (refresh) {
      state = state.copyWith(isLoading: true, error: null);
    }

    try {
      final userId = ref.read(reportCurrentUserIdProvider);
      if (userId == null) {
        state = state.copyWith(isLoading: false, appeals: []);
        return;
      }

      final appeals = await _repository.getUserAppeals(userId);

      state = state.copyWith(appeals: appeals, isLoading: false);
    } catch (e) {
      state = state.copyWith(isLoading: false, error: e.toString());
    }
  }

  /// Refresh the list
  Future<void> refresh() async {
    await loadAppeals(refresh: true);
  }
}
