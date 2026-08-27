import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';

/// Avatar Cache Service
///
/// **R2.3 PROFILE DOMAIN EXTRACTION:**
/// This service is the honest owner of avatar URL caching.
/// Replaces UserAvatarApiService from shared (deprecated).
///
/// Avatar is profile data - fetching and caching avatars
/// belongs to the profile domain, not shared utilities.
///
/// Hybrid strategy:
/// 1. Check memory cache (instant)
/// 2. Fetch from API user profile endpoint (fresh)
/// 3. Cache result for performance
class AvatarCacheService {
  final UserApiDatasource _datasource;
  final ILoggerService? _logger;

  // Memory cache for avatar URLs
  static final Map<String, String?> _avatarCache = {};
  static final Map<String, DateTime> _cacheTimestamps = {};

  // Cache TTL: 5 minutes
  static const Duration _cacheTTL = Duration(minutes: 5);

  AvatarCacheService({
    required UserApiDatasource datasource,
    ILoggerService? logger,
  }) : _datasource = datasource,
       _logger = logger;

  /// Get avatar URL for user with caching strategy
  ///
  /// GET /api/v1/users/{userId}
  ///
  /// Returns:
  /// - Cached avatar if still valid
  /// - Fresh avatar from API if cache expired
  /// - null if user not found or error
  Future<String?> getUserAvatarUrl(String userId) async {
    try {
      // Check cache first
      if (_isCacheValid(userId)) {
        _logger?.info('📸 Avatar cache hit for user: $userId');
        return _avatarCache[userId];
      }

      // Fetch fresh from API
      _logger?.info('🔄 Fetching fresh avatar for user: $userId');

      final result = await _datasource.getUserById(userId);

      return result.fold(
        (error) {
          _logger?.error(
            'Failed to fetch avatar for user: $userId',
            extra: {'error': error},
          );
          return null;
        },
        (response) {
          final avatarUrl = response.profile?.avatarUrl;

          _logger?.info(
            'API result - user exists: $userId, avatar: $avatarUrl',
          );

          // Update cache
          _avatarCache[userId] = avatarUrl;
          _cacheTimestamps[userId] = DateTime.now();

          _logger?.info('✅ Avatar fetched and cached for user: $userId');
          return avatarUrl;
        },
      );
    } catch (e) {
      _logger?.error(
        'Failed to fetch avatar for user: $userId',
        extra: {'error': e.toString()},
      );
      return null;
    }
  }

  /// Batch fetch multiple user avatars (for feed performance)
  Future<Map<String, String?>> getUserAvatarUrls(List<String> userIds) async {
    try {
      final results = <String, String?>{};
      final uncachedUserIds = <String>[];

      // Separate cached vs uncached
      for (final userId in userIds) {
        if (_isCacheValid(userId)) {
          results[userId] = _avatarCache[userId];
        } else {
          uncachedUserIds.add(userId);
        }
      }

      // Batch fetch uncached avatars
      if (uncachedUserIds.isNotEmpty) {
        _logger?.info('🔄 Batch fetching ${uncachedUserIds.length} avatars');

        final result = await _datasource.getMultipleUsers(uncachedUserIds);

        result.fold(
          (error) {
            _logger?.error(
              'Failed to batch fetch avatars',
              extra: {'error': error, 'userIds': userIds},
            );
          },
          (responses) {
            for (final response in responses) {
              final avatarUrl = response.profile?.avatarUrl;
              results[response.id] = avatarUrl;
              _avatarCache[response.id] = avatarUrl;
              _cacheTimestamps[response.id] = DateTime.now();
            }
          },
        );
      }

      return results;
    } catch (e) {
      _logger?.error(
        'Failed to batch fetch avatars',
        extra: {'error': e.toString(), 'userIds': userIds},
      );
      return {};
    }
  }

  /// Pre-warm cache for frequently viewed users
  Future<void> preloadAvatars(List<String> userIds) async {
    await getUserAvatarUrls(userIds);
  }

  /// Clear cache for specific user (e.g., after avatar update)
  void clearUserCache(String userId) {
    _avatarCache.remove(userId);
    _cacheTimestamps.remove(userId);
    _logger?.info('Cleared avatar cache for user: $userId');
  }

  /// Clear all cache
  void clearAllCache() {
    final count = _avatarCache.length;
    _avatarCache.clear();
    _cacheTimestamps.clear();
    _logger?.info('Cleared all avatar cache ($count entries)');
  }

  /// Check if cache is still valid for user
  bool _isCacheValid(String userId) {
    if (!_avatarCache.containsKey(userId) ||
        !_cacheTimestamps.containsKey(userId)) {
      return false;
    }

    final cacheTime = _cacheTimestamps[userId]!;
    final now = DateTime.now();
    return now.difference(cacheTime) < _cacheTTL;
  }

  /// Get cache stats for debugging
  Map<String, dynamic> getCacheStats() {
    final now = DateTime.now();
    int validEntries = 0;
    int expiredEntries = 0;

    for (final userId in _cacheTimestamps.keys) {
      final cacheTime = _cacheTimestamps[userId]!;
      if (now.difference(cacheTime) < _cacheTTL) {
        validEntries++;
      } else {
        expiredEntries++;
      }
    }

    return {
      'total_entries': _avatarCache.length,
      'valid_entries': validEntries,
      'expired_entries': expiredEntries,
      'cache_ttl_minutes': _cacheTTL.inMinutes,
    };
  }

  /// Check if user has cached avatar
  bool hasCachedAvatar(String userId) {
    return _isCacheValid(userId) && _avatarCache[userId] != null;
  }
}
