import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';
import 'package:labuda/features/search/search/domain/entities/user_search.dart';

/// User Lookup Service
///
/// Lightweight service for fetching user identity data needed for:
/// - Tagging users (TaggedUsersChips)
/// - Displaying user info (avatars, names)
/// - Mention resolution
///
/// **R2.3 PROFILE DOMAIN EXTRACTION:**
/// This service is the honest owner of user identity lookup.
/// Replaces UserSearchApiService from shared (deprecated).
///
/// Uses efficient batch API endpoint (POST /users/batch)
/// instead of sequential individual fetches.
class UserLookupService {
  final UserApiDatasource _datasource;
  final ILoggerService? _logger;

  UserLookupService({
    required UserApiDatasource datasource,
    ILoggerService? logger,
  }) : _datasource = datasource,
       _logger = logger;

  /// Get a single user by ID
  ///
  /// Returns UserSearch (lightweight entity) for display purposes.
  /// Returns null if user not found.
  Future<UserSearch?> getUserById(String userId) async {
    if (userId.isEmpty) {
      return null;
    }

    final result = await _datasource.getUserById(userId);

    return result.fold((error) {
      if (error.contains('not found') || error.contains('404')) {
        _logger?.debug('User not found: $userId');
        return null;
      }
      _logger?.error(
        'Failed to get user by ID',
        extra: {'userId': userId, 'error': error},
      );
      return null;
    }, (response) => _toUserSearch(response));
  }

  /// Get multiple users by IDs (batch fetch)
  ///
  /// **UNBLOCKS TaggedUsersChips** - Efficient batch lookup.
  /// Uses POST /users/batch endpoint instead of N individual GET requests.
  ///
  /// Returns only users that were successfully fetched.
  /// Failed users are silently ignored (graceful degradation).
  Future<List<UserSearch>> getUsersByIds(List<String> userIds) async {
    if (userIds.isEmpty) {
      return [];
    }

    _logger?.info('Batch fetching ${userIds.length} users');

    final result = await _datasource.getMultipleUsers(userIds);

    return result.fold((error) {
      _logger?.error(
        'Failed to batch fetch users',
        extra: {'error': error, 'count': userIds.length},
      );
      return [];
    }, (responses) => responses.map(_toUserSearch).toList());
  }

  /// Search users by username
  ///
  /// For user search/autocomplete use cases (e.g., user selection modal).
  Future<List<UserSearch>> searchUsers({
    required String query,
    int limit = 20,
  }) async {
    if (query.trim().isEmpty) {
      return [];
    }

    final result = await _datasource.searchUsers(query: query, limit: limit);

    return result.fold((error) {
      _logger?.error(
        'Failed to search users',
        extra: {'query': query, 'error': error},
      );
      return [];
    }, (responses) => responses.map(_toUserSearch).toList());
  }

  /// Convert UserApiResponse to UserSearch (lightweight entity)
  ///
  /// OWNER TRUTH: UserSearch carries only the public identity scalars
  /// (userId, username, avatarUrl). fullName is private and must not
  /// surface here; verification status is not part of the search
  /// projection. See [UserSearch] doctrine comment.
  UserSearch _toUserSearch(UserApiResponse response) {
    final profile = response.profile;
    return UserSearch(
      userId: response.id,
      username: profile?.username ?? '',
      avatarUrl: profile?.avatarUrl,
    );
  }
}
