import 'package:labuda/core/common/result.dart';

abstract class ILocalStorageService {
  Future<Result<void>> initialize();

  // String operations
  Future<Result<void>> setString(String key, String value);
  Future<Result<String?>> getString(String key);

  // Integer operations
  Future<Result<void>> setInt(String key, int value);
  Future<Result<int?>> getInt(String key);

  // Boolean operations
  Future<Result<void>> setBool(String key, bool value);
  Future<Result<bool?>> getBool(String key);

  // Double operations
  Future<Result<void>> setDouble(String key, double value);
  Future<Result<double?>> getDouble(String key);

  // List operations
  Future<Result<void>> setStringList(String key, List<String> value);
  Future<Result<List<String>?>> getStringList(String key);

  // Object operations (JSON)
  Future<Result<void>> setObject(String key, Map<String, dynamic> value);
  Future<Result<Map<String, dynamic>?>> getObject(String key);

  // Secure storage operations
  Future<Result<void>> setSecureString(String key, String value);
  Future<Result<String?>> getSecureString(String key);

  // Management operations
  Future<Result<void>> remove(String key);
  Future<Result<void>> removeSecure(String key);
  Future<Result<void>> clear();
  Future<Result<void>> clearSecure();
  Future<Result<bool>> containsKey(String key);
  Future<Result<Set<String>>> getKeys();

  // Auth-specific operations (canonical helpers — non-termination reads)
  Future<Result<void>> setAuthToken(String token);
  Future<Result<String?>> getAuthToken();

  Future<Result<void>> setRefreshToken(String token);
  Future<Result<String?>> getRefreshToken();

  Future<Result<void>> setUserSession(Map<String, dynamic> session);
  Future<Result<Map<String, dynamic>?>> getUserSession();

  // Restricted profile-completion credential (isolated from normal access token)
  Future<Result<void>> setRestrictedToken(String token);
  Future<Result<String?>> getRestrictedToken();
  Future<Result<void>> clearRestrictedToken();

  // Canonical Labuda credential operations
  Future<Result<void>> saveLabudaCredential(String accessToken, String refreshToken);
  Future<Result<String?>> readLabudaAccessToken();
  Future<Result<String?>> readLabudaRefreshToken();
  Future<Result<void>> clearLabudaCredential();
  Future<Result<bool>> hasLabudaCredential();
}

class StorageKeys {
  static const String authToken = 'auth_token';
  static const String refreshToken = 'refresh_token';
  static const String restrictedToken = 'restricted_token';
  static const String userSession = 'user_session';
  static const String userPreferences = 'user_preferences';
  // REMOVED: onboardingCompleted - was never used, app entry is controlled by AuthController
  static const String themeMode = 'theme_mode';
  static const String language = 'language';
  static const String lastSyncTime = 'last_sync_time';
  static const String deviceId = 'device_id';
  static const String pushToken = 'push_token';
  static const String lastVerificationEmailSentAt =
      'last_verification_email_sent_at';
}
