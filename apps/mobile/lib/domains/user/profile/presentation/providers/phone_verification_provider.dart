import 'dart:async';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/services/phone_verification_service.dart'
    show PhoneVerificationService;

/// Unified state untuk phone verification (send + verify)
class PhoneVerificationState {
  final bool isLoading;
  final bool isResending;
  final bool codeSent;
  final bool isVerifying;
  final bool isVerified;
  final String? verificationId;
  final String? errorMessage;
  final int resendCountdown;
  final String? verifiedPhoneNumber;

  const PhoneVerificationState({
    this.isLoading = false,
    this.isResending = false,
    this.codeSent = false,
    this.isVerifying = false,
    this.isVerified = false,
    this.verificationId,
    this.errorMessage,
    this.resendCountdown = 0,
    this.verifiedPhoneNumber,
  });

  PhoneVerificationState copyWith({
    bool? isLoading,
    bool? isResending,
    bool? codeSent,
    bool? isVerifying,
    bool? isVerified,
    String? verificationId,
    String? errorMessage,
    int? resendCountdown,
    String? verifiedPhoneNumber,
  }) {
    return PhoneVerificationState(
      isLoading: isLoading ?? this.isLoading,
      isResending: isResending ?? this.isResending,
      codeSent: codeSent ?? this.codeSent,
      isVerifying: isVerifying ?? this.isVerifying,
      isVerified: isVerified ?? this.isVerified,
      verificationId: verificationId ?? this.verificationId,
      errorMessage: errorMessage,
      resendCountdown: resendCountdown ?? this.resendCountdown,
      verifiedPhoneNumber: verifiedPhoneNumber ?? this.verifiedPhoneNumber,
    );
  }
}

/// Provider untuk phone verification service instance
final phoneVerificationServiceProvider = Provider<PhoneVerificationService>((
  ref,
) {
  return PhoneVerificationService();
});

/// Unified Notifier untuk managing phone verification (send + verify)
class PhoneVerificationNotifier extends Notifier<PhoneVerificationState> {
  late PhoneVerificationService _service;
  Timer? _countdownTimer;

  @override
  PhoneVerificationState build() {
    _service = ref.read(phoneVerificationServiceProvider);
    return const PhoneVerificationState();
  }

  void dispose() {
    _countdownTimer?.cancel();
    _service.clearSession();
  }

  /// Send OTP to phone number
  Future<Result<void>> sendOTP(String phoneNumber) async {
    state = state.copyWith(isLoading: true, errorMessage: null);

    // Format phone number
    final formattedNumber = _service.formatPhoneNumber(phoneNumber);

    if (!_service.isValidPhoneNumber(formattedNumber)) {
      state = state.copyWith(
        isLoading: false,
        errorMessage: 'Invalid phone number format',
      );
      return Result.error('Invalid phone number format');
    }

    return await _sendOTPInternal(formattedNumber);
  }

  /// Resend OTP
  Future<Result<void>> resendOTP(String phoneNumber) async {
    if (state.resendCountdown > 0) {
      return Result.error(
        'Wait ${state.resendCountdown} seconds before resending',
      );
    }

    state = state.copyWith(isResending: true, errorMessage: null);
    final result = await _sendOTPInternal(phoneNumber, isResend: true);
    return result;
  }

  /// Internal method for sending OTP
  Future<Result<void>> _sendOTPInternal(
    String phoneNumber, {
    bool isResend = false,
  }) async {
    try {
      await _service.sendOTP(
        phoneNumber: phoneNumber,
        onCodeSent: (verificationId) {
          state = state.copyWith(
            verificationId: verificationId,
            codeSent: true,
            isLoading: false,
            isResending: false,
            resendCountdown: 60,
            errorMessage: null,
          );
          _startResendCountdown();
        },
        onVerificationFailed: (error) {
          state = state.copyWith(
            errorMessage: error,
            isLoading: false,
            isResending: false,
          );
        },
        onVerificationCompleted: () {
          // Auto-verification completed (rare, mostly on iOS)
          state = state.copyWith(
            isLoading: false,
            isResending: false,
            isVerified: true,
            verifiedPhoneNumber: phoneNumber,
            errorMessage: null,
          );
        },
      );

      return Result.success(null);
    } catch (e) {
      state = state.copyWith(
        errorMessage: e.toString(),
        isLoading: false,
        isResending: false,
      );
      return Result.error(e.toString());
    }
  }

  /// Verify OTP code
  Future<Result<String>> verifyOTP(String otpCode, String phoneNumber) async {
    if (otpCode.length != 6) {
      state = state.copyWith(errorMessage: 'OTP code must be 6 digits');
      return Result.error('OTP code must be 6 digits');
    }

    state = state.copyWith(isVerifying: true, errorMessage: null);

    try {
      final success = await _service.verifyOTP(otpCode);

      if (success) {
        state = state.copyWith(
          isVerifying: false,
          isVerified: true,
          verifiedPhoneNumber: phoneNumber,
          errorMessage: null,
        );
        return Result.success(phoneNumber);
      } else {
        state = state.copyWith(
          isVerifying: false,
          errorMessage: 'Verification failed',
        );
        return Result.error('Verification failed');
      }
    } catch (e) {
      String errorMessage = 'Verification failed';

      if (e.toString().contains('invalid-verification-code')) {
        errorMessage = 'Invalid OTP code';
      } else if (e.toString().contains('session-expired')) {
        errorMessage = 'OTP code expired. Send new code';
      } else if (e.toString().contains('credential-already-in-use')) {
        errorMessage = 'Phone number already registered to another account';
      } else {
        errorMessage = e.toString().replaceAll('Exception: ', '');
      }

      state = state.copyWith(isVerifying: false, errorMessage: errorMessage);
      return Result.error(errorMessage);
    }
  }

  /// Start countdown timer for resend button
  void _startResendCountdown() {
    _countdownTimer?.cancel();
    _countdownTimer = Timer.periodic(const Duration(seconds: 1), (timer) {
      if (state.resendCountdown <= 0) {
        timer.cancel();
        return;
      }
      state = state.copyWith(resendCountdown: state.resendCountdown - 1);
    });
  }

  /// Clear error message
  void clearError() {
    state = state.copyWith(errorMessage: null);
  }

  /// Reset to initial state
  void reset() {
    _countdownTimer?.cancel();
    _service.clearSession();
    state = const PhoneVerificationState();
  }
}

/// Provider instance
final phoneVerificationProvider =
    NotifierProvider<PhoneVerificationNotifier, PhoneVerificationState>(() {
      return PhoneVerificationNotifier();
    });
