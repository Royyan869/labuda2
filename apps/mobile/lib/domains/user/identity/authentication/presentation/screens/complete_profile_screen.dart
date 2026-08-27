library;

import 'dart:async';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/data/username_service.dart';
import 'package:labuda/shared/shared.dart';

/// Username availability status for UI
enum UsernameStatus {
  idle,
  checking,
  available,
  taken,
  invalid,

  /// Username just became unavailable (race condition during submit)
  justBecameUnavailable,
}

/// Complete Profile Screen
///
/// Shown after Google sign-in for new users.
/// User must complete their profile (username, etc.) before proceeding.
///
/// Features:
/// - Username input field with edit capability
/// - "Complete Profile" button
/// - Sign out option
/// - 409 Conflict handling for race conditions
class CompleteProfileScreen extends ConsumerStatefulWidget {
  const CompleteProfileScreen({super.key});

  @override
  ConsumerState<CompleteProfileScreen> createState() =>
      _CompleteProfileScreenState();
}

class _CompleteProfileScreenState extends ConsumerState<CompleteProfileScreen> {
  late TextEditingController _usernameController;
  UsernameStatus _usernameStatus = UsernameStatus.invalid;
  Timer? _debounceTimer;
  bool _isSubmitting = false;

  @override
  void initState() {
    super.initState();
    _usernameController = TextEditingController();
  }

  @override
  void dispose() {
    _debounceTimer?.cancel();
    _usernameController.dispose();
    super.dispose();
  }

  /// Handle username input changes with debounce
  void _onUsernameChanged(String value) {
    // Reset race condition status when user types
    if (_usernameStatus == UsernameStatus.justBecameUnavailable) {
      setState(() => _usernameStatus = UsernameStatus.invalid);
    }

    // Cancel previous timer
    _debounceTimer?.cancel();

    // Check length >= 3 and regex validation (lowercase letters, numbers, underscore only)
    if (value.length < 3) {
      setState(() => _usernameStatus = UsernameStatus.invalid);
      return;
    }

    if (!RegExp(r'^[a-z0-9_]+$').hasMatch(value)) {
      setState(() => _usernameStatus = UsernameStatus.invalid);
      return;
    }

    // Set checking state
    setState(() => _usernameStatus = UsernameStatus.checking);

    // Start 500ms debounce timer
    _debounceTimer = Timer(const Duration(milliseconds: 500), () {
      final service = ref.read(usernameServiceProvider);
      service.checkUsernameAvailability(
        username: value.toLowerCase(),
        onResult: (result) {
          if (!mounted) return;
          setState(() {
            if (result.status == UsernameCheckStatus.available) {
              _usernameStatus = UsernameStatus.available;
            } else if (result.status == UsernameCheckStatus.unavailable) {
              _usernameStatus = UsernameStatus.taken;
            } else {
              _usernameStatus = UsernameStatus.invalid;
            }
          });
        },
      );
    });
  }

  /// Submit profile completion with 409 Conflict handling
  ///
  /// 409 Detection:
  /// - Catches DioException at repository level
  /// - Checks statusCode == 409 or ConflictException type
  /// - Sets UsernameStatus.justBecameUnavailable
  /// - Shows specific error message
  /// - Keeps user on screen (no navigation)
  Future<void> _submitProfile() async {
    if (_usernameStatus != UsernameStatus.available || _isSubmitting) return;

    setState(() => _isSubmitting = true);

    try {
      final username = _usernameController.text.trim().toLowerCase();
      final authState = ref.read(authControllerProvider);

      if (authState is! AuthStateRequiresProfileCompletion) {
        if (mounted) setState(() => _isSubmitting = false);
        AppSnackBar.showError(context, 'Invalid authentication state');
        return;
      }

      final result = await ref
          .read(authControllerProvider.notifier)
          .completeProfile(username: username);

      if (mounted) setState(() => _isSubmitting = false);

      if (!result) {
        if (mounted) {
          AppSnackBar.showError(context, 'Failed to complete profile');
        }
      }
    } on DioException catch (e) {
      if (mounted) setState(() => _isSubmitting = false);

      // 🔍 409 CONFLICT DETECTION
      final statusCode = e.response?.statusCode;
      final error = e.error;

      if (statusCode == 409 || error is ConflictException) {
        // Username was taken between availability check and submit
        if (mounted) {
          setState(
            () => _usernameStatus = UsernameStatus.justBecameUnavailable,
          );
          AppSnackBar.showError(context, 'Username just became unavailable');
        }
        return;
      }

      // Other errors - show generic message
      if (mounted) {
        AppSnackBar.showError(
          context,
          'Failed to complete profile. Please try again.',
        );
      }
    } catch (e) {
      if (mounted) {
        setState(() => _isSubmitting = false);
        AppSnackBar.showError(context, 'An unexpected error occurred');
      }
    }
  }

  /// Build username status message widget
  Widget _buildUsernameStatus(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    switch (_usernameStatus) {
      case UsernameStatus.idle:
        return const SizedBox.shrink();
      case UsernameStatus.checking:
        return Row(
          children: [
            SizedBox(
              width: 16,
              height: 16,
              child: CircularProgressIndicator(strokeWidth: 2),
            ),
            const SizedBox(width: 8),
            Text(
              'Checking...',
              style: TextStyle(
                fontSize: 12,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
              ),
            ),
          ],
        );
      case UsernameStatus.available:
        return const Text(
          'Username available',
          style: TextStyle(color: Colors.green, fontSize: 12),
        );
      case UsernameStatus.taken:
        return const Text(
          'Username already taken',
          style: TextStyle(color: Colors.red, fontSize: 12),
        );
      case UsernameStatus.justBecameUnavailable:
        return const Text(
          'Username just became unavailable',
          style: TextStyle(color: Colors.orange, fontSize: 12),
        );
      case UsernameStatus.invalid:
        return const Text(
          'Only lowercase letters, numbers, underscore',
          style: TextStyle(color: Colors.red, fontSize: 12),
        );
    }
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final authState = ref.watch(authControllerProvider);
    String? email;

    // Extract data from AuthStateRequiresProfileCompletion
    if (authState is AuthStateRequiresProfileCompletion) {
      email = authState.email;
    }

    return Scaffold(
      body: Container(
        decoration: BoxDecoration(
          gradient: isDark
              ? LinearGradient(
                  begin: Alignment.topCenter,
                  end: Alignment.bottomCenter,
                  colors: [AppColors.darkGray900, AppColors.darkGray800],
                )
              : const LinearGradient(
                  begin: Alignment.topCenter,
                  end: Alignment.bottomCenter,
                  colors: [AppColors.neutralGray50, AppColors.neutralWhite],
                ),
        ),
        child: SafeArea(
          child: Center(
            child: SingleChildScrollView(
              padding: const EdgeInsets.all(32),
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  // Profile Icon
                  Container(
                    padding: const EdgeInsets.all(24),
                    decoration: BoxDecoration(
                      color: AppColors.primaryRed.withValues(alpha: 0.1),
                      shape: BoxShape.circle,
                    ),
                    child: Icon(
                      Icons.person_add_outlined,
                      size: 64,
                      color: AppColors.primaryRed,
                    ),
                  ),
                  const SizedBox(height: 32),

                  // Title
                  Text(
                    'Complete Your Profile',
                    style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                      fontWeight: FontWeight.bold,
                      color: isDark
                          ? AppColors.neutralWhite
                          : AppColors.darkGray800,
                    ),
                    textAlign: TextAlign.center,
                  ),
                  const SizedBox(height: 16),

                  // Subtitle
                  Text(
                    'Please choose a username to continue',
                    style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                      color: isDark
                          ? AppColors.neutralGray300
                          : AppColors.neutralGray600,
                    ),
                    textAlign: TextAlign.center,
                  ),
                  const SizedBox(height: 40),

                  // Username Input Field
                  TextField(
                    controller: _usernameController,
                    onChanged: _onUsernameChanged,
                    decoration: InputDecoration(
                      labelText: 'Username',
                      hintText: 'Enter username',
                      prefixIcon: const Icon(Icons.person_outline),
                      border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(12),
                      ),
                    ),
                  ),
                  const SizedBox(height: 8),

                  // Username status message
                  _buildUsernameStatus(context),
                  const SizedBox(height: 16),

                  // Info message about email
                  if (email != null && email.isNotEmpty)
                    Container(
                      padding: const EdgeInsets.all(16),
                      decoration: BoxDecoration(
                        color: isDark
                            ? AppColors.darkGray700.withValues(alpha: 0.5)
                            : AppColors.neutralGray50,
                        borderRadius: BorderRadius.circular(12),
                        border: Border.all(
                          color: isDark
                              ? AppColors.darkGray600
                              : AppColors.neutralGray200,
                        ),
                      ),
                      child: Row(
                        children: [
                          Icon(
                            Icons.email_outlined,
                            size: 20,
                            color: isDark
                                ? AppColors.neutralGray400
                                : AppColors.neutralGray600,
                          ),
                          const SizedBox(width: 12),
                          Expanded(
                            child: Text(
                              'Signed in with: $email',
                              style: TextStyle(
                                fontSize: 13,
                                color: isDark
                                    ? AppColors.neutralGray300
                                    : AppColors.neutralGray700,
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                  const SizedBox(height: 40),

                  // Complete Profile Button
                  SizedBox(
                    width: double.infinity,
                    child: ElevatedButton(
                      onPressed:
                          (_usernameStatus == UsernameStatus.available &&
                              !_isSubmitting)
                          ? _submitProfile
                          : null,
                      style: ElevatedButton.styleFrom(
                        backgroundColor: AppColors.primaryRed,
                        foregroundColor: AppColors.light,
                        disabledBackgroundColor: isDark
                            ? AppColors.darkGray600
                            : AppColors.neutralGray300,
                        padding: const EdgeInsets.symmetric(vertical: 14),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(12),
                        ),
                      ),
                      child: _isSubmitting
                          ? const SizedBox(
                              width: 20,
                              height: 20,
                              child: CircularProgressIndicator(
                                strokeWidth: 2,
                                color: AppColors.light,
                              ),
                            )
                          : const Text('Complete Profile'),
                    ),
                  ),
                  const SizedBox(height: 16),

                  // Sign Out Button
                  SizedBox(
                    width: double.infinity,
                    child: TextButton(
                      onPressed: _isSubmitting
                          ? null
                          : () {
                              ref
                                  .read(authControllerProvider.notifier)
                                  .signOut();
                            },
                      style: TextButton.styleFrom(
                        padding: const EdgeInsets.symmetric(vertical: 14),
                      ),
                      child: Text(
                        'Sign Out',
                        style: TextStyle(
                          fontSize: 16,
                          color: isDark
                              ? AppColors.neutralGray400
                              : AppColors.neutralGray500,
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
