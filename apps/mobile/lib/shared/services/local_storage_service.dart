import 'dart:convert';

import 'package:shared_preferences/shared_preferences.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'package:labuda/core/core.dart';

class LocalStorageService implements ILocalStorageService {
  static final LocalStorageService _instance = LocalStorageService._internal();
  factory LocalStorageService() => _instance;
  LocalStorageService._internal();

  SharedPreferences? _prefs;
  static const FlutterSecureStorage _secureStorage = FlutterSecureStorage(
    aOptions: AndroidOptions(),
    iOptions: IOSOptions(
      accessibility: KeychainAccessibility.first_unlock_this_device,
    ),
  );

  @override
  Future<Result<void>> initialize() async {
    try {
      _prefs = await SharedPreferences.getInstance();
      return Result.success(null);
    } catch (e) {
      return Result.error(
        'Failed to initialize local storage: ${e.toString()}',
      );
    }
  }

  // String operations
  @override
  Future<Result<void>> setString(String key, String value) async {
    try {
      if (_prefs == null) {
        final initResult = await initialize();
        if (initResult.isError) return Result.error(initResult.error!);
      }

      final success = await _prefs!.setString(key, value);
      if (success) {
        return Result.success(null);
      } else {
        return Result.error('Failed to save string value');
      }
    } catch (e) {
      return Result.error('Error saving string: ${e.toString()}');
    }
  }

  @override
  Future<Result<String?>> getString(String key) async {
    try {
      if (_prefs == null) {
        final initResult = await initialize();
        if (initResult.isError) return Result.error(initResult.error!);
      }

      final value = _prefs!.getString(key);
      return Result.success(value);
    } catch (e) {
      return Result.error('Error retrieving string: ${e.toString()}');
    }
  }

  // Integer operations
  @override
  Future<Result<void>> setInt(String key, int value) async {
    try {
      if (_prefs == null) {
        final initResult = await initialize();
        if (initResult.isError) return Result.error(initResult.error!);
      }

      final success = await _prefs!.setInt(key, value);
      if (success) {
        return Result.success(null);
      } else {
        return Result.error('Failed to save integer value');
      }
    } catch (e) {
      return Result.error('Error saving integer: ${e.toString()}');
    }
  }

  @override
  Future<Result<int?>> getInt(String key) async {
    try {
      if (_prefs == null) {
        final initResult = await initialize();
        if (initResult.isError) return Result.error(initResult.error!);
      }

      final value = _prefs!.getInt(key);
      return Result.success(value);
    } catch (e) {
      return Result.error('Error retrieving integer: ${e.toString()}');
    }
  }

  // Boolean operations
  @override
  Future<Result<void>> setBool(String key, bool value) async {
    try {
      if (_prefs == null) {
        final initResult = await initialize();
        if (initResult.isError) return Result.error(initResult.error!);
      }

      final success = await _prefs!.setBool(key, value);
      if (success) {
        return Result.success(null);
      } else {
        return Result.error('Failed to save boolean value');
      }
    } catch (e) {
      return Result.error('Error saving boolean: ${e.toString()}');
    }
  }

  @override
  Future<Result<bool?>> getBool(String key) async {
    try {
      if (_prefs == null) {
        final initResult = await initialize();
        if (initResult.isError) return Result.error(initResult.error!);
      }

      final value = _prefs!.getBool(key);
      return Result.success(value);
    } catch (e) {
      return Result.error('Error retrieving boolean: ${e.toString()}');
    }
  }

  // Double operations
  @override
  Future<Result<void>> setDouble(String key, double value) async {
    try {
      if (_prefs == null) {
        final initResult = await initialize();
        if (initResult.isError) return Result.error(initResult.error!);
      }

      final success = await _prefs!.setDouble(key, value);
      if (success) {
        return Result.success(null);
      } else {
        return Result.error('Failed to save double value');
      }
    } catch (e) {
      return Result.error('Error saving double: ${e.toString()}');
    }
  }

  @override
  Future<Result<double?>> getDouble(String key) async {
    try {
      if (_prefs == null) {
        final initResult = await initialize();
        if (initResult.isError) return Result.error(initResult.error!);
      }

      final value = _prefs!.getDouble(key);
      return Result.success(value);
    } catch (e) {
      return Result.error('Error retrieving double: ${e.toString()}');
    }
  }

  // List operations
  @override
  Future<Result<void>> setStringList(String key, List<String> value) async {
    try {
      if (_prefs == null) {
        final initResult = await initialize();
        if (initResult.isError) return Result.error(initResult.error!);
      }

      final success = await _prefs!.setStringList(key, value);
      if (success) {
        return Result.success(null);
      } else {
        return Result.error('Failed to save string list');
      }
    } catch (e) {
      return Result.error('Error saving string list: ${e.toString()}');
    }
  }

  @override
  Future<Result<List<String>?>> getStringList(String key) async {
    try {
      if (_prefs == null) {
        final initResult = await initialize();
        if (initResult.isError) return Result.error(initResult.error!);
      }

      final value = _prefs!.getStringList(key);
      return Result.success(value);
    } catch (e) {
      return Result.error('Error retrieving string list: ${e.toString()}');
    }
  }

  // Object operations (JSON)
  @override
  Future<Result<void>> setObject(String key, Map<String, dynamic> value) async {
    try {
      final jsonString = jsonEncode(value);
      return await setString(key, jsonString);
    } catch (e) {
      return Result.error('Error encoding object: ${e.toString()}');
    }
  }

  @override
  Future<Result<Map<String, dynamic>?>> getObject(String key) async {
    try {
      final stringResult = await getString(key);
      if (stringResult.isError) return Result.error(stringResult.error!);

      final jsonString = stringResult.data;
      if (jsonString == null) return Result.success(null);

      final object = jsonDecode(jsonString) as Map<String, dynamic>;
      return Result.success(object);
    } catch (e) {
      return Result.error('Error decoding object: ${e.toString()}');
    }
  }

  // Secure storage operations
  @override
  Future<Result<void>> setSecureString(String key, String value) async {
    try {
      await _secureStorage.write(key: key, value: value);
      return Result.success(null);
    } catch (e) {
      return Result.error('Error saving secure string: ${e.toString()}');
    }
  }

  @override
  Future<Result<String?>> getSecureString(String key) async {
    try {
      final value = await _secureStorage.read(key: key);
      return Result.success(value);
    } catch (e) {
      return Result.error('Error retrieving secure string: ${e.toString()}');
    }
  }

  // Management operations
  @override
  Future<Result<void>> remove(String key) async {
    try {
      if (_prefs == null) {
        final initResult = await initialize();
        if (initResult.isError) return Result.error(initResult.error!);
      }

      final success = await _prefs!.remove(key);
      if (success) {
        return Result.success(null);
      } else {
        return Result.error('Failed to remove key');
      }
    } catch (e) {
      return Result.error('Error removing key: ${e.toString()}');
    }
  }

  @override
  Future<Result<void>> removeSecure(String key) async {
    try {
      await _secureStorage.delete(key: key);
      return Result.success(null);
    } catch (e) {
      return Result.error('Error removing secure key: ${e.toString()}');
    }
  }

  @override
  Future<Result<void>> clear() async {
    try {
      if (_prefs == null) {
        final initResult = await initialize();
        if (initResult.isError) return Result.error(initResult.error!);
      }

      final success = await _prefs!.clear();
      if (success) {
        return Result.success(null);
      } else {
        return Result.error('Failed to clear storage');
      }
    } catch (e) {
      return Result.error('Error clearing storage: ${e.toString()}');
    }
  }

  @override
  Future<Result<void>> clearSecure() async {
    try {
      await _secureStorage.deleteAll();
      return Result.success(null);
    } catch (e) {
      return Result.error('Error clearing secure storage: ${e.toString()}');
    }
  }

  @override
  Future<Result<bool>> containsKey(String key) async {
    try {
      if (_prefs == null) {
        final initResult = await initialize();
        if (initResult.isError) return Result.error(initResult.error!);
      }

      final contains = _prefs!.containsKey(key);
      return Result.success(contains);
    } catch (e) {
      return Result.error('Error checking key existence: ${e.toString()}');
    }
  }

  @override
  Future<Result<Set<String>>> getKeys() async {
    try {
      if (_prefs == null) {
        final initResult = await initialize();
        if (initResult.isError) return Result.error(initResult.error!);
      }

      final keys = _prefs!.getKeys();
      return Result.success(keys);
    } catch (e) {
      return Result.error('Error retrieving keys: ${e.toString()}');
    }
  }

  // Auth-specific operations
  @override
  Future<Result<void>> setAuthToken(String token) async {
    return await setSecureString(StorageKeys.authToken, token);
  }

  @override
  Future<Result<String?>> getAuthToken() async {
    return await getSecureString(StorageKeys.authToken);
  }

  @override
  Future<Result<void>> setRefreshToken(String token) async {
    return await setSecureString(StorageKeys.refreshToken, token);
  }

  @override
  Future<Result<String?>> getRefreshToken() async {
    return await getSecureString(StorageKeys.refreshToken);
  }

  @override
  Future<Result<void>> setUserSession(Map<String, dynamic> session) async {
    return await setObject(StorageKeys.userSession, session);
  }

  @override
  Future<Result<Map<String, dynamic>?>> getUserSession() async {
    return await getObject(StorageKeys.userSession);
  }

  // Restricted profile-completion credential (isolated from normal access token)
  @override
  Future<Result<void>> setRestrictedToken(String token) async {
    return await setSecureString(StorageKeys.restrictedToken, token);
  }

  @override
  Future<Result<String?>> getRestrictedToken() async {
    return await getSecureString(StorageKeys.restrictedToken);
  }

  @override
  Future<Result<void>> clearRestrictedToken() async {
    return await removeSecure(StorageKeys.restrictedToken);
  }

  // Canonical Labuda credential operations
  @override
  Future<Result<void>> saveLabudaCredential(
    String accessToken,
    String refreshToken,
  ) async {
    if (accessToken.isEmpty || refreshToken.isEmpty) {
      return Result.error('Both access and refresh tokens are required');
    }

    final accessResult = await setSecureString(StorageKeys.authToken, accessToken);
    if (accessResult.isError) return accessResult;

    final refreshResult = await setSecureString(StorageKeys.refreshToken, refreshToken);
    if (refreshResult.isError) return refreshResult;

    return Result.success(null);
  }

  @override
  Future<Result<String?>> readLabudaAccessToken() async {
    return await getSecureString(StorageKeys.authToken);
  }

  @override
  Future<Result<String?>> readLabudaRefreshToken() async {
    return await getSecureString(StorageKeys.refreshToken);
  }

  @override
  Future<Result<void>> clearLabudaCredential() async {
    final clearAccess = await removeSecure(StorageKeys.authToken);
    final clearRefresh = await removeSecure(StorageKeys.refreshToken);

    if (clearAccess.isError) return clearAccess;
    if (clearRefresh.isError) return clearRefresh;

    return Result.success(null);
  }

  @override
  Future<Result<bool>> hasLabudaCredential() async {
    final access = await readLabudaAccessToken();
    if (access.isError || access.data == null || access.data!.isEmpty) {
      return Result.success(false);
    }

    final refresh = await readLabudaRefreshToken();
    if (refresh.isError || refresh.data == null || refresh.data!.isEmpty) {
      return Result.success(false);
    }

    return Result.success(true);
  }
}
