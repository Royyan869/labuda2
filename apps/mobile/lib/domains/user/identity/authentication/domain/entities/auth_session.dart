/// Domain entity representing a single active device/refresh-session.
///
/// Maps to the backend safe response fields returned by
/// GET /api/v1/auth/sessions (mapSessionDeviceSummaries).
///
/// Sensitive fields (token_hash, jti, ip_hash) are never present here.
class AuthSession {
  /// Server-side refresh session family identifier.
  /// Used as the key for DELETE /auth/sessions/:family_id.
  final String familyId;

  /// Optional device identifier (set by mobile during login).
  final String? deviceId;

  /// Optional human-readable device name.
  final String? deviceName;

  /// Optional platform string ("android", "ios", "web").
  final String? platform;

  /// Optional app version string.
  final String? appVersion;

  /// When this session family was first issued.
  final DateTime issuedAt;

  /// When the refresh token for this session expires.
  final DateTime expiresAt;

  /// When this session was last used (refresh). May be null if never refreshed.
  final DateTime? lastUsedAt;

  /// Whether the FCM token associated with this device is still active.
  /// Null if no FCM token was registered for this session.
  final bool? fcmTokenActive;

  const AuthSession({
    required this.familyId,
    this.deviceId,
    this.deviceName,
    this.platform,
    this.appVersion,
    required this.issuedAt,
    required this.expiresAt,
    this.lastUsedAt,
    this.fcmTokenActive,
  });

  /// The most recent activity time for display.
  /// Prefers lastUsedAt, falls back to issuedAt.
  DateTime get lastActivity => lastUsedAt ?? issuedAt;

  /// Display label for the device.
  ///
  /// Priority:
  ///   1. deviceName
  ///   2. platform (capitalized)
  ///   3. "Perangkat tidak dikenal"
  String get deviceLabel {
    if (deviceName != null && deviceName!.isNotEmpty) return deviceName!;
    if (platform != null && platform!.isNotEmpty) {
      return _capitalizePlatform(platform!);
    }
    return 'Perangkat tidak dikenal';
  }

  String _capitalizePlatform(String p) {
    switch (p.toLowerCase()) {
      case 'android':
        return 'Android';
      case 'ios':
        return 'iOS';
      case 'web':
        return 'Web';
      default:
        return p.length > 1
            ? '${p[0].toUpperCase()}${p.substring(1)}'
            : p.toUpperCase();
    }
  }
}
