import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/helpers/canonical_password_policy.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/screens/login_sessions_screen.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/shared/widgets/auth_password_field.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/shared/widgets/auth_button.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/domains/user/profile/presentation/shared/shared.dart';

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
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final authState = ref.watch(authControllerProvider);

    return Scaffold(
      appBar: AppBarCustom(title: AppLocalizations.of(context)!.securityTitle),
      body: authState is AuthStateAuthenticated
          ? ProfileStateView(
              isLoading: _controller.isLoading,
              error: _controller.errorMessage,
              success: _controller.successMessage,
              onErrorDismiss: _controller.clearError,
              onSuccessDismiss: _controller.clearSuccess,
              content: _buildForm(context, isDark, authState.user),
            )
          : Center(
              child: Text(AppLocalizations.of(context)!.pleaseLoginToManage),
            ),
    );
  }

  Widget _buildForm(BuildContext context, bool isDark, AuthUser user) {
    return SafeArea(
      child: Form(
        key: _formKey,
        child: ListView(
          padding: const EdgeInsets.all(24),
          children: [
            // Password Section
            _buildSectionHeader(
              AppLocalizations.of(context)!.passwordManagement,
            ),
            const SizedBox(height: 16),
            _buildPasswordSection(context, isDark),
            const SizedBox(height: 32),

            // Security Settings Section
            _buildSectionHeader(AppLocalizations.of(context)!.advancedSecurity),
            const SizedBox(height: 16),
            _buildSecuritySettingsSection(context, isDark),
            const SizedBox(height: 32),

            // Account Management Section
            _buildSectionHeader(
              AppLocalizations.of(context)!.accountManagement,
            ),
            const SizedBox(height: 16),
            _buildAccountManagementSection(context, isDark),
          ],
        ),
      ),
    );
  }

  Widget _buildPasswordSection(BuildContext context, bool isDark) {
    final l10n = AppLocalizations.of(context)!;

    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            l10n.changePassword,
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray200
                  : AppColors.neutralGray900,
              fontSize: 16,
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
                    isDark: isDark,
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
              // Stage 4D: trimmed comparison — agrees with the shared match
              // indicator and the sign-up gate.
              if (value.trim() != _newPasswordController.text.trim()) {
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
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: AppColors.statusWarning.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(8),
              border: Border.all(
                color: AppColors.statusWarning.withValues(alpha: 0.3),
              ),
            ),
            child: Row(
              children: [
                Icon(Icons.security, color: AppColors.statusWarning, size: 16),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    l10n.strongPasswordMessage,
                    style: const TextStyle(
                      color: AppColors.neutralGray700,
                      fontSize: 12,
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

  Widget _buildSecuritySettingsSection(BuildContext context, bool isDark) {
    final l10n = AppLocalizations.of(context)!;

    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
        ),
      ),
      child: Column(
        children: [
          // Login Sessions
          ListTile(
            leading: Icon(
              Icons.devices_outlined,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
            title: Text(l10n.loginSessions),
            subtitle: Text(l10n.manageActiveSessions),
            trailing: const Icon(Icons.chevron_right),
            onTap: () {
              Navigator.of(context).push(
                MaterialPageRoute(builder: (_) => const LoginSessionsScreen()),
              );
            },
          ),
        ],
      ),
    );
  }

  Widget _buildAccountManagementSection(BuildContext context, bool isDark) {
    final l10n = AppLocalizations.of(context)!;

    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
        ),
      ),
      child: Column(
        children: [
          // Deactivate Account
          ListTile(
            leading: const Icon(
              Icons.person_off_outlined,
              color: AppColors.warning,
            ),
            title: Text(l10n.deactivateAccount),
            trailing: const Icon(Icons.chevron_right),
            onTap: () => _showDeactivateAccountDialog(context),
          ),

          const Divider(height: 1),

          // Delete Account
          ListTile(
            leading: const Icon(
              Icons.delete_forever_outlined,
              color: AppColors.error,
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
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Text(
      title,
      style: TextStyle(
        fontSize: 18,
        fontWeight: FontWeight.bold,
        color: isDark ? AppColors.neutralGray200 : AppColors.neutralGray900,
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
      final success = await ref
          .read(authControllerProvider.notifier)
          .changePassword(
            currentPassword: _currentPasswordController.text.trim(),
            newPassword: _newPasswordController.text.trim(),
          );

      if (mounted) {
        if (success) {
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
          final authState = ref.read(authControllerProvider);
          if (authState is AuthStateError) {
            _controller.showError(authState.message);
          } else {
            _controller.showError(l10n.failedToChangePassword);
          }
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
                  style: const TextStyle(fontSize: 14),
                ),
                const SizedBox(height: 16),
                Text(
                  l10n.reasonForDeactivation,
                  style: const TextStyle(fontWeight: FontWeight.w500),
                ),
                const SizedBox(height: 8),
                Container(
                  decoration: BoxDecoration(
                    border: Border.all(color: AppColors.neutralGray300),
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: DropdownButtonHideUnderline(
                    child: DropdownButton<String>(
                      value: selectedReason.isEmpty ? null : selectedReason,
                      hint: Padding(
                        padding: const EdgeInsets.symmetric(horizontal: 12),
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
                  decoration: InputDecoration(
                    labelText: l10n.additionalNotesOptional,
                    border: const OutlineInputBorder(),
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
                      ? AppColors.neutralGray400
                      : AppColors.warning,
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
      AppSnackBar.showError(context, l10n.userNotAuthenticated);
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
      if (mounted && context.mounted) {
        Navigator.of(context).pop();
        if (!context.mounted) return;
        AppSnackBar.showError(context, '${l10n.failedToDeactivateAccount}: $e');
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
              style: const TextStyle(color: AppColors.error),
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

    AppSnackBar.showError(
      context,
      error == 'requires-recent-login'
          ? 'Please sign in again to delete your account'
          : error,
    );
  }
}
