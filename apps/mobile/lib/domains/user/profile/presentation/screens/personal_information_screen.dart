import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/helpers/canonical_email_validator.dart';
import 'package:labuda/shared/helpers/canonical_phone_validator.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/personal_information_section.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/phone_verification_dialog.dart';
import 'package:labuda/domains/user/profile/presentation/shared/shared.dart';

/// Personal Information Screen (Refactored)
///
/// Demonstrates applying Auth Form Pattern to non-auth screens.
///
/// ## Architecture Principles:
/// - Screen-level UI state managed by ProfileFormController
/// - Async verification state managed at widget level (not controller)
/// - Form validation handled by Flutter Form + widget validators
///
/// @see REFACTOR_UI.md section 11 for non-auth form guidelines
class PersonalInformationScreen extends ConsumerStatefulWidget {
  const PersonalInformationScreen({super.key});

  @override
  ConsumerState<PersonalInformationScreen> createState() =>
      _PersonalInformationScreenState();
}

class _PersonalInformationScreenState
    extends ConsumerState<PersonalInformationScreen> {
  final _formKey = GlobalKey<FormState>();
  late final ProfileFormController _controller;

  final _phoneController = TextEditingController();

  // Data state (not UI state)
  DateTime? _selectedDateOfBirth;

  // Widget-level async verification state (per architecture pattern)
  // These are NOT in controller because they represent async operation results
  bool _isEmailVerified = false;
  bool _isPhoneVerified = false;
  DateTime? _phoneVerifiedAt;

  @override
  void initState() {
    super.initState();
    _controller = ProfileFormController.personalInformation();
    _setupRealtimeListeners();
    _loadUserData();
  }

  void _setupRealtimeListeners() {
    ref.listenManual(authControllerProvider, (previous, next) {
      if (next is AuthStateAuthenticated) {
        if (mounted) {
          setState(() {
            _isEmailVerified = next.emailVerified;
            _isPhoneVerified = next.user.isPhoneVerified ?? false;
          });
        }
      }
    });
  }

  @override
  void dispose() {
    _phoneController.dispose();
    _controller.dispose();
    super.dispose();
  }

  Future<void> _loadUserData() async {
    final authState = ref.read(authControllerProvider);

    if (authState is AuthStateAuthenticated) {
      final user = authState.user;

      // Load existing data (data state, not UI state)
      setState(() {
        _selectedDateOfBirth = user.dateOfBirth;
        _phoneController.text = user.phoneNumber ?? '';
        _isEmailVerified = authState.emailVerified;
        _isPhoneVerified = user.isPhoneVerified ?? false;
      });
    }
  }

  Future<void> _selectDateOfBirth() async {
    // Dismiss keyboard before showing date picker
    FocusManager.instance.primaryFocus?.unfocus();

    final now = DateTime.now();
    final minDate = DateTime(now.year - 13, now.month, now.day);
    final maxDate = DateTime(1924);

    final DateTime? picked = await showDatePicker(
      context: context,
      initialDate: _selectedDateOfBirth ?? minDate,
      firstDate: maxDate,
      lastDate: minDate,
      helpText: 'Select Date of Birth',
      cancelText: 'Cancel',
      confirmText: 'Select',
    );

    if (picked != null && picked != _selectedDateOfBirth) {
      setState(() => _selectedDateOfBirth = picked);
    }
  }

  Future<void> _verifyPhone() async {
    final phoneNumber = _phoneController.text.trim();

    if (phoneNumber.isEmpty) {
      if (mounted) {
        AppSnackBar.showError(context, 'Phone number is required');
      }
      return;
    }

    if (_isPhoneVerified) {
      if (mounted) {
        AppSnackBar.showSuccess(context, 'Phone number already verified!');
      }
      return;
    }

    final cleanedPhone = phoneNumber.replaceAll(RegExp(r'[\s-]'), '');
    if (!CanonicalPhoneValidator.isValid(cleanedPhone)) {
      if (mounted) {
        AppSnackBar.showError(context, 'Invalid phone number format');
      }
      return;
    }

    if (!mounted) return;
    final success = await PhoneVerificationDialog.show(
      context: context,
      phoneNumber: phoneNumber,
      onSuccess: () {
        setState(() {
          _isPhoneVerified = true;
          _phoneVerifiedAt = DateTime.now();
        });
        if (mounted) {
          AppSnackBar.showSuccess(
            context,
            'Phone number verified successfully!',
          );
        }
      },
    );

    if (success && mounted) {
      setState(() {
        _isPhoneVerified = true;
        _phoneVerifiedAt = DateTime.now();
      });
    }
  }

  Future<void> _sendEmailVerification() async {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) {
      AppSnackBar.showError(context, 'User not authenticated');
      return;
    }

    final email = authState.user.email;

    if (email.isEmpty) {
      AppSnackBar.showError(context, 'Email is required');
      return;
    }

    if (!CanonicalEmailValidator.isValid(email)) {
      AppSnackBar.showError(context, 'Invalid email format');
      return;
    }

    _controller.setLoading(true);

    try {
      final success = await ref
          .read(authControllerProvider.notifier)
          .sendEmailVerification();

      if (success && mounted) {
        AppSnackBar.showInfo(
          context,
          'Verification email sent. Please check your inbox and spam folder.',
        );

        _showEmailVerificationDialog();
      } else if (mounted) {
        AppSnackBar.showError(context, 'Failed to send verification email');
      }
    } finally {
      if (mounted) {
        _controller.setLoading(false);
      }
    }
  }

  void _showEmailVerificationDialog() {
    if (!mounted) return;

    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Check Your Email'),
        content: const Text(
          'We have sent a verification link to your email.\n\n'
          'Click the link in the email, then come back here and press the "Refresh" button to continue.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('OK'),
          ),
          ElevatedButton(
            onPressed: () async {
              final scaffoldMessenger = ScaffoldMessenger.of(context);
              final navigator = Navigator.of(context);

              navigator.pop();

              _controller.setLoading(true);

              try {
                await ref
                    .read(authControllerProvider.notifier)
                    .forceRefreshAuthState();
                await _loadUserData();

                if (!mounted) return;

                if (_isEmailVerified) {
                  scaffoldMessenger.showSnackBar(
                    const SnackBar(
                      content: Text('Email verified successfully!'),
                      backgroundColor: AppColors.successGreen,
                      duration: Duration(seconds: 3),
                    ),
                  );
                } else {
                  scaffoldMessenger.showSnackBar(
                    const SnackBar(
                      content: Text(
                        'Email not verified yet. Please click the link in your email first.',
                      ),
                      backgroundColor: AppColors.warningYellow,
                      duration: Duration(seconds: 3),
                    ),
                  );
                }
              } finally {
                if (mounted) {
                  _controller.setLoading(false);
                }
              }
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: AppColors.primaryRed,
            ),
            child: const Text('Refresh'),
          ),
        ],
      ),
    );
  }

  Future<void> _savePersonalInformation() async {
    if (!_formKey.currentState!.validate()) {
      return;
    }

    _controller.setLoading(true);

    try {
      final authState = ref.read(authControllerProvider);
      if (authState is! AuthStateAuthenticated) {
        throw Exception('User not authenticated');
      }

      // Save Date of Birth (if changed)
      if (_selectedDateOfBirth != authState.user.dateOfBirth) {
        final authController = ref.read(authControllerProvider.notifier);
        final success = await authController.updateProfile(
          dateOfBirth: _selectedDateOfBirth,
        );

        if (!success) {
          throw Exception('Failed to save date of birth');
        }
      }

      // Save Phone (if changed)
      final phoneValue = _phoneController.text.trim();
      if (phoneValue.isNotEmpty && phoneValue != authState.user.phoneNumber) {
        // Stage 4D: the save path must respect the canonical phone format
        // authority just like the verify path does. Reject invalid formats
        // before they reach the backend instead of persisting garbage.
        if (!CanonicalPhoneValidator.isValid(phoneValue)) {
          if (mounted) {
            AppSnackBar.showError(context, 'Invalid phone number format');
          }
          return;
        }
        final authController = ref.read(authControllerProvider.notifier);
        final success = await authController.updateProfile(
          phoneNumber: phoneValue,
        );

        if (!success) {
          throw Exception('Failed to save phone number');
        }
      }

      if (!mounted) return;

      _controller.showSuccess('Personal information saved successfully');

      // Wait for animation then navigate back
      await Future.delayed(const Duration(milliseconds: 500));
      if (mounted) {
        Navigator.of(context).pop();
      }
    } catch (e) {
      if (!mounted) return;
      _controller.showError('Failed to save: ${e.toString()}');
    } finally {
      if (mounted) {
        _controller.setLoading(false);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final authState = ref.watch(authControllerProvider);

    return Scaffold(
      appBar: AppBarCustom(title: 'Personal Information'),
      body: ProfileStateView(
        isLoading: _controller.isLoading,
        error: _controller.errorMessage,
        success: _controller.successMessage,
        onErrorDismiss: _controller.clearError,
        onSuccessDismiss: _controller.clearSuccess,
        content: Form(
          key: _formKey,
          child: ListView(
            padding: const EdgeInsets.all(16),
            children: [
              // Contact Information Section
              PersonalInformationSection(
                dateOfBirth: _selectedDateOfBirth,
                onSelectDateOfBirth: _selectDateOfBirth,
                email: authState is AuthStateAuthenticated
                    ? authState.user.email
                    : '',
                emailVerified: _isEmailVerified,
                onVerifyEmail: _sendEmailVerification,
                isLoadingEmailVerification: _controller.isLoading,
                phoneController: _phoneController,
                phoneVerified: _isPhoneVerified,
                phoneVerifiedAt: _phoneVerifiedAt,
                onVerifyPhone: _verifyPhone,
                isDark: isDark,
              ),
              const SizedBox(height: 24),

              // Save Button
              AppButton(
                text: 'Save Changes',
                onPressed: _controller.isLoading
                    ? null
                    : _savePersonalInformation,
                isLoading: _controller.isLoading,
              ),
            ],
          ),
        ),
      ),
    );
  }
}
