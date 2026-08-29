// =============================================================================
// ARCHITECTURE GUARDRAIL (R5) - FEATURE PROVIDER PATTERN EXAMPLE
// =============================================================================
//
// **CANONICAL PATTERN for Feature Providers:**
// 1. Import data layer providers via show (hide internal details)
// 2. Import core services from core/providers/core_providers.dart
// 3. Use ref.read() for dependencies in Provider constructors
// 4. DO NOT use sl<T>() or ServiceLocator.getService<T>()
// 5. Return domain entities or use cases, not data sources
//
// **CORRECT DEPENDENCY INJECTION:**
// ```dart
// final repository = ref.read(profileRepositoryProvider);  // ✅ From data layer
// final validation = ref.read(validationServiceProvider);   // ✅ From core
// ```
//
// **WRONG PATTERNS:**
// ```dart
// final api = sl<ApiClient>();                    // ❌ Don't use sl<T>()
// final service = ServiceLocator.getService();      // ❌ Don't use ServiceLocator
// final repo = ProfileRepository();                // ❌ Don't instantiate directly
// ```
// =============================================================================

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:labuda/domains/user/profile/domain/use_cases/get_profile_use_case.dart';
import 'package:labuda/domains/user/profile/domain/use_cases/update_profile_use_case.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart';

// Use case providers
final getProfileUseCaseProvider = Provider<GetProfileUseCase>((ref) {
  final repository = ref.read(profileRepositoryProvider);
  return GetProfileUseCase(repository);
});

final updateProfileUseCaseProvider = Provider<UpdateProfileUseCase>((ref) {
  final repository = ref.read(profileRepositoryProvider);
  final validation = ref.read(validationServiceProvider);
  return UpdateProfileUseCase(repository, validation);
});

// Profile state provider - simple FutureProvider approach
final profileProvider = FutureProvider.family<ProfileEntity?, String>((
  ref,
  userId,
) async {
  final useCase = ref.read(getProfileUseCaseProvider);
  final result = await useCase(userId);

  if (result.isSuccess) {
    return result.data;
  } else {
    throw Exception(result.error);
  }
});

// Profile actions provider for performing updates
final profileActionsProvider = Provider<ProfileActions>((ref) {
  final updateUseCase = ref.read(updateProfileUseCaseProvider);

  return ProfileActions(
    updateUseCase: updateUseCase,
    onProfileUpdated: () {
      // Invalidate profile provider to trigger refresh
      ref.invalidate(profileProvider);
    },
  );
});

/// Profile actions class to handle profile operations
class ProfileActions {
  final UpdateProfileUseCase updateUseCase;
  final VoidCallback onProfileUpdated;

  ProfileActions({required this.updateUseCase, required this.onProfileUpdated});

  /// Update profile
  Future<ProfileEntity> updateProfile(ProfileEntity profile) async {
    final result = await updateUseCase(profile);

    if (result.isSuccess) {
      onProfileUpdated();
      return result.data!;
    } else {
      throw Exception(result.error);
    }
  }

  /// Update specific fields only
  ///
  /// Only updates fields that are present in the map.
  /// To explicitly set a field to null (for removal), include the key with null value.
  Future<ProfileEntity> updateFields(
    ProfileEntity currentProfile,
    Map<String, dynamic> fields,
  ) async {
    // Build updated profile only with fields that are explicitly provided
    ProfileEntity updatedProfile = currentProfile;

    if (fields.containsKey('location')) {
      updatedProfile = ProfileEntity(
        id: updatedProfile.id,
        userId: updatedProfile.userId,
        location: fields['location'] as String?,
        coverPhotoUrl: updatedProfile.coverPhotoUrl,
        joinedAt: updatedProfile.joinedAt,
        lastActiveAt: updatedProfile.lastActiveAt,
        stats: updatedProfile.stats,
        verification: updatedProfile.verification,
        // REMOVED: achievements (PROFILE PURGE)
        contactInfo: updatedProfile.contactInfo,
        farmInfo: updatedProfile.farmInfo,
      );
    }

    if (fields.containsKey('coverPhotoUrl')) {
      updatedProfile = ProfileEntity(
        id: updatedProfile.id,
        userId: updatedProfile.userId,
        location: updatedProfile.location,
        coverPhotoUrl: fields['coverPhotoUrl'] as String?,
        joinedAt: updatedProfile.joinedAt,
        lastActiveAt: updatedProfile.lastActiveAt,
        stats: updatedProfile.stats,
        verification: updatedProfile.verification,
        // REMOVED: achievements (PROFILE PURGE)
        contactInfo: updatedProfile.contactInfo,
        farmInfo: updatedProfile.farmInfo,
      );
    }

    if (fields.containsKey('contactInfo')) {
      updatedProfile = ProfileEntity(
        id: updatedProfile.id,
        userId: updatedProfile.userId,
        location: updatedProfile.location,
        coverPhotoUrl: updatedProfile.coverPhotoUrl,
        joinedAt: updatedProfile.joinedAt,
        lastActiveAt: updatedProfile.lastActiveAt,
        stats: updatedProfile.stats,
        verification: updatedProfile.verification,
        // REMOVED: achievements (PROFILE PURGE)
        contactInfo: fields['contactInfo'] as ContactInfo?,
        farmInfo: updatedProfile.farmInfo,
      );
    }

    if (fields.containsKey('farmInfo')) {
      updatedProfile = ProfileEntity(
        id: updatedProfile.id,
        userId: updatedProfile.userId,
        location: updatedProfile.location,
        coverPhotoUrl: updatedProfile.coverPhotoUrl,
        joinedAt: updatedProfile.joinedAt,
        lastActiveAt: updatedProfile.lastActiveAt,
        stats: updatedProfile.stats,
        verification: updatedProfile.verification,
        // REMOVED: achievements (PROFILE PURGE)
        contactInfo: updatedProfile.contactInfo,
        farmInfo: fields['farmInfo'] as FarmInfo?,
      );
    }

    return await updateProfile(updatedProfile);
  }
}
