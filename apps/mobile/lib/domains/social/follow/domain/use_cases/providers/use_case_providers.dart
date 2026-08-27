/// Follow Use Case Providers - Riverpod providers for follow domain use cases
///
/// This file provides all use case dependencies for the follow feature using pure Riverpod.
/// Replaces the GetIt-based service locator pattern.
///
/// MIGRATION STATUS: New file - migrating from GetIt ServiceLocator to Riverpod
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/social/follow/data/follow_providers.dart';
import 'package:labuda/domains/social/follow/domain/use_cases/follow_user_use_case.dart';
import 'package:labuda/domains/social/follow/domain/use_cases/unfollow_user_use_case.dart';
import 'package:labuda/domains/social/follow/domain/use_cases/get_followers_use_case.dart';
import 'package:labuda/domains/social/follow/domain/use_cases/get_following_use_case.dart';
import 'package:labuda/domains/social/follow/domain/use_cases/get_follow_stats_use_case.dart';
import 'package:labuda/domains/social/follow/domain/use_cases/search_users_use_case.dart';

// Re-exports for use cases and their parameter classes
export 'package:labuda/domains/social/follow/domain/use_cases/follow_user_use_case.dart'
    show FollowUserUseCase, FollowUserParams;
export 'package:labuda/domains/social/follow/domain/use_cases/unfollow_user_use_case.dart'
    show UnfollowUserUseCase, UnfollowUserParams;
export 'package:labuda/domains/social/follow/domain/use_cases/get_followers_use_case.dart'
    show GetFollowersUseCase, GetFollowersParams;
export 'package:labuda/domains/social/follow/domain/use_cases/get_following_use_case.dart'
    show GetFollowingUseCase, GetFollowingParams;
export 'package:labuda/domains/social/follow/domain/use_cases/get_follow_stats_use_case.dart'
    show GetFollowStatsUseCase, GetFollowStatsParams;
export 'package:labuda/domains/social/follow/domain/use_cases/search_users_use_case.dart'
    show SearchUsersUseCase, SearchUsersParams;
export 'package:labuda/domains/social/follow/domain/repositories/i_follow_repository.dart'
    show IFollowRepository;

// =============================================================================
// USE CASE PROVIDERS
// =============================================================================

/// Follow User Use Case Provider
///
/// Provides the use case for following users.
/// BATCH N2: Notification trigger removed - follow notifications are backend-only.
final followUserUseCaseProvider = Provider<FollowUserUseCase>((ref) {
  final repository = ref.watch(followRepositoryProvider);
  return FollowUserUseCase(repository);
});

/// Unfollow User Use Case Provider
final unfollowUserUseCaseProvider = Provider<UnfollowUserUseCase>((ref) {
  final repository = ref.watch(followRepositoryProvider);
  return UnfollowUserUseCase(repository);
});

/// Get Followers Use Case Provider
final getFollowersUseCaseProvider = Provider<GetFollowersUseCase>((ref) {
  final repository = ref.watch(followRepositoryProvider);
  return GetFollowersUseCase(repository);
});

/// Get Following Use Case Provider
final getFollowingUseCaseProvider = Provider<GetFollowingUseCase>((ref) {
  final repository = ref.watch(followRepositoryProvider);
  return GetFollowingUseCase(repository);
});

/// Get Follow Stats Use Case Provider
final getFollowStatsUseCaseProvider = Provider<GetFollowStatsUseCase>((ref) {
  final repository = ref.watch(followRepositoryProvider);
  return GetFollowStatsUseCase(repository);
});

/// Search Users Use Case Provider
final searchUsersUseCaseProvider = Provider<SearchUsersUseCase>((ref) {
  final repository = ref.watch(followRepositoryProvider);
  return SearchUsersUseCase(repository);
});
