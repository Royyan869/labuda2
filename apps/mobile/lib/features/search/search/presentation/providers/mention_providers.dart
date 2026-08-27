import 'dart:async';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/features/search/search/domain/entities/user_search.dart';

/// Mention-related providers for search domain
///
/// **R2.2 MIGRATED**: Moved from shared/providers/ to search/presentation/providers/
/// as user search for mentions is search domain functionality.

// =====================
// MENTION SEARCH PROVIDER
// =====================

/// Debounced provider untuk search users by username atau full name untuk mention
///
/// Features:
/// - 300ms debounce untuk avoid excessive queries
/// - Cache results for better performance
/// - Digunakan untuk suggestion dropdown saat user ketik @username
///
/// Menggunakan API backend alih-alih langsung ke Firestore
final mentionUserSearchProvider =
    FutureProvider.family<List<UserSearch>, MentionSearchParams>((
      ref,
      params,
    ) async {
      if (params.query.trim().isEmpty) {
        return [];
      }

      // Debounce: wait 300ms before executing search
      await Future.delayed(const Duration(milliseconds: 300));

      try {
        final apiClient = ref.watch(apiClientProvider);
        final logger = ref.watch(loggerServiceProvider);

        logger.info('Searching users for mention: ${params.query}');

        final query = params.query.toLowerCase().trim();

        final response = await apiClient.get(
          '/users/search',
          queryParameters: {'q': query, 'limit': '100'},
        );

        final data = response.data as Map<String, dynamic>;
        final usersList = data['users'] as List<dynamic>?;

        if (usersList == null) {
          return [];
        }

        final userMap = <String, UserSearch>{};

        for (final json in usersList) {
          try {
            final userData = json as Map<String, dynamic>;

            // Skip users with empty username
            final username = userData['username'] as String? ?? '';
            if (username.isEmpty) {
              continue;
            }

            // OWNER TRUTH: public identity is username only — mention search
            // matches solely on username (fullName/email are private/contractual
            // and must not surface as public identity discovery).
            final canonicalUsername = username.toLowerCase();
            if (!_isMentionableUsername(canonicalUsername)) {
              continue;
            }

            if (canonicalUsername.contains(query)) {
              final user = UserSearch(
                userId: userData['id'] as String,
                username: canonicalUsername,
                avatarUrl:
                    userData['avatar_url'] as String? ??
                    userData['avatarUrl'] as String?,
              );

              // Filter by allowed user IDs if specified (for group chat)
              if (params.allowedUserIds != null &&
                  !params.allowedUserIds!.contains(user.userId)) {
                continue;
              }

              userMap[user.userId] = user;
            }
          } catch (e) {
            // Silently skip malformed documents
            logger.warning('Error parsing user data: $e');
            continue;
          }
        }

        var results = userMap.values.toList();

        // Sort: exact username match first, then alphabetically
        results.sort((a, b) {
          final aUsernameMatch = a.username.toLowerCase() == query;
          final bUsernameMatch = b.username.toLowerCase() == query;

          if (aUsernameMatch && !bUsernameMatch) return -1;
          if (!aUsernameMatch && bUsernameMatch) return 1;

          // Both match or both don't match - sort alphabetically
          return a.username.compareTo(b.username);
        });

        final finalResults = results
            .take(10)
            .toList(); // Limit to 10 suggestions

        logger.info('Found ${finalResults.length} users for mention');
        return finalResults;
    } catch (e) {
        // Return empty list on error
        ref
            .read(loggerServiceProvider)
            .error(
              'Error searching users for mention',
              extra: {'error': e.toString()},
        );
        return [];
      }
    });

bool _isMentionableUsername(String username) {
  return RegExp(r'^[a-z0-9_]+$').hasMatch(username);
}

/// Parameters for mention search
class MentionSearchParams {
  final String query;
  final List<String>? allowedUserIds; // Untuk filter group members only

  const MentionSearchParams({required this.query, this.allowedUserIds});

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is MentionSearchParams &&
          runtimeType == other.runtimeType &&
          query == other.query &&
          _listEquals(allowedUserIds, other.allowedUserIds);

  @override
  int get hashCode => query.hashCode ^ (allowedUserIds?.hashCode ?? 0);

  bool _listEquals(List<String>? a, List<String>? b) {
    if (a == null) return b == null;
    if (b == null || a.length != b.length) return false;
    for (int i = 0; i < a.length; i++) {
      if (a[i] != b[i]) return false;
    }
    return true;
  }
}

// =====================
// MENTION RESOLVER PROVIDER
// =====================

/// Provider untuk resolve username ke user ID dengan caching
///
/// Features:
/// - Resolve single username to user ID
/// - Resolve multiple usernames at once (batch)
/// - LRU cache untuk avoid repeated API calls
/// - Handle special mentions (@everyone, @admins, @online)
///
/// MIGRATED: Now using API instead of Firestore
final mentionResolverProvider = Provider((ref) {
  final apiClient = ref.watch(apiClientProvider);
  final logger = ref.watch(loggerServiceProvider);
  return MentionResolver(apiClient: apiClient, logger: logger);
});

/// Mention resolver class
class MentionResolver {
  final ApiClient _apiClient;
  final ILoggerService _logger;
  final _cache = <String, String?>{}; // username -> userId cache
  static const _cacheLimit = 100; // LRU cache limit

  MentionResolver({
    required ApiClient apiClient,
    required ILoggerService logger,
  }) : _apiClient = apiClient,
       _logger = logger;

  /// Resolve single username to user ID
  ///
  /// Returns userId if found, null if not found
  /// Uses cache to avoid repeated queries
  Future<String?> resolveUsername(String username) async {
    // Check cache first
    if (_cache.containsKey(username)) {
      _logger.info('Username resolved from cache: $username');
      return _cache[username];
    }

    try {
      final normalizedUsername = username.toLowerCase().trim();
      if (normalizedUsername.isEmpty ||
          !_isMentionableUsername(normalizedUsername)) {
        _addToCache(username, null);
        _logger.info('Username not found: $username');
        return null;
      }

      // Use user search API to find user by username
      final response = await _apiClient.get(
        '/users/search',
        queryParameters: {'q': normalizedUsername, 'limit': '1'},
      );

      final data = response.data as Map<String, dynamic>;
      final usersList = data['users'] as List<dynamic>?;

      if (usersList == null || usersList.isEmpty) {
        // Cache null result to avoid repeated queries for non-existent users
        _addToCache(username, null);
        _logger.info('Username not found: $username');
        return null;
      }

      final firstUser = usersList.first as Map<String, dynamic>;
      final userId = firstUser['id'] as String?;

      if (userId == null) {
        _addToCache(username, null);
        return null;
      }

      // Add to cache
      _addToCache(username, userId);

      _logger.info('Username resolved: $username -> $userId');
      return userId;
    } catch (e) {
      _logger.error(
        'Error resolving username',
        extra: {'username': username, 'error': e.toString()},
      );
      // On error, don't cache and return null
      return null;
    }
  }

  /// Resolve multiple usernames to user IDs (batch operation)
  ///
  /// Returns `Map<username, userId>`
  /// Skips usernames that don't exist
  /// Uses cache for known usernames
  Future<Map<String, String>> resolveUsernames(List<String> usernames) async {
    if (usernames.isEmpty) return {};

    final result = <String, String>{};
    final uncachedUsernames = <String>[];

    // Check cache first
    for (final username in usernames) {
      if (_cache.containsKey(username)) {
        final userId = _cache[username];
        if (userId != null) {
          result[username] = userId;
        }
      } else {
        uncachedUsernames.add(username);
      }
    }

    // Fetch uncached usernames from API
    if (uncachedUsernames.isNotEmpty) {
      try {
        _logger.info(
          'Resolving ${uncachedUsernames.length} uncached usernames',
        );

        // For each username, search individually
        // TODO: Backend could add a batch endpoint for better performance
        for (final username in uncachedUsernames) {
          final userId = await resolveUsername(username);
          if (userId != null) {
            result[username] = userId;
          }
        }
      } catch (e) {
        _logger.error(
          'Error resolving usernames batch',
          extra: {'error': e.toString()},
        );
        // On error, return what we have from cache
      }
    }

    _logger.info('Resolved ${result.length}/${usernames.length} usernames');
    return result;
  }

  /// Add to cache with LRU eviction
  void _addToCache(String username, String? userId) {
    // If cache is full, remove oldest entry
    if (_cache.length >= _cacheLimit) {
      final firstKey = _cache.keys.first;
      _cache.remove(firstKey);
    }

    _cache[username] = userId;
  }

  /// Clear cache (useful for testing or after user updates)
  void clearCache() {
    _cache.clear();
    _logger.info('Mention resolver cache cleared');
  }

  /// Remove specific username from cache
  void invalidateUsername(String username) {
    _cache.remove(username);
    _logger.info('Invalidated username from cache: $username');
  }

  /// Get cache size (for debugging)
  int get cacheSize => _cache.length;
}
