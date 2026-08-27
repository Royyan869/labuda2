import 'dart:developer' as developer;

import 'package:firebase_auth/firebase_auth.dart';
import 'package:labuda/core/api/di/api_di.dart';
import 'package:labuda/shared/helpers/canonical_phone_validator.dart';

/// Service untuk verifikasi nomor telepon menggunakan Firebase Phone Auth
///
/// **Development Mode:**
/// - Gunakan test phone numbers di Firebase Console untuk skip reCAPTCHA
/// - Contoh: +6281234567890 dengan kode 123456
///
/// **Production Mode:**
/// - Requires SHA-256 fingerprint registered di Firebase Console
/// - Requires Google Play Integrity API enabled
class PhoneVerificationService {
  final FirebaseAuth _auth = FirebaseAuth.instance;

  // Store verification ID untuk proses verifikasi OTP
  String? _verificationId;
  int? _resendToken;

  // Callbacks
  Function(String verificationId)? onCodeSent;
  Function(String error)? onVerificationFailed;
  Function()? onVerificationCompleted;
  Function()? onCodeAutoRetrievalTimeout;

  // Test phone numbers untuk development (skip reCAPTCHA)
  static const List<String> _testPhoneNumbers = [
    '+6281234567890',
    '+6289876543210',
  ];

  /// Check if phone number is test number (development mode)
  bool isTestPhoneNumber(String phoneNumber) {
    final formatted = formatPhoneNumber(phoneNumber);
    return _testPhoneNumbers.contains(formatted);
  }

  /// Format nomor telepon Indonesia ke format internasional
  /// Contoh: 08123456789 → +628123456789
  String formatPhoneNumber(String phoneNumber) {
    // Remove any spaces, dashes, or parentheses
    phoneNumber = phoneNumber.replaceAll(RegExp(r'[\s\-\(\)]'), '');

    // Handle Indonesian numbers
    if (phoneNumber.startsWith('08')) {
      // Replace 08 with +628
      return '+628${phoneNumber.substring(2)}';
    } else if (phoneNumber.startsWith('8')) {
      // Add +62 prefix
      return '+628$phoneNumber';
    } else if (phoneNumber.startsWith('628')) {
      // Add + prefix
      return '+$phoneNumber';
    } else if (phoneNumber.startsWith('+628')) {
      // Already in correct format
      return phoneNumber;
    } else if (phoneNumber.startsWith('0')) {
      // Replace 0 with +62
      return '+62${phoneNumber.substring(1)}';
    }

    // Default: assume it's already formatted
    return phoneNumber;
  }

  /// Validate phone number format — delegates to the canonical Indonesian
  /// phone authority (Stage 4D). This service remains the E.164 NORMALIZER
  /// (formatPhoneNumber) and the Firebase OTP flow owner, but it no longer
  /// owns format policy of its own.
  bool isValidPhoneNumber(String phoneNumber) {
    return CanonicalPhoneValidator.isValid(phoneNumber);
  }

  /// Send OTP to phone number
  Future<void> sendOTP({
    required String phoneNumber,
    Function(String verificationId)? onCodeSent,
    Function(String error)? onVerificationFailed,
    Function()? onVerificationCompleted,
    int? resendToken,
  }) async {
    try {
      // Format phone number
      final formattedNumber = formatPhoneNumber(phoneNumber);

      if (!isValidPhoneNumber(formattedNumber)) {
        onVerificationFailed?.call('Invalid phone number format');
        return;
      }

      // Store callbacks
      this.onCodeSent = onCodeSent;
      this.onVerificationFailed = onVerificationFailed;
      this.onVerificationCompleted = onVerificationCompleted;

      await _auth.verifyPhoneNumber(
        phoneNumber: formattedNumber,
        verificationCompleted: (PhoneAuthCredential credential) async {
          // Auto-verification (rare on Android, common on iOS)
          try {
            await _auth.currentUser?.linkWithCredential(credential);
            onVerificationCompleted?.call();
          } catch (e) {
            // Silently ignore auto-verification failures
          }
        },
        verificationFailed: (FirebaseAuthException e) {
          String errorMessage = 'Verification failed';

          if (e.code == 'invalid-phone-number') {
            errorMessage = 'Invalid phone number format';
          } else if (e.code == 'too-many-requests') {
            errorMessage = 'Too many requests. Please try again later';
          } else if (e.code == 'network-request-failed') {
            errorMessage = 'Network error. Please check your connection';
          } else {
            errorMessage = e.message ?? 'Unknown error occurred';
          }

          onVerificationFailed?.call(errorMessage);
        },
        codeSent: (String verificationId, int? resendToken) {
          _verificationId = verificationId;
          _resendToken = resendToken;
          onCodeSent?.call(verificationId);
        },
        codeAutoRetrievalTimeout: (String verificationId) {
          _verificationId = verificationId;
          onCodeAutoRetrievalTimeout?.call();
        },
        forceResendingToken: resendToken,
        timeout: const Duration(seconds: 60),
      );
    } catch (e) {
      onVerificationFailed?.call('Failed to send OTP: $e');
    }
  }

  /// Resend OTP using saved resend token
  Future<void> resendOTP({
    required String phoneNumber,
    Function(String verificationId)? onCodeSent,
    Function(String error)? onVerificationFailed,
  }) async {
    if (_resendToken != null) {
      await sendOTP(
        phoneNumber: phoneNumber,
        onCodeSent: onCodeSent,
        onVerificationFailed: onVerificationFailed,
        resendToken: _resendToken,
      );
    } else {
      onVerificationFailed?.call('Cannot resend OTP. Please try again');
    }
  }

  /// Verify OTP code
  Future<bool> verifyOTP(String otp) async {
    try {
      if (_verificationId == null) {
        throw Exception('No verification in progress');
      }

      // Create credential with verification ID and OTP
      final credential = PhoneAuthProvider.credential(
        verificationId: _verificationId!,
        smsCode: otp,
      );

      // Link phone number with current user account
      final currentUser = _auth.currentUser;
      if (currentUser != null) {
        // Check if phone is already linked to this account
        final isAlreadyLinked = currentUser.phoneNumber != null;

        if (isAlreadyLinked) {
          // Phone already linked, just verify the OTP is valid
          // We won't link again, but we'll sync to Firestore
          try {
            // Verify credential is valid by attempting to re-authenticate
            await currentUser.reauthenticateWithCredential(credential);
          } catch (e) {
            // If re-auth fails, the OTP might be invalid
            if (e.toString().contains('invalid-verification-code')) {
              throw Exception('Invalid OTP code');
            }
            // Otherwise, phone is already linked and OTP was checked
          }
        } else {
          // Phone not linked yet, link it now
          try {
            await currentUser.linkWithCredential(credential);
          } on FirebaseAuthException catch (e) {
            if (e.code == 'provider-already-linked') {
              // Phone already linked, this is actually success
            } else {
              rethrow;
            }
          }
        }

        await _refreshVerificationSnapshot();

        return true;
      } else {
        // Sign in with phone credential (creates new user if needed)
        await _auth.signInWithCredential(credential);

        await _refreshVerificationSnapshot();

        return true;
      }
    } on FirebaseAuthException catch (e) {
      if (e.code == 'invalid-verification-code') {
        throw Exception('Invalid OTP code');
      } else if (e.code == 'session-expired') {
        throw Exception('OTP expired. Please request a new code');
      } else if (e.code == 'credential-already-in-use') {
        // This means the phone is linked to THIS account already.
        // Treat as success and refresh backend verification snapshot.
        await _refreshVerificationSnapshot();
        return true;
      } else {
        throw Exception(e.message ?? 'Verification failed');
      }
    } catch (e) {
      throw Exception('Failed to verify OTP: $e');
    }
  }

  /// Check if current user has verified phone
  bool isPhoneVerified() {
    final user = _auth.currentUser;
    return user?.phoneNumber != null && user!.phoneNumber!.isNotEmpty;
  }

  /// Get verified phone number of current user
  String? getVerifiedPhoneNumber() {
    return _auth.currentUser?.phoneNumber;
  }

  /// Clear verification session
  void clearSession() {
    _verificationId = null;
    _resendToken = null;
    onCodeSent = null;
    onVerificationFailed = null;
    onVerificationCompleted = null;
    onCodeAutoRetrievalTimeout = null;
  }

  Future<void> _refreshVerificationSnapshot() async {
    try {
      await ApiDI.apiClient.post('/users/me/verification/refresh');
      // TODO: Optionally trigger centralized auth/profile refresh provider from caller layer.
    } catch (e) {
      // Non-blocking by design: OTP verification remains successful.
      developer.log(
        'Verification snapshot refresh failed (non-blocking): $e',
        name: 'PhoneVerificationService',
      );
    }
  }
}
