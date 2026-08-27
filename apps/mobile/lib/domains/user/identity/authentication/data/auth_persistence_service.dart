import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:firebase_auth/firebase_auth.dart';

/// Service untuk mengelola persistence authentication
///
/// Features:
/// - Remember me functionality
/// - Auto-login management
/// - Secure logout dengan clear preferences
class AuthPersistenceService {
  static const String _rememberMeKey = 'remember_me';
  static const String _lastLoginEmailKey = 'last_login_email';

  /// Set remember me preference
  static Future<void> setRememberMe(bool remember, {String? email}) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool(_rememberMeKey, remember);

    if (remember && email != null) {
      await prefs.setString(_lastLoginEmailKey, email);
    } else {
      await prefs.remove(_lastLoginEmailKey);
    }
  }

  /// Get remember me preference
  static Future<bool> getRememberMe() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getBool(_rememberMeKey) ?? false;
  }

  /// Get last login email (jika remember me active)
  static Future<String?> getLastLoginEmail() async {
    final prefs = await SharedPreferences.getInstance();
    final rememberMe = await getRememberMe();

    if (rememberMe) {
      return prefs.getString(_lastLoginEmailKey);
    }
    return null;
  }

  /// Check if user should stay logged in
  static Future<bool> shouldStayLoggedIn() async {
    final rememberMe = await getRememberMe();
    final currentUser = FirebaseAuth.instance.currentUser;

    return rememberMe && currentUser != null;
  }

  /// Clear all auth preferences (logout)
  static Future<void> clearAuthPreferences() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_rememberMeKey);
    await prefs.remove(_lastLoginEmailKey);
  }

  /// Set Firebase Auth persistence
  static Future<void> setFirebaseAuthPersistence(bool persistent) async {
    try {
      if (persistent) {
        // Firebase Auth by default persists on mobile, but we can be explicit
        await FirebaseAuth.instance.setPersistence(Persistence.LOCAL);
      } else {
        await FirebaseAuth.instance.setPersistence(Persistence.NONE);
      }
    } catch (e) {
      // setPersistence() is only supported on web platforms
      // On mobile/desktop, Firebase Auth persists by default
      // So we can safely ignore this error
      debugPrint('setPersistence not supported on this platform: $e');
    }
  }
}
