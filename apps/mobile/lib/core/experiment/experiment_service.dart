import 'feature_flag.dart';

/// Core Experiment / A-B Testing Service Interface
///
/// Pure interface without Firebase dependencies.
/// Implementation will be provided by FirebaseRemoteConfigImpl.
///
/// **Location:** lib/core/experiment/experiment_service.dart
/// **Implementation:** lib/core/experiment/firebase_remote_config_impl.dart
abstract class ExperimentService {
  /// Check if a feature flag is enabled.
  bool isEnabled(FeatureFlag flag);

  /// Get string value for a key.
  String getString(String key, {String defaultValue = ''});

  /// Get int value for a key.
  int getInt(String key, {int defaultValue = 0});

  /// Get double value for a key.
  double getDouble(String key, {double defaultValue = 0.0});

  /// Get bool value for a key.
  bool getBool(String key, {bool defaultValue = false});

  /// Fetch and activate latest config.
  Future<bool> fetchAndActivate();

  /// Initialize with default values.
  Future<void> initialize(Map<String, dynamic> defaults);

  /// Set config settings.
  Future<void> setConfigSettings({
    Duration? minimumFetchInterval,
    Duration? fetchTimeout,
  });
}
