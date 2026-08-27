/// Canonical Google Maps & Places client configuration.
///
/// This file is tracked in Git so clean clones keep the import target for the
/// mobile app. The values here are restricted client-side keys, not backend
/// secrets. Restrict them in Google Cloud by app package / bundle identifier
/// and enabled Maps / Places APIs.
class GoogleConfig {
  GoogleConfig._();

  /// Google Maps API key for Android.
  static const String androidApiKey = 'PASTE_YOUR_API_KEY_HERE';

  /// Google Maps API key for iOS.
  static const String iosApiKey = 'PASTE_YOUR_API_KEY_HERE';

  /// Google Places API key used by shared map widgets.
  static const String placesApiKey = 'PASTE_YOUR_API_KEY_HERE';

  /// Returns the key used by the shared map widgets.
  static String get apiKey {
    return placesApiKey;
  }

  /// Checks whether the key has been configured with a real value.
  static bool get isConfigured {
    return !placesApiKey.contains('YOUR_') &&
        placesApiKey.isNotEmpty &&
        placesApiKey.length > 20;
  }

  /// Returns a user-friendly error message for missing client config.
  static String get configurationError {
    if (!isConfigured) {
      return 'Google Maps API key belum dikonfigurasi.\n'
          'Isi lib/core/src/config/google_config.dart dengan client key yang '
          'sudah dibatasi di Google Cloud.';
    }
    return '';
  }
}
