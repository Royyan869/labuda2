library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/helpers/canonical_username_validator.dart';

import '../widgets/username_field.dart';

/// Complete Profile Screen
///
/// Shown after Google sign-in (or any provider) for users whose backend
/// profile has no username yet (`requiresProfileCompletion`).
///
/// 🔒 CANONICAL USERNAME AUTHORITY (single contract shared with the sign-up
/// screen — one language, one truth, no second authority):
/// - FORMAT validation is LOCAL via [CanonicalUsernameValidator] (mirrors the
///   backend identityusername rules) and auto-lowercases as the user types.
/// - AVAILABILITY (taken / reserved / final acceptance) is BACKEND authority,
///   decided at the transactional moment (`POST /auth/complete-profile`).
///   There is NO client-side availability pre-check — an advisory endpoint
///   cannot authenticate with the restricted token and would create a second,
///   disagreeable source of truth.
/// - Backend rejections surface INLINE on this screen via
///   [ProfileCompletionOutcome.usernameError] (same message mapping as the
///   registration flow) and never mutate the global auth state — the user
///   stays here to correct the choice.
///
/// Features:
/// - Username input (shared [UsernameField] widget)
/// - "Complete Profile" button gated on local format validity only
/// - Sign out option
class CompleteProfileScreen extends ConsumerStatefulWidget {
  const CompleteProfileScreen({super.key});

  @override
  ConsumerState<CompleteProfileScreen> createState() =>
      _CompleteProfileScreenState();
}

class _CompleteProfileScreenState extends ConsumerState<CompleteProfileScreen> {
  final _usernameController = TextEditingController();

  /// Format validity from the shared [UsernameField] (local, instant).
  bool _isUsernameValid = false;

  /// Backend rejection / transient failure, shown INLINE under the field.
  String? _inlineError;

  bool _isSubmitting = false;

  @override
  void initState() {
    super.initState();
    // Same controller as [UsernameField]; clear the inline rejection as soon
    // as the user starts correcting the username.
    _usernameController.addListener(_onUsernameChanged);
  }

  @override
  void dispose() {
    _usernameController.removeListener(_onUsernameChanged);
    _usernameController.dispose();
    super.dispose();
  }

  void _onUsernameChanged() {
    if (_inlineError != null && mounted) {
      setState(() => _inlineError = null);
    }
  }

  /// Format-only callback from the shared [UsernameField]. Availability is
  /// NEVER claimed here — [isAvailable] is ignored by contract (backend
  /// authority decides at submit time).
  void _onValidationChanged(bool isValid, bool isAvailable) {
    if (_isUsernameValid != isValid) {
      setState(() => _isUsernameValid = isValid);
    }
  }

  /// Submit profile completion. The backend exchange is the SINGLE authority:
  /// a rejected username arrives as [ProfileCompletionOutcome.usernameError]
  /// and is rendered inline; success flows through the router via the
  /// AuthState change (no manual navigation).
  Future<void> _submitProfile() async {
    if (!_isUsernameValid || _isSubmitting) return;

    setState(() {
      _isSubmitting = true;
      _inlineError = null;
    });

    final username =
        CanonicalUsernameValidator.normalize(_usernameController.text) ?? '';

    try {
      final outcome = await ref
          .read(authControllerProvider.notifier)
          .completeProfile(username: username);

      if (!mounted) return;

      if (outcome.success) {
        // AuthStateAuthenticated (or restricted) drives the router redirect.
        // Reset the spinner regardless: if navigation lags (or in tests, when
        // the screen stays mounted), an eternal CircularProgressIndicator
        // must never spin forever on a settled screen.
        if (mounted) setState(() => _isSubmitting = false);
        return;
      }

      setState(() {
        _isSubmitting = false;
        _inlineError = outcome.usernameError ?? outcome.failureError;
      });
    } catch (e) {
      if (mounted) {
        setState(() {
          _isSubmitting = false;
          _inlineError = 'Gagal melengkapi profil. Coba lagi.';
        });
      }
    }
  }

  /// Inline status line under the username field — the single message surface
  /// for both local format feedback and backend rejections.
  Widget _buildUsernameStatus(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    if (_inlineError != null) {
      return Text(
        _inlineError!,
        style: context.typeRoles.labelMicro.copyWith(color: scheme.error),
      );
    }
    if (_isUsernameValid) {
      return Text(
        'Username terlihat baik — ketersediaan diputuskan server saat disimpan.',
        style: context.typeRoles.labelMicro.copyWith(
          color: scheme.onSurfaceVariant,
        ),
      );
    }
    if (_usernameController.text.isNotEmpty) {
      return Text(
        'Gunakan 3-30 karakter: huruf kecil, angka, dan underscore.',
        style: context.typeRoles.labelMicro.copyWith(
          color: context.statusColors.warning,
        ),
      );
    }
    return const SizedBox.shrink();
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final authState = ref.watch(authControllerProvider);
    String? email;

    // Extract data from AuthStateRequiresProfileCompletion
    if (authState is AuthStateRequiresProfileCompletion) {
      email = authState.email;
    }

    return Scaffold(
      body: Container(
        decoration: BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topLeft,
            end: Alignment.bottomRight,
            colors: [scheme.surfaceContainerLow, scheme.surface],
          ),
        ),
        child: SafeArea(
          child: Center(
            child: SingleChildScrollView(
              padding: const EdgeInsets.all(AppMetrics.p32),
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  // Profile Icon
                  Container(
                    padding: const EdgeInsets.all(AppMetrics.p24),
                    decoration: BoxDecoration(
                      color: scheme.primary.withValues(alpha: 0.1),
                      shape: BoxShape.circle,
                    ),
                    child: Icon(
                      Icons.person_add_outlined,
                      size: AppIconSize.display,
                      color: scheme.primary,
                    ),
                  ),
                  const SizedBox(height: 32),

                  // Title
                  Text(
                    'Complete Your Profile',
                    style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                      fontWeight: FontWeight.bold,
                      color: scheme.onSurface,
                    ),
                    textAlign: TextAlign.center,
                  ),
                  const SizedBox(height: 16),

                  // Subtitle
                  Text(
                    'Please choose a username to continue',
                    style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                      color: scheme.onSurfaceVariant,
                    ),
                    textAlign: TextAlign.center,
                  ),
                  const SizedBox(height: 40),

                  // Username Input Field — the SAME canonical widget as the
                  // sign-up screen (auto-lowercase, format-only validation).
                  UsernameField(
                    controller: _usernameController,
                    onValidationChanged: _onValidationChanged,
                  ),
                  const SizedBox(height: 8),

                  // Username status message (format hint / backend rejection)
                  _buildUsernameStatus(context),
                  const SizedBox(height: 16),

                  // Info message about email
                  if (email != null && email.isNotEmpty)
                    Container(
                      padding: const EdgeInsets.all(AppMetrics.p16),
                      decoration: BoxDecoration(
                        color: scheme.surfaceContainerLow,
                        borderRadius: BorderRadius.circular(AppShape.r12),
                        border: Border.all(color: scheme.outlineVariant),
                      ),
                      child: Row(
                        children: [
                          Icon(
                            Icons.email_outlined,
                            size: AppIconSize.action,
                            color: scheme.onSurfaceVariant,
                          ),
                          const SizedBox(width: 12),
                          Expanded(
                            child: Text(
                              'Signed in with: $email',
                              style: context.typeRoles.bodyDense.copyWith(
                                color: scheme.onSurface,
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                  const SizedBox(height: 40),

                  // Complete Profile Button — gated on LOCAL FORMAT validity
                  // only (canonical contract). Backend rejections arrive after
                  // submit and render inline without disabling the flow.
                  SizedBox(
                    width: double.infinity,
                    child: ElevatedButton(
                      onPressed: (_isUsernameValid && !_isSubmitting)
                          ? _submitProfile
                          : null,
                      style: ElevatedButton.styleFrom(
                        padding: const EdgeInsets.symmetric(
                          vertical: AppMetrics.p16,
                        ),
                      ),
                      child: _isSubmitting
                          ? SizedBox(
                              width: 20,
                              height: 20,
                              child: CircularProgressIndicator(
                                strokeWidth: 2,
                                color: scheme.onSurfaceVariant,
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
                        padding: const EdgeInsets.symmetric(
                          vertical: AppMetrics.p16,
                        ),
                      ),
                      child: Text(
                        'Sign Out',
                        style: Theme.of(context).textTheme.labelLarge?.copyWith(
                          color: scheme.onSurfaceVariant,
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
