/// Follow Stream Providers - Riverpod stream providers for real-time follow data
///
/// This file provides stream providers for real-time follow data using pure Riverpod.
/// Replaces the GetIt-based ServiceLocator pattern.
///
/// MIGRATION STATUS: Migrated from ServiceLocator to followRepositoryProvider
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/social/follow/data/follow_providers.dart';
import 'package:labuda/domains/social/follow/domain/entities/follow_entity.dart';

/// Stream provider untuk followers real-time
/// Uses keepAlive to maintain connection while screen is active
final followersStreamProvider = StreamProvider.autoDispose
    .family<List<FollowableUser>, String>((ref, userId) {
      // Keep alive while there are listeners
      ref.keepAlive();
      final repository = ref.watch(followRepositoryProvider);
      return repository.watchFollowers(userId);
    });

/// Stream provider untuk following real-time
/// Uses keepAlive to maintain connection while screen is active
final followingStreamProvider = StreamProvider.autoDispose
    .family<List<FollowableUser>, String>((ref, userId) {
      // Keep alive while there are listeners
      ref.keepAlive();
      final repository = ref.watch(followRepositoryProvider);
      return repository.watchFollowing(userId);
    });

/// Stream provider untuk follow stats real-time
/// Uses keepAlive to maintain connection for profile header
final followStatsStreamProvider = StreamProvider.autoDispose
    .family<FollowStats, String>((ref, userId) {
      // Keep alive while there are listeners
      ref.keepAlive();
      final repository = ref.watch(followRepositoryProvider);
      return repository.watchFollowStats(userId);
    });

/// Stream provider untuk follow activities
final followActivitiesStreamProvider = StreamProvider.autoDispose
    .family<List<FollowActivity>, String>((ref, userId) {
      ref.keepAlive();
      final repository = ref.watch(followRepositoryProvider);
      return repository.watchFollowActivities(userId);
    });
