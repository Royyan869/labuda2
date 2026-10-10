import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/shared.dart';
import 'package:hishumi/shared/helpers/canonical_password_policy.dart';
import 'package:hishumi/shared/helpers/canonical_password_match.dart';
import 'package:hishumi/domains/user/identity/authentication/presentation/shared/widgets/auth_password_field.dart';
import 'package:hishumi/domains/user/identity/authentication/presentation/shared/widgets/auth_button.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/domains/user/profile/presentation/shared/shared.dart';

/// Security Management Screen (Refactored)
///
/// Golden Sample #2 for Settings module refactor.
///
/// ## Architecture Principles:
/// - Screen-level UI state managed by SecurityFormController
/// - Toggle settings stay in screen (not in controller)
/// - Password fields use shared AuthPasswordField
/// - State rendering via ProfileStateView
///
/// ## State Classification:
/// - **Controller (UI state)**: `isLoading`, `errorMessage`, `successMessage`
/// - **Screen (visibility state)**: `_isCurrentPasswordVisible`, `_isNewPasswordVisible`, `_isConfirmPasswordVisible`
///
/// @see REFACTOR_UI.md section 13 for security form guidelines
class SecurityScreen extends ConsumerStatefulWidget {
  const SecurityScreen({super.key});

  @override
  ConsumerState<SecurityScreen> createState() => _SecurityScreenState();
}

class _SecurityScreenState extends ConsumerState<SecurityScreen> {
  final _formKey = GlobalKey<FormState>();
  late final SecurityFormController _controller;

  // Password change controllers
  final _currentPasswordController = TextEditingController();
  final _newPasswordController = TextEditingController();
  final _confirmPasswordController = TextEditingController();

  // Password visibility (NOT in controller - managed per field)
  bool _isCurrentPasswordVisible = false;
  bool _isNewPasswordVisible = false;
  bool _isConfirmPasswordVisible = false;

  @override
  void initState() {
    super.initState();
    _controller = SecurityFormController();
  }

  @override
  void dispose() {
    _currentPasswordController.dispose();
    _newPasswordController.dispose();
    _confirmPasswordController.dispose();
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authControllerProvider);
    // AUTH-H2 G2: Change Password availability is decided by the SINGLE
    // credential authority (hasPasswordCredentialProvider). The password
    // section renders from this one value — no parallel provider checks.
    final hasPasswordCredential = ref.watch(hasPasswordCredentialProvider);

    return Scaffold(
      appBar: AppBarCustom(title: AppLocalizations.of(context)!.securityTitle),
      body: authState is AuthStateAuthenticated
          // AUTH-H2 G3: the screen must LISTEN to its local form controller —
          // without this, a changePassword failure (which deliberately no
          // longer mutates the global auth state) had nothing to trigger a
          // rebuild and the error stayed invisible.
          ? ListenableBuilder(
              listenable: _controller,
              builder: (context, _) => ProfileStateView(
                isLoading: _controller.isLoading,
                error: _controller.errorMessage,
                success: _controller.successMessage,
                onErrorDismiss: _controller.clearError,
                onSuccessDismiss: _controller.clearSuccess,
                content: _buildForm(context, hasPasswordCredential),
              ),
            )
          : Center(
              child: Text(AppLocalizations.of(context)!.pleaseLoginToManage),
            ),
    );
  }

  Widget _buildForm(BuildContext context, bool hasPasswordCredential) {
    return SafeArea(
      child: Form(
        key: _formKey,
        child: ListView(
          padding: const EdgeInsets.all(AppMetrics.p24),
          children: [
            // Password Section — gated on ACTUAL password-credential
            // availability (AUTH-H2). A Google-only account must never see
            // an executable Change Password form.
            _buildSectionHeader(
              AppLocalizations.of(context)!.passwordManagement,
            ),
            const SizedBox(height: 16),
            hasPasswordCredential
                ? _buildPasswordSection(context)
                : _buildPasswordManagedByGoogleNotice(context),
            const SizedBox(height: 32),

            // Security Settings Section
            _buildSectionHeader(AppLocalizations.of(context)!.advancedSecurity),
            const SizedBox(height: 16),
            _buildSecuritySettingsSection(context),
            const SizedBox(height: 32),

            // Account Management Section
            _buildSectionHeader(
              AppLocalizations.of(context)!.accountManagement,
            ),
            const SizedBox(height: 16),
            _buildAccountManagementSection(context),
          ],
        ),
      ),
    );
  }

  /// AUTH-H2 G2 (rule 3): accounts WITHOUT a HiShumi password credential
  /// (Google-only, or an identity whose credentials cannot be proven) get a
  /// clear explanation instead of the form. No account-linking entry point
  /// is offered here — adding a password is a separate linking concern.
  Widget _buildPasswordManagedByGoogleNotice(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final scheme = Theme.of(context).colorScheme;

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p24),
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r16),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.g_mobiledata,
                color: scheme.onSurface,
                size: AppIconSize.emphasis,
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  l10n.passwordManagedByGoogleTitle,
                  style: context.typeRoles.titleCompact.copyWith(
                    color: scheme.onSurface,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          Text(
            l10n.passwordManagedByGoogleBody,
            style: context.typeRoles.bodyDense.copyWith(
              color: scheme.onSurfaceVariant,
              height: 1.5,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildPasswordSection(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final scheme = Theme.of(context).colorScheme;

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p24),
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r16),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            l10n.changePassword,
            style: context.typeRoles.titleCompact.copyWith(
              color: scheme.onSurface,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 16),

          // Current Password Field - using shared AuthPasswordField
          AuthPasswordField(
            controller: _currentPasswordController,
            labelText: l10n.currentPassword,
            hintText: l10n.enterCurrentPassword,
            isPasswordVisible: _isCurrentPasswordVisible,
            onToggleVisibility: () {
              setState(
                () => _isCurrentPasswordVisible = !_isCurrentPasswordVisible,
              );
            },
            validator: (value) {
              if (value == null || value.trim().isEmpty) {
                return l10n.currentPasswordRequired;
              }
              return null;
            },
          ),
          const SizedBox(height: 16),

          // New Password Field - using shared AuthPasswordField with strength indicator
          AuthPasswordField(
            controller: _newPasswordController,
            labelText: l10n.newPassword,
            hintText: l10n.enterNewPassword,
            isPasswordVisible: _isNewPasswordVisible,
            onToggleVisibility: () {
              setState(() => _isNewPasswordVisible = !_isNewPasswordVisible);
            },
            onChanged: (value) {
              // Trigger rebuild to update password strength indicator
              setState(() {});
            },
            strengthIndicator: _newPasswordController.text.isNotEmpty
                ? PasswordStrengthIndicator(
                    password: _newPasswordController.text,
                  )
                : null,
            validator: (value) {
              if (value == null || value.trim().isEmpty) {
                return l10n.newPasswordRequired;
              }
              // Canonical Labuda password policy (min 8 + upper + lower + digit).
              return CanonicalPasswordPolicy.validationMessage(value);
            },
          ),
          const SizedBox(height: 16),

          // Confirm Password Field - using shared AuthConfirmPasswordField
          AuthConfirmPasswordField(
            controller: _confirmPasswordController,
            passwordController: _newPasswordController,
            labelText: l10n.confirmNewPassword,
            hintText: l10n.confirmNewPasswordPlaceholder,
            isVisible: _isConfirmPasswordVisible,
            onToggleVisibility: () {
              setState(
                () => _isConfirmPasswordVisible = !_isConfirmPasswordVisible,
              );
            },
            validator: (value) {
              if (value == null || value.trim().isEmpty) {
                return l10n.confirmPasswordRequired;
              }
              // Canonical confirm-password rule — agrees with the shared match
              // indicator and the sign-up gate.
              if (!CanonicalPasswordMatch.matches(
                value,
                _newPasswordController.text,
              )) {
                return l10n.newPasswordsDoNotMatch;
              }
              return null;
            },
          ),
          const SizedBox(height: 20),

          // Update Password Button - using shared AuthButton
          AuthButton.primary(
            text: l10n.updatePassword,
            onPressed: _controller.isLoading ? null : _changePassword,
            isLoading: _controller.isLoading,
          ),
          const SizedBox(height: 12),

          // Security Tip
          Container(
            padding: const EdgeInsets.all(AppMetrics.p12),
            decoration: BoxDecoration(
              color: context.statusColors.warning.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(AppShape.r8),
              border: Border.all(
                color: context.statusColors.warning.withValues(alpha: 0.3),
              ),
            ),
            child: Row(
              children: [
                Icon(
                  Icons.security,
                  color: context.statusColors.warning,
                  size: AppIconSize.inlineGlyph,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    l10n.strongPasswordMessage,
                    style: context.typeRoles.labelMicro.copyWith(
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSecuritySettingsSection(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final scheme = Theme.of(context).colorScheme;

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p24),
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r16),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Column(
        children: [
          // Login Sessions
          ListTile(
            leading: Icon(
              Icons.devices_outlined,
              color: scheme.onSurfaceVariant,
            ),
            title: Text(l10n.loginSessions),
            subtitle: Text(l10n.manageActiveSessions),
            trailing: const Icon(Icons.chevron_right),
            onTap: () => context.push(RoutePaths.loginSessions),
          ),
        ],
      ),
    );
  }

  Widget _buildAccountManagementSection(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final scheme = Theme.of(context).colorScheme;

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p24),
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r16),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Column(
        children: [
          // Deactivate Account
          ListTile(
            leading: Icon(
              Icons.person_off_outlined,
              color: context.statusColors.warning,
            ),
            title: Text(l10n.deactivateAccount),
            trailing: const Icon(Icons.chevron_right),
            onTap: () => _showDeactivateAccountDialog(context),
          ),

          const Divider(height: 1),

          // Delete Account
          ListTile(
            leading: Icon(
              Icons.delete_forever_outlined,
              color: context.statusColors.error,
            ),
            title: Text(l10n.deleteAccount),
            trailing: const Icon(Icons.chevron_right),
            onTap: () => _showDeleteAccountDialog(context),
          ),
        ],
      ),
    );
  }

  Widget _buildSectionHeader(String title) {
    return Text(
      title,
      style: context.typeRoles.titleSection.copyWith(
        fontWeight: FontWeight.bold,
        color: Theme.of(context).colorScheme.onSurface,
      ),
    );
  }

  Future<void> _changePassword() async {
    final l10n = AppLocalizations.of(context)!;

    // Validate form using Flutter Form validation
    if (!_formKey.currentState!.validate()) {
      return;
    }

    _controller.setLoading(true);

    try {
      // AUTH-H2 G3: the controller returns the failure message LOCALLY —
      // a wrong current password never mutates the global auth state, so
      // the session stays alive and the user stays on this screen.
      final error = await ref
          .read(authControllerProvider.notifier)
          .changePassword(
            currentPassword: _currentPasswordController.text.trim(),
            newPassword: _newPasswordController.text.trim(),
          );

      if (mounted) {
        if (error == null) {
          // Clear form
          _currentPasswordController.clear();
          _newPasswordController.clear();
          _confirmPasswordController.clear();
          setState(() {
            _isCurrentPasswordVisible = false;
            _isNewPasswordVisible = false;
            _isConfirmPasswordVisible = false;
          });

          _controller.showSuccess(l10n.passwordUpdatedSuccessfully);

          // Clear success message after delay
          Future.delayed(const Duration(seconds: 3), () {
            _controller.clearSuccess();
          });
        } else {
          // Failure is rendered INLINE on this screen — never as success,
          // never as a global auth error.
          _controller.showError(error);
        }
      }
    } catch (e) {
      if (mounted) {
        _controller.showError(l10n.anErrorOccurred);
      }
    } finally {
      if (mounted) {
        _controller.setLoading(false);
      }
    }
  }

  void _showDeactivateAccountDialog(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final notesController = TextEditingController();
    String selectedReason = '';

    showDialog(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setState) => AlertDialog(
          title: Text(l10n.deactivateAccountTitle),
          content: SizedBox(
            width: double.maxFinite,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  l10n.deactivateAccountDescription,
                  style: context.typeRoles.bodyDense,
                ),
                const SizedBox(height: 16),
                Text(
                  l10n.reasonForDeactivation,
                  style: const TextStyle(fontWeight: FontWeight.w500),
                ),
                const SizedBox(height: 8),
                Container(
                  decoration: BoxDecoration(
                    border: Border.all(
                      color: Theme.of(context).colorScheme.outlineVariant,
                    ),
                    borderRadius: BorderRadius.circular(AppShape.r8),
                  ),
                  child: DropdownButtonHideUnderline(
                    child: DropdownButton<String>(
                      value: selectedReason.isEmpty ? null : selectedReason,
                      hint: Padding(
                        padding: const EdgeInsets.symmetric(
                          horizontal: AppMetrics.p12,
                        ),
                        child: Text(l10n.selectReason),
                      ),
                      isExpanded: true,
                      items: [
                        DropdownMenuItem(
                          value: 'privacy_concern',
                          child: Text(l10n.privacyConcerns),
                        ),
                        DropdownMenuItem(
                          value: 'not_using',
                          child: Text(l10n.notUsingApp),
                        ),
                        DropdownMenuItem(
                          value: 'technical_issues',
                          child: Text(l10n.technicalIssues),
                        ),
                        DropdownMenuItem(
                          value: 'security_concern',
                          child: Text(l10n.securityConcerns),
                        ),
                        DropdownMenuItem(
                          value: 'temporary_break',
                          child: Text(l10n.takingBreak),
                        ),
                        DropdownMenuItem(
                          value: 'other',
                          child: Text(l10n.other),
                        ),
                      ],
                      onChanged: (value) =>
                          setState(() => selectedReason = value ?? ''),
                    ),
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: notesController,
                  // Border/fill come from `inputDecorationTheme` (AppTheme) —
                  // the one form-field authority.
                  decoration: InputDecoration(
                    labelText: l10n.additionalNotesOptional,
                    hintText: l10n.pleaseTellUsMore,
                  ),
                  maxLines: 3,
                  maxLength: 200,
                ),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(context).pop(),
              child: Text(l10n.cancel),
            ),
            TextButton(
              onPressed: selectedReason.isEmpty
                  ? null
                  : () async {
                      Navigator.of(context).pop();
                      await _performAccountDeactivation(
                        context,
                        selectedReason,
                        notesController.text,
                      );
                    },
              child: Text(
                l10n.deactivate,
                style: TextStyle(
                  color: selectedReason.isEmpty
                      ? Theme.of(context).colorScheme.onSurfaceVariant
                      : context.statusColors.warning,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _performAccountDeactivation(
    BuildContext context,
    String reason,
    String notes,
  ) async {
    final l10n = AppLocalizations.of(context)!;
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) {
      ref.read(navigationHandlerProvider).navigateToSignIn();
      return;
    }

    showDialog(
      context: context,
      barrierDismissible: false,
      builder: (context) => const Center(child: CircularProgressIndicator()),
    );

    try {
      final authController = ref.read(authControllerProvider.notifier);
      final combinedReason = notes.isNotEmpty ? '$reason - $notes' : reason;

      final success = await authController.deactivateAccount(
        userId: authState.user.id,
        reason: combinedReason,
      );

      if (mounted && context.mounted) {
        Navigator.of(context).pop();
        if (success) {
          if (!context.mounted) return;
          AppSnackBar.showSuccess(context, l10n.accountDeactivatedSuccessfully);
        } else {
          final currentState = ref.read(authControllerProvider);
          if (currentState is AuthStateError) {
            if (!context.mounted) return;
            AppSnackBar.showError(context, currentState.message);
          } else {
            if (!context.mounted) return;
            AppSnackBar.showError(context, l10n.failedToDeactivateAccount);
          }
        }
      }
    } catch (e) {
      debugPrint('account.deactivate failed: $e');
      if (mounted && context.mounted) {
        Navigator.of(context).pop();
        if (!context.mounted) return;
        AppSnackBar.showError(context, l10n.failedToDeactivateAccount);
      }
    }
  }

  void _showDeleteAccountDialog(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(l10n.deleteAccount),
        content: Text(l10n.deleteAccountConfirm),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: Text(l10n.cancel),
          ),
          TextButton(
            onPressed: () {
              Navigator.of(context).pop();
              _performAccountDeletion(context);
            },
            child: Text(
              l10n.delete,
              style: TextStyle(color: context.statusColors.error),
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _performAccountDeletion(BuildContext context) async {
    showDialog(
      context: context,
      barrierDismissible: false,
      builder: (context) => const Center(child: CircularProgressIndicator()),
    );

    final error = await ref
        .read(authControllerProvider.notifier)
        .deleteAccount();

    if (!context.mounted) return;
    Navigator.of(context).pop(); // dismiss progress

    if (error == null) return; // success — signOut already called

    // Re-authentication required: the canonical flow is the sign-in route, not
    // a transient toast. Any other failure is shown with safe, non-technical
    // copy (never the raw backend error).
    if (error == 'requires-recent-login') {
      ref.read(navigationHandlerProvider).navigateToSignIn();
      return;
    }
    AppSnackBar.showError(context, 'Gagal menghapus akun. Coba lagi.');
  }
}
