import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart';

// Profile stream provider for real-time updates
final profileStreamProvider = StreamProvider.family<ProfileEntity?, String>((
  ref,
  userId,
) {
  // MIGRATION: Now uses data layer provider instead of bootstrap
  final repository = ref.read(profileRepositoryProvider);
  return repository.watchProfile(userId);
});
