/// Seller Verification V2 Provider
///
/// Simplified seller verification using V2 API endpoints.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/verification/data/remote/verification_v2_datasource.dart';
import 'package:labuda/domains/user/identity/verification/data/repositories/seller_verification_repository_v2.dart';
import 'package:labuda/domains/user/identity/verification/domain/entities/seller_verification_status.dart';

// =============================================================================
// STATE
// =============================================================================

/// Seller verification state
class SellerVerificationV2State {
  final bool isLoading;
  final bool isVerified;
  final SellerVerificationStatus status;
  final String? rejectionReason;
  final String? errorMessage;

  /// Machine-readable backend error code (e.g.
  /// `EMAIL_VERIFICATION_REQUIRED`, `ACCOUNT_SUSPENDED`,
  /// `ACCOUNT_BANNED`). Set on submit failure when the backend tagged
  /// the response with a known code; null on success or transport-level
  /// failures.
  final String? errorCode;
  final List<Map<String, dynamic>> documents;

  const SellerVerificationV2State({
    this.isLoading = false,
    this.isVerified = false,
    this.status = SellerVerificationStatus.notSubmitted,
    this.rejectionReason,
    this.errorMessage,
    this.errorCode,
    this.documents = const [],
  });

  SellerVerificationV2State copyWith({
    bool? isLoading,
    bool? isVerified,
    SellerVerificationStatus? status,
    String? rejectionReason,
    String? errorMessage,
    String? errorCode,
    List<Map<String, dynamic>>? documents,
  }) {
    return SellerVerificationV2State(
      isLoading: isLoading ?? this.isLoading,
      isVerified: isVerified ?? this.isVerified,
      status: status ?? this.status,
      rejectionReason: rejectionReason ?? this.rejectionReason,
      errorMessage: errorMessage,
      errorCode: errorCode,
      documents: documents ?? this.documents,
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is SellerVerificationV2State &&
          runtimeType == other.runtimeType &&
          isLoading == other.isLoading &&
          isVerified == other.isVerified &&
          status == other.status &&
          rejectionReason == other.rejectionReason &&
          errorMessage == other.errorMessage &&
          errorCode == other.errorCode &&
          documents.length == other.documents.length;

  @override
  int get hashCode =>
      isLoading.hashCode ^
      isVerified.hashCode ^
      status.hashCode ^
      rejectionReason.hashCode ^
      errorMessage.hashCode ^
      errorCode.hashCode ^
      documents.length.hashCode;
}

// =============================================================================
// NOTIFIER
// =============================================================================

/// Seller verification V2 notifier
class SellerVerificationV2Notifier extends Notifier<SellerVerificationV2State> {
  @override
  SellerVerificationV2State build() {
    return const SellerVerificationV2State();
  }

  /// Load verification status
  Future<void> loadStatus() async {
    state = state.copyWith(isLoading: true, errorMessage: null);

    final repository = ref.read(sellerVerificationV2RepositoryProvider);
    final result = await repository.getVerificationStatus();

    result.fold(
      (error) {
        state = state.copyWith(isLoading: false, errorMessage: error);
      },
      (data) {
        state = state.copyWith(
          isLoading: false,
          isVerified: data.isVerified,
          status: data.status,
          rejectionReason: data.rejectionReason,
          documents: data.documents,
        );
      },
    );
  }

  /// Submit KYC documents (KTP + selfie atomically)
  Future<bool> submitKYC({
    required String fullName,
    required String nationalId,
    required String ktpStorageKey,
    required String selfieStorageKey,
  }) async {
    state = state.copyWith(
      isLoading: true,
      errorMessage: null,
      errorCode: null,
    );

    final repository = ref.read(sellerVerificationV2RepositoryProvider);
    final result = await repository.submitKYC(
      fullName: fullName,
      nationalId: nationalId,
      ktpStorageKey: ktpStorageKey,
      selfieStorageKey: selfieStorageKey,
    );

    if (result.isError) {
      // Preserve the backend's machine-readable code (e.g.
      // EMAIL_VERIFICATION_REQUIRED, ACCOUNT_SUSPENDED, ACCOUNT_BANNED) so
      // the screen can branch to the existing email-verification gate or a
      // blocking dialog instead of falling back to a generic snackbar.
      state = state.copyWith(
        isLoading: false,
        errorMessage: result.error,
        errorCode: result.errorCode,
      );
      return false;
    }

    // Refresh status after submission
    loadStatus();
    return true;
  }

  /// Load documents
  Future<void> loadDocuments() async {
    // Canonical source is GET /seller/verification/status.
    await loadStatus();
  }

  /// Delete document
  Future<bool> deleteDocument(String documentId) async {
    state = state.copyWith(isLoading: true);
    Result<void> result;
    try {
      final repository = ref.read(sellerVerificationV2RepositoryProvider);
      result = await repository.deleteDocument(documentId);
    } on UnsupportedError catch (e) {
      state = state.copyWith(
        isLoading: false,
        errorMessage: e.message?.toString() ?? e.toString(),
      );
      return false;
    }

    return result.fold(
      (error) {
        state = state.copyWith(isLoading: false, errorMessage: error);
        return false;
      },
      (_) {
        loadStatus();
        return true;
      },
    );
  }

  /// Reset state
  void reset() {
    state = const SellerVerificationV2State();
  }
}

// =============================================================================
// PROVIDERS
// =============================================================================

/// V2 Datasource provider
final verificationV2DatasourceProvider = Provider<VerificationV2Datasource>((
  ref,
) {
  final apiClient = ref.watch(apiClientProvider);
  return VerificationV2Datasource(apiClient);
});

/// V2 Repository provider
final sellerVerificationV2RepositoryProvider =
    Provider<SellerVerificationRepositoryV2>((ref) {
      final datasource = ref.watch(verificationV2DatasourceProvider);
      final logger = ref.watch(loggerServiceProvider);
      return SellerVerificationRepositoryV2(
        datasource: datasource,
        logger: logger,
      );
    });

/// V2 Notifier provider
final sellerVerificationV2NotifierProvider =
    NotifierProvider<SellerVerificationV2Notifier, SellerVerificationV2State>(
      SellerVerificationV2Notifier.new,
      dependencies: [sellerVerificationV2RepositoryProvider],
    );

/// V2 State provider (auto-loads on first access)
final sellerVerificationV2StateProvider = Provider<SellerVerificationV2State>(
  (ref) {
    final notifier = ref.watch(sellerVerificationV2NotifierProvider.notifier);

    // Load on first access
    ref.onAddListener(() {
      final currentState = ref.read(sellerVerificationV2NotifierProvider);
      if (!currentState.isLoading &&
          currentState.status == SellerVerificationStatus.notSubmitted) {
        notifier.loadStatus();
      }
    });

    return ref.watch(sellerVerificationV2NotifierProvider);
  },
  dependencies: [
    sellerVerificationV2NotifierProvider,
    sellerVerificationV2RepositoryProvider,
  ],
);

/// Is seller verified provider (Future-based for quick checks)
final isSellerVerifiedV2Provider = FutureProvider<bool>((ref) async {
  final repository = ref.watch(sellerVerificationV2RepositoryProvider);
  final result = await repository.isSellerVerified();
  return result.fold((error) => false, (isVerified) => isVerified);
}, dependencies: [sellerVerificationV2RepositoryProvider]);
