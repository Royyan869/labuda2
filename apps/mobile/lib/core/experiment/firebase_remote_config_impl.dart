import 'package:firebase_remote_config/firebase_remote_config.dart';
import 'experiment_service.dart';
import 'feature_flag.dart';

/// Firebase Remote Config Implementation
///
/// **Location:** lib/core/experiment/firebase_remote_config_impl.dart
/// **Interface:** lib/core/experiment/experiment_service.dart
///
/// Firebase SDK is ONLY allowed in this file.
class FirebaseRemoteConfigImpl implements ExperimentService {
  final FirebaseRemoteConfig _remoteConfig;

  FirebaseRemoteConfigImpl(this._remoteConfig);

  /// Factory for default instance
  factory FirebaseRemoteConfigImpl.instance() {
    return FirebaseRemoteConfigImpl(FirebaseRemoteConfig.instance);
  }

  @override
  bool isEnabled(FeatureFlag flag) {
    final key = 'feature_${flag.name}';
    return getBool(key, defaultValue: false);
  }

  @override
  String getString(String key, {String defaultValue = ''}) {
    final value = _remoteConfig.getString(key);
    return value.isEmpty ? defaultValue : value;
  }

  @override
  int getInt(String key, {int defaultValue = 0}) {
    return _remoteConfig.getInt(key);
  }

  @override
  double getDouble(String key, {double defaultValue = 0.0}) {
    return _remoteConfig.getDouble(key);
  }

  @override
  bool getBool(String key, {bool defaultValue = false}) {
    return _remoteConfig.getBool(key);
  }

  @override
  Future<bool> fetchAndActivate() async {
    return await _remoteConfig.fetchAndActivate();
  }

  @override
  Future<void> initialize(Map<String, dynamic> defaults) async {
    await _remoteConfig.setDefaults(defaults);
  }

  @override
  Future<void> setConfigSettings({
    Duration? minimumFetchInterval,
    Duration? fetchTimeout,
  }) async {
    await _remoteConfig.setConfigSettings(
      RemoteConfigSettings(
        fetchTimeout: fetchTimeout ?? const Duration(seconds: 60),
        minimumFetchInterval: minimumFetchInterval ?? const Duration(hours: 1),
      ),
    );
  }
}
