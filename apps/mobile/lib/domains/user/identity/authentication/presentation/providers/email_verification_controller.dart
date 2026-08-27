/// Email Verification Controller
///
/// Controller for email verification state management.
/// Handles refresh of email verification status from Firebase, then syncs the
/// hydrated authenticated account only when verification is actually proven.
/// Also handles sending verification emails.
///
/// Key behaviors:
/// - NEVER signs out user for unverified email (ABSOLUTE RULE)
/// - Source of truth is FirebaseAuth.currentUser.emailVerified
/// - Syncs state after app resume, deep link, manual refresh
library;

import 'package:firebase_auth/firebase_auth.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'email_verification_state.dart';

/// Email verification controller
class EmailVerificationController extends Notifier<EmailVerificationState> {
  EmailVerificationController({FirebaseAuth? firebaseAuth})
    : _firebaseAuth = firebaseAuth ?? FirebaseAuth.instance;

  final FirebaseAuth _firebaseAuth;

  @override
  EmailVerificationState build() {
    // Initialize with current Firebase Auth user's verification status
    _initializeFromFirebase();
    return const EmailVerificationState.initial();
  }

  /// Initialize state from Firebase Auth user
  void _initializeFromFirebase() {
    final user = _firebaseAuth.currentUser;
    if (user != null) {
      if (user.emailVerified == true) {
        state = const EmailVerificationState.verified();
      } else {
        state = const EmailVerificationState.unverified();
      }
    } else {
      state = const EmailVerificationState.initial();
    }
  }

  /// Refresh email verification status from Firebase
  ///
  /// This should be called:
  /// - After app resume (user may have verified in email client)
  /// - After email verification deep link is handled
  /// - When user manually refreshes
  Future<void> refreshEmailVerificationStatus() async {
    if (state is EmailVerificationChecking) {
      return;
    }

    final user = _firebaseAuth.currentUser;
    if (user == null) {
      state = const EmailVerificationState.initial();
      return;
    }

    state = const EmailVerificationState.checking();

    try {
      // Reload user to get latest emailVerified status from Firebase
      await user.reload();

      // Get updated user
      final updatedUser = _firebaseAuth.currentUser;
      if (updatedUser != null && updatedUser.emailVerified) {
        final synced = await ref
            .read(authControllerProvider.notifier)
            .refreshVerifiedEmailAccount();
        if (synced) {
          state = const EmailVerificationState.verified();
          return;
        }

        final currentUser = _firebaseAuth.currentUser;
        if (currentUser == null) {
          state = const EmailVerificationState.initial();
          return;
        }

        if (currentUser.emailVerified) {
          state = EmailVerificationState.error(
            'Unable to sync verified email right now. Please try again.',
          );
          return;
        }

        state = const EmailVerificationState.unverified();
      } else {
        state = const EmailVerificationState.unverified();
      }
    } catch (e) {
      // IMPORTANT: Do NOT sign out user on error (ABSOLUTE RULE)
      // Keep user authenticated, just show error state
      state = EmailVerificationState.error(
        'Unable to check verification status. Please try again.',
      );
    }
  }

  /// Send verification email to current user
  ///
  /// Returns true if email was sent successfully, false otherwise.
  Future<bool> sendVerificationEmail() async {
    final user = _firebaseAuth.currentUser;
    if (user == null) {
      state = const EmailVerificationState.error('No authenticated user found');
      return false;
    }

    if (user.emailVerified) {
      // Already verified, no need to send
      state = const EmailVerificationState.verified();
      return true;
    }

    state = const EmailVerificationState.checking();

    try {
      await user.sendEmailVerification();
      state = const EmailVerificationState.unverified();
      return true;
    } on FirebaseException catch (e) {
      // IMPORTANT: Do NOT sign out user on error (ABSOLUTE RULE)
      state = EmailVerificationState.error(_getErrorMessage(e.code));
      return false;
    } catch (e) {
      state = const EmailVerificationState.error(
        'Unable to send verification email. Please try again.',
      );
      return false;
    }
  }

  /// Get user-friendly error message from Firebase error code
  String _getErrorMessage(String code) {
    switch (code) {
      case 'too-many-requests':
        return 'Too many attempts. Please wait a few minutes before trying again.';
      case 'invalid-email':
        return 'Invalid email address.';
      case 'user-not-found':
        return 'User not found. Please sign in again.';
      default:
        return 'Unable to send verification email. Please try again.';
    }
  }

  /// Check if current user's email is verified
  ///
  /// This is a convenience method that checks Firebase Auth directly.
  bool get isEmailVerified {
    return _firebaseAuth.currentUser?.emailVerified ?? false;
  }

  /// Get current user's email
  String? get currentEmail {
    return _firebaseAuth.currentUser?.email;
  }
}

/// Provider for email verification controller
final emailVerificationControllerProvider =
    NotifierProvider<EmailVerificationController, EmailVerificationState>(
      EmailVerificationController.new,
    );

/// Stream provider that emits email verification status changes
///
/// This can be used to listen for real-time changes when user
/// clicks verification link in email client.
final emailVerificationStatusProvider = StreamProvider<bool>((ref) {
  final auth = FirebaseAuth.instance;

  // Listen to auth state changes and emit verification status
  return auth.authStateChanges().asyncMap((_) async {
    final user = auth.currentUser;
    if (user == null) return false;

    // Reload to get latest status
    try {
      await user.reload();
      final updatedUser = auth.currentUser;
      return updatedUser?.emailVerified ?? false;
    } catch (e) {
      // If error, return current status
      return user.emailVerified;
    }
  });
});
