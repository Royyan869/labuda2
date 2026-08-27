import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/models/blocked_user_model.dart';
import 'package:labuda/domains/user/profile/data/services/blocked_users_service.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart';

/// Provider for BlockedUsersService
/// MIGRATED: Now using ApiClient instead of Firestore
final blockedUsersServiceProvider = Provider<BlockedUsersService>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  final userLookupService = ref.watch(userLookupServiceProvider);
  final logger = ref.watch(loggerServiceProvider);
  return BlockedUsersService(
    apiClient,
    userLookupService: userLookupService,
    logger: logger,
  );
});

/// Provider for streaming blocked users list
final blockedUsersProvider =
    StreamProvider.family<List<BlockedUserModel>, String>((ref, userId) {
      final service = ref.watch(blockedUsersServiceProvider);
      return service.watchBlockedUsers(userId);
    });

/// Provider for blocked users actions
final blockedUsersActionsProvider = Provider<BlockedUsersService>((ref) {
  return ref.watch(blockedUsersServiceProvider);
});
