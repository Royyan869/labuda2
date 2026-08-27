/// Block State Provider
///
/// Block functionality is owned by the profile domain
/// (domains/user/profile/data/services/blocked_users_service.dart) which
/// targets the canonical backend routes
/// `GET /api/v1/blocks`, `POST /api/v1/users/:id/block`,
/// `DELETE /api/v1/users/:id/block`. This file lives in `shared/` because
/// block filtering is a cross-feature primitive (content, chat, profile),
/// and the streamed UUID set surfaced here is the single source of truth
/// the rest of the app reads from.
///
/// Backend has no block-status check endpoint — `isUserBlockedProvider`
/// must be the only "is X blocked?" path. Do not add an async per-id check
/// provider; resolve via the cached set.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/user/profile/profile.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart';

export 'block_action_state.dart';

/// Stream provider untuk blocked user IDs (untuk filtering efisien)
final blockedUserIdsProvider = StreamProvider<Set<String>>((ref) {
  final currentUserId = ref.watch(currentUserIdProvider);
  if (currentUserId.isEmpty) return Stream.value(<String>{});

  final service = ref.watch(blockedUsersServiceProvider);
  return service.watchBlockedUsers(currentUserId).map((blockedUsers) {
    return blockedUsers.map((user) => user.id).toSet();
  });
});

/// Provider untuk cek apakah user tertentu diblokir
final isUserBlockedProvider = Provider.family<bool, String>((ref, userId) {
  final blockedIdsAsync = ref.watch(blockedUserIdsProvider);
  return blockedIdsAsync.when(
    data: (blockedIds) => blockedIds.contains(userId),
    loading: () => false,
    error: (_, _) => false,
  );
});

/// Block actions state - imported from block_action_state.dart
class BlockActionState {
  final bool isLoading;
  final String? error;
  final String? successMessage;

  const BlockActionState({
    this.isLoading = false,
    this.error,
    this.successMessage,
  });

  BlockActionState copyWith({
    bool? isLoading,
    String? error,
    String? successMessage,
  }) {
    return BlockActionState(
      isLoading: isLoading ?? this.isLoading,
      error: error,
      successMessage: successMessage,
    );
  }
}

/// Notifier untuk block/unblock actions
class BlockActionsNotifier extends Notifier<BlockActionState> {
  @override
  BlockActionState build() => const BlockActionState();

  Future<bool> blockUser({
    required String targetUserId,
    required String targetDisplayName,
    String? targetAvatarUrl,
  }) async {
    final currentUserId = ref.read(currentUserIdProvider);

    if (currentUserId.isEmpty) {
      state = state.copyWith(error: 'Anda harus login untuk memblokir user');
      return false;
    }
    if (currentUserId == targetUserId) {
      state = state.copyWith(error: 'Tidak bisa memblokir diri sendiri');
      return false;
    }

    state = state.copyWith(isLoading: true, error: null);

    try {
      final service = ref.read(blockedUsersServiceProvider);
      await service.blockUser(
        currentUserId: currentUserId,
        blockedUserId: targetUserId,
        blockedUserUsername: targetDisplayName.replaceFirst('@', ''),
        blockedUserAvatarUrl: targetAvatarUrl,
      );
      state = state.copyWith(
        isLoading: false,
        successMessage: '$targetDisplayName telah diblokir',
      );
      return true;
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: 'Gagal memblokir user: $e',
      );
      return false;
    }
  }

  Future<bool> unblockUser({
    required String targetUserId,
    String? targetDisplayName,
  }) async {
    final currentUserId = ref.read(currentUserIdProvider);
    if (currentUserId.isEmpty) {
      state = state.copyWith(error: 'Anda harus login');
      return false;
    }

    state = state.copyWith(isLoading: true, error: null);

    try {
      final service = ref.read(blockedUsersServiceProvider);
      await service.unblockUser(
        currentUserId: currentUserId,
        blockedUserId: targetUserId,
      );
      final name = targetDisplayName ?? 'User';
      state = state.copyWith(
        isLoading: false,
        successMessage: '$name telah di-unblock',
      );
      return true;
    } catch (e) {
      state = state.copyWith(isLoading: false, error: 'Gagal unblock user: $e');
      return false;
    }
  }

  void clearMessages() {
    state = state.copyWith(error: null, successMessage: null);
  }
}

/// Provider untuk block actions
final blockActionsProvider =
    NotifierProvider<BlockActionsNotifier, BlockActionState>(() {
      return BlockActionsNotifier();
    });
