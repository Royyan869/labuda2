import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:labuda/domains/user/profile/presentation/providers/state/profile_state.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show profileRepositoryProvider;

part 'profile_notifier.g.dart';

/// Profile Notifier - Application Layer Orchestrator
///
/// Phase 5: Replaces UseCase pattern with Riverpod Notifier
/// Using @riverpod annotation for code generation (pure Riverpod, no get_it)
///
/// Responsibilities:
/// - Orchestrate profile CRUD operations
/// - Manage loading/error states
/// - Delegate to repository interface (no direct API/Firebase access)
///
/// 🚫 RULES:
/// - No Firebase imports
/// - No get_it/service locator
/// - No UI logic (formatting, etc.)
/// - Only business orchestration
@riverpod
class ProfileNotifier extends _$ProfileNotifier {
  @override
  ProfileState build() {
    // Repository is injected via ref.watch() - no get_it!
    return const ProfileState();
  }

  /// Fetch profile by user ID
  Future<void> fetchProfile(String userId) async {
    final repository = ref.read(profileRepositoryProvider);

    state = const ProfileState(profile: AsyncValue.loading());

    final result = await repository.getProfile(userId);

    result.fold(
      (error) {
        state = ProfileState.error(error, StackTrace.current);
      },
      (data) {
        if (data != null) {
          state = ProfileState.data(data);
        } else {
          // Profile not found - keep as null
          state = const ProfileState(profile: AsyncValue.data(null));
        }
      },
    );
  }

  /// Update profile
  Future<void> updateProfile(ProfileEntity profile) async {
    final repository = ref.read(profileRepositoryProvider);

    state = state.copyWith(isUpdating: true);

    final result = await repository.updateProfile(profile);

    result.fold(
      (error) {
        state = state.copyWith(isUpdating: false, errorMessage: error);
      },
      (updated) {
        state = ProfileState.data(updated);
      },
    );
  }

  /// Update farm info
  Future<void> updateFarmInfo(String userId, FarmInfo farmInfo) async {
    final repository = ref.read(profileRepositoryProvider);
    final currentProfile = state.profile.value;
    if (currentProfile == null) {
      state = state.copyWith(errorMessage: 'Cannot update: no profile loaded');
      return;
    }

    state = state.copyWith(isUpdating: true);

    final result = await repository.updateFarmInfo(userId, farmInfo);

    result.fold(
      (error) {
        state = state.copyWith(isUpdating: false, errorMessage: error);
      },
      (updated) {
        state = ProfileState.data(updated);
      },
    );
  }

  /// Get profile stats
  Future<void> fetchStats(String userId) async {
    final repository = ref.read(profileRepositoryProvider);

    final result = await repository.getProfileStats(userId);

    result.fold(
      (error) {
        // Stats fetch failure is not critical - don't change state
      },
      (stats) {
        // Stats could be stored separately if needed
        // For now, we just fetch them without storing in state
      },
    );
  }

  /// Search profiles
  Future<List<ProfileEntity>> searchProfiles(
    String query, {
    int limit = 20,
  }) async {
    final repository = ref.read(profileRepositoryProvider);

    final result = await repository.searchProfiles(query, limit: limit);

    return result.fold((error) => <ProfileEntity>[], (profiles) => profiles);
  }

  /// Clear error message
  void clearError() {
    state = state.copyWith(errorMessage: null);
  }

  /// Reset to initial state
  void reset() {
    state = const ProfileState();
  }
}

// Alias for backward compatibility
// The generated provider name is 'profileProvider', but code expects 'profileNotifierProvider'
final profileNotifierProvider = profileProvider;
