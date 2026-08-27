import 'dart:async';

import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/mappers/user_api_mapper.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:labuda/domains/user/profile/domain/repositories/i_profile_repository.dart';

/// API implementation of IProfileRepository
///
/// Replaces Firestore-based ProfileRepositoryImpl with Go backend API calls.
/// Uses UserApiDatasource for HTTP operations and UserApiMapper for conversions.
///
/// Migration Note:
/// - Interface stays aligned with the live profile/social API surface
/// - All Firestore operations are replaced with HTTP API calls
/// - Real-time streams use polling or WebSocket (future implementation)
class ProfileRepositoryApi implements IProfileRepository {
  final UserApiDatasource _datasource;
  final ILoggerService _logger;

  ProfileRepositoryApi({
    required UserApiDatasource datasource,
    required ILoggerService logger,
  }) : _datasource = datasource,
       _logger = logger;

  // ========================================
  // Profile CRUD Operations
  // ========================================

  @override
  Future<Result<ProfileEntity?>> getProfile(String userId) async {
    final result = await _datasource.getUserById(userId);

    return result.fold(
      (error) {
        // Return null for not found, error for other failures
        if (error.contains('not found') || error.contains('404')) {
          return Result.success(null);
        }
        _logger.error('Failed to get profile: $error');
        return Result.error(error);
      },
      (response) {
        final profile = UserApiMapper.toProfileEntity(response);
        return Result.success(profile);
      },
    );
  }

  @override
  Future<Result<ProfileEntity>> createProfile(ProfileEntity profile) async {
    // Profile creation is handled during user sync (Firebase auth → backend)
    // This method creates/updates the profile via the update endpoint
    final request = _profileEntityToUpdateRequest(profile);
    // Use /users/me/profile - backend extracts identity from auth token
    final result = await _datasource.updateMyProfile(request);

    return result.fold(
      (error) {
        _logger.error('Failed to create profile: $error');
        return Result.error(error);
      },
      (response) {
        final updatedProfile = UserApiMapper.toProfileEntity(response);
        return Result.success(updatedProfile);
      },
    );
  }

  @override
  Future<Result<ProfileEntity>> updateProfile(ProfileEntity profile) async {
    final request = _profileEntityToUpdateRequest(profile);
    // Use /users/me/profile - backend extracts identity from auth token
    final result = await _datasource.updateMyProfile(request);

    return result.fold(
      (error) {
        _logger.error('Failed to update profile: $error');
        return Result.error(error);
      },
      (response) {
        final updatedProfile = UserApiMapper.toProfileEntity(response);
        return Result.success(updatedProfile);
      },
    );
  }

  @override
  Future<Result<bool>> profileExists(String userId) async {
    final result = await _datasource.getUserById(userId);

    return result.fold((error) {
      if (error.contains('not found') || error.contains('404')) {
        return Result.success(false);
      }
      return Result.error(error);
    }, (_) => Result.success(true));
  }

  // ========================================
  // Stats & Metrics Operations
  // ========================================

  @override
  Future<Result<ProfileStats>> getProfileStats(String userId) async {
    final result = await _datasource.getProfileStats(userId);

    return result.fold((error) {
      _logger.error('Failed to get profile stats: $error');
      return Result.error(error);
    }, (stats) => Result.success(stats));
  }

  // ========================================
  // Search & Discovery Operations
  // ========================================

  @override
  Future<Result<List<ProfileEntity>>> searchProfiles(
    String query, {
    int limit = 20,
    String? lastDocumentId,
  }) async {
    // Convert lastDocumentId to page number if provided
    final page = lastDocumentId != null ? int.tryParse(lastDocumentId) ?? 1 : 1;

    final result = await _datasource.searchUsers(
      query: query,
      page: page,
      limit: limit,
    );

    return result.fold(
      (error) {
        _logger.error('Failed to search profiles: $error');
        return Result.error(error);
      },
      (users) {
        final profiles = users.map(UserApiMapper.toProfileEntity).toList();
        return Result.success(profiles);
      },
    );
  }

  @override
  Future<Result<List<ProfileEntity>>> getProfilesByType(
    UserRole userRole, {
    int limit = 20,
    String? lastDocumentId,
  }) async {
    final page = lastDocumentId != null ? int.tryParse(lastDocumentId) ?? 1 : 1;

    final result = await _datasource.getUsersByRole(
      role: userRole.name,
      page: page,
      limit: limit,
    );

    return result.fold(
      (error) {
        _logger.error('Failed to get profiles by type: $error');
        return Result.error(error);
      },
      (users) {
        final profiles = users.map(UserApiMapper.toProfileEntity).toList();
        return Result.success(profiles);
      },
    );
  }

  @override
  Future<Result<List<ProfileEntity>>> getTrendingProfiles({
    int limit = 10,
  }) async {
    final result = await _datasource.getTrendingUsers(limit: limit);

    return result.fold(
      (error) {
        _logger.error('Failed to get trending profiles: $error');
        return Result.success(<ProfileEntity>[]); // Return empty list on error
      },
      (users) {
        final profiles = users.map(UserApiMapper.toProfileEntity).toList();
        return Result.success(profiles);
      },
    );
  }

  // ========================================
  // Real-time Streams
  // ========================================

  @override
  Stream<ProfileEntity?> watchProfile(String userId) {
    // API implementation uses polling instead of Firestore streams
    // For real-time updates, WebSocket will be implemented in Phase 3D
    return Stream.periodic(const Duration(seconds: 30), (_) => userId).asyncMap(
      (id) async {
        final result = await getProfile(id);
        return result.fold((_) => null, (profile) => profile);
      },
    );
  }

  // ========================================
  // Batch Operations
  // ========================================

  @override
  Future<Result<List<ProfileEntity>>> getMultipleProfiles(
    List<String> userIds,
  ) async {
    if (userIds.isEmpty) {
      return Result.success(<ProfileEntity>[]);
    }

    final result = await _datasource.getMultipleUsers(userIds);

    return result.fold(
      (error) {
        _logger.error('Failed to get multiple profiles: $error');
        return Result.error(error);
      },
      (users) {
        final profiles = users.map(UserApiMapper.toProfileEntity).toList();
        return Result.success(profiles);
      },
    );
  }

  // ========================================
  // Business/Seller Specific Operations
  // ========================================

  @override
  Future<Result<ProfileEntity>> updateFarmInfo(
    String userId,
    FarmInfo farmInfo,
  ) async {
    final result = await _datasource.updateFarmInfo(userId, farmInfo);

    return result.fold(
      (error) {
        _logger.error('Failed to update farm info: $error');
        return Result.error(error);
      },
      (response) {
        final profile = UserApiMapper.toProfileEntity(response);
        return Result.success(profile);
      },
    );
  }

  @override
  Future<Result<List<ProfileEntity>>> getVerifiedSellers({
    int limit = 20,
    String? lastDocumentId,
  }) async {
    final page = lastDocumentId != null ? int.tryParse(lastDocumentId) ?? 1 : 1;

    final result = await _datasource.getVerifiedSellers(
      page: page,
      limit: limit,
    );

    return result.fold(
      (error) {
        _logger.error('Failed to get verified sellers: $error');
        return Result.success(<ProfileEntity>[]);
      },
      (users) {
        final profiles = users.map(UserApiMapper.toProfileEntity).toList();
        return Result.success(profiles);
      },
    );
  }

  // ========================================
  // Private Helpers
  // ========================================

  UpdateProfileApiRequest _profileEntityToUpdateRequest(ProfileEntity profile) {
    return UpdateProfileApiRequest(
      location: profile.location,
      // Cover photo: persisted as the canonical storage key; empty string
      // clears the cover (backend converts to NULL).
      coverPhotoUrl: profile.coverPhotoUrl,
      // Contact info
      instagramHandle: profile.contactInfo?.instagramHandle,
      facebookHandle: profile.contactInfo?.facebookHandle,
      twitterHandle: profile.contactInfo?.twitterHandle,
      tiktokHandle: profile.contactInfo?.tiktokHandle,
      // Privacy settings
      showPhoneNumber: profile.contactInfo?.isPhonePublic,
      showEmail: profile.contactInfo?.isEmailPublic,
    );
  }
}
