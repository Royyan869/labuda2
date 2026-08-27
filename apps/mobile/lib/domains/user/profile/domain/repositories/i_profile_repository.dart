import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';

/// Profile repository interface
///
/// Mengikuti GUIDELINES.md:
/// - Interface-first design
/// - `Result<T>` pattern untuk error handling
/// - Clean architecture compliance
abstract class IProfileRepository {
  // ========================================
  // Profile CRUD Operations
  // ========================================

  /// Get profile by user ID
  Future<Result<ProfileEntity?>> getProfile(String userId);

  /// Create new profile (usually called after registration)
  Future<Result<ProfileEntity>> createProfile(ProfileEntity profile);

  /// Update existing profile
  Future<Result<ProfileEntity>> updateProfile(ProfileEntity profile);

  /// Check if profile exists
  Future<Result<bool>> profileExists(String userId);

  // ========================================
  // Stats & Metrics Operations
  // ========================================

  /// Get current profile stats (posts count, etc)
  Future<Result<ProfileStats>> getProfileStats(String userId);

  // ========================================
  // Search & Discovery Operations
  // ========================================

  /// Search profiles by username or display name
  Future<Result<List<ProfileEntity>>> searchProfiles(
    String query, {
    int limit = 20,
    String? lastDocumentId,
  });

  /// Get profiles by user type (sellers, buyers, etc)
  /// NOTE: userType now uses AuthUser.UserRole for unified architecture
  Future<Result<List<ProfileEntity>>> getProfilesByType(
    UserRole userRole, {
    int limit = 20,
    String? lastDocumentId,
  });

  /// Get trending/popular profiles
  Future<Result<List<ProfileEntity>>> getTrendingProfiles({int limit = 10});

  // ========================================
  // Real-time Streams
  // ========================================

  /// Watch profile changes in real-time
  Stream<ProfileEntity?> watchProfile(String userId);

  // ========================================
  // Batch Operations
  // ========================================

  /// Get multiple profiles by user IDs
  Future<Result<List<ProfileEntity>>> getMultipleProfiles(List<String> userIds);

  // ========================================
  // Business/Seller Specific Operations
  // ========================================

  /// Update farm information for sellers
  Future<Result<ProfileEntity>> updateFarmInfo(
    String userId,
    FarmInfo farmInfo,
  );

  /// Get verified sellers
  Future<Result<List<ProfileEntity>>> getVerifiedSellers({
    int limit = 20,
    String? lastDocumentId,
  });
}
