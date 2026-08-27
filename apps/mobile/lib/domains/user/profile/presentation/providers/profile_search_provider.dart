import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart';

// Multiple profiles provider
final multipleProfilesProvider =
    FutureProvider.family<List<ProfileEntity>, List<String>>((
      ref,
      userIds,
    ) async {
      final repository = ref.read(profileRepositoryProvider);
      final result = await repository.getMultipleProfiles(userIds);

      if (result.isSuccess) {
        return result.data!;
      } else {
        throw Exception(result.error);
      }
    });

// Search profiles provider
final searchProfilesProvider =
    FutureProvider.family<List<ProfileEntity>, String>((ref, query) async {
      if (query.trim().isEmpty) return [];

      final repository = ref.read(profileRepositoryProvider);
      final result = await repository.searchProfiles(query);

      if (result.isSuccess) {
        return result.data!;
      } else {
        throw Exception(result.error);
      }
    });

// Trending profiles provider
final trendingProfilesProvider = FutureProvider<List<ProfileEntity>>((
  ref,
) async {
  final repository = ref.read(profileRepositoryProvider);
  final result = await repository.getTrendingProfiles();

  if (result.isSuccess) {
    return result.data!;
  } else {
    throw Exception(result.error);
  }
});

// Verified sellers provider
final verifiedSellersProvider = FutureProvider<List<ProfileEntity>>((
  ref,
) async {
  final repository = ref.read(profileRepositoryProvider);
  final result = await repository.getVerifiedSellers();

  if (result.isSuccess) {
    return result.data!;
  } else {
    throw Exception(result.error);
  }
});

// Profiles by type provider
final profilesByTypeProvider =
    FutureProvider.family<List<ProfileEntity>, UserRole>((ref, userRole) async {
      final repository = ref.read(profileRepositoryProvider);
      final result = await repository.getProfilesByType(userRole);

      if (result.isSuccess) {
        return result.data!;
      } else {
        throw Exception(result.error);
      }
    });
