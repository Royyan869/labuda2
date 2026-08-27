import 'dart:convert';
import 'package:shared_preferences/shared_preferences.dart';

/// Service untuk caching blurhash values locally
///
/// Features:
/// - Cache blurhash dengan URL sebagai key
/// - Persistent storage menggunakan SharedPreferences
/// - Auto cleanup untuk cache yang expired
/// - Memory-efficient dengan size limits
class BlurhashCacheService {
  static const String _cacheKeyPrefix = 'blurhash_cache_';
  static const String _metaKey = 'blurhash_cache_meta';
  static const int _maxCacheEntries = 500; // Limit untuk performa
  static const Duration _cacheExpiration = Duration(days: 30); // 30 hari

  static BlurhashCacheService? _instance;
  static BlurhashCacheService get instance =>
      _instance ??= BlurhashCacheService._();

  BlurhashCacheService._();

  /// Get blurhash from cache
  Future<String?> getBlurhash(String imageUrl) async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final cacheKey = _getCacheKey(imageUrl);
      final cachedData = prefs.getString(cacheKey);

      if (cachedData == null) return null;

      final data = jsonDecode(cachedData);
      final timestamp = DateTime.parse(data['timestamp']);

      // Check if cache is expired
      if (DateTime.now().difference(timestamp) > _cacheExpiration) {
        await _removeCacheEntry(cacheKey);
        return null;
      }

      return data['blurhash'] as String?;
    } catch (e) {
      // Silently fail - cache is not critical
      return null;
    }
  }

  /// Store blurhash in cache
  Future<void> setBlurhash(String imageUrl, String blurhash) async {
    try {
      final prefs = await SharedPreferences.getInstance();

      // Check cache size and cleanup if needed
      await _cleanupCacheIfNeeded();

      final cacheKey = _getCacheKey(imageUrl);
      final data = {
        'blurhash': blurhash,
        'timestamp': DateTime.now().toIso8601String(),
        'url': imageUrl,
      };

      await prefs.setString(cacheKey, jsonEncode(data));
      await _updateCacheMeta(cacheKey);
    } catch (e) {
      // Silently fail - cache is not critical
    }
  }

  /// Clear all blurhash cache
  Future<void> clearCache() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final meta = await _getCacheMeta();

      for (final key in meta.keys) {
        await prefs.remove(key);
      }

      await prefs.remove(_metaKey);
    } catch (e) {
      // Silently fail
    }
  }

  /// Get cache statistics for debugging
  Future<Map<String, dynamic>> getCacheStats() async {
    try {
      final meta = await _getCacheMeta();
      final validEntries = <String>[];
      final expiredEntries = <String>[];

      for (final entry in meta.entries) {
        final timestamp = DateTime.parse(entry.value);
        if (DateTime.now().difference(timestamp) > _cacheExpiration) {
          expiredEntries.add(entry.key);
        } else {
          validEntries.add(entry.key);
        }
      }

      return {
        'total_entries': meta.length,
        'valid_entries': validEntries.length,
        'expired_entries': expiredEntries.length,
        'cache_size_limit': _maxCacheEntries,
      };
    } catch (e) {
      return {'error': e.toString()};
    }
  }

  /// Generate cache key from URL
  String _getCacheKey(String imageUrl) {
    final uri = Uri.parse(imageUrl);
    final path = uri.path.split('/').last;
    return '$_cacheKeyPrefix${path.hashCode}';
  }

  /// Update cache metadata
  Future<void> _updateCacheMeta(String cacheKey) async {
    final prefs = await SharedPreferences.getInstance();
    final meta = await _getCacheMeta();

    meta[cacheKey] = DateTime.now().toIso8601String();
    await prefs.setString(_metaKey, jsonEncode(meta));
  }

  /// Get cache metadata
  Future<Map<String, String>> _getCacheMeta() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final metaString = prefs.getString(_metaKey);

      if (metaString == null) return {};

      final metaData = jsonDecode(metaString) as Map<String, dynamic>;
      return metaData.cast<String, String>();
    } catch (e) {
      return {};
    }
  }

  /// Remove specific cache entry
  Future<void> _removeCacheEntry(String cacheKey) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(cacheKey);

    final meta = await _getCacheMeta();
    meta.remove(cacheKey);
    await prefs.setString(_metaKey, jsonEncode(meta));
  }

  /// Cleanup cache if size exceeds limit
  Future<void> _cleanupCacheIfNeeded() async {
    final meta = await _getCacheMeta();

    if (meta.length < _maxCacheEntries) return;

    // Sort by timestamp and remove oldest entries
    final sortedEntries = meta.entries.toList()
      ..sort(
        (a, b) => DateTime.parse(a.value).compareTo(DateTime.parse(b.value)),
      );

    final entriesToRemove = sortedEntries.take(100); // Remove 100 oldest
    final prefs = await SharedPreferences.getInstance();

    for (final entry in entriesToRemove) {
      await prefs.remove(entry.key);
      meta.remove(entry.key);
    }

    await prefs.setString(_metaKey, jsonEncode(meta));
  }
}
