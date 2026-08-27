/// Seller Verification Repository V2
///
/// Uses V2 API endpoints for simplified seller verification flow.
/// State model uses the canonical 8-state SellerVerificationStatus entity.
library;

import 'package:dio/dio.dart';
import 'package:labuda/core/api/exceptions/api_exception.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/domains/user/identity/verification/data/remote/verification_v2_datasource.dart';
import 'package:labuda/domains/user/identity/verification/domain/entities/seller_verification_status.dart';

/// Seller verification data — wraps canonical lifecycle status.
class SellerVerificationData {
  final bool isVerified;
  final SellerVerificationStatus status;
  final String? rejectionReason;
  final List<Map<String, dynamic>> documents;

  const SellerVerificationData({
    required this.isVerified,
    required this.status,
    this.rejectionReason,
    this.documents = const [],
  });

  /// Parse from backend DTO. isVerified is true only when status == approved.
  factory SellerVerificationData.fromDto(VerificationStatusDto dto) {
    final status = SellerVerificationStatusExtension.parse(dto.status);
    return SellerVerificationData(
      isVerified: status == SellerVerificationStatus.approved,
      status: status,
      rejectionReason: dto.reason,
      documents: dto.documents,
    );
  }

  static const notSubmitted = SellerVerificationData(
    isVerified: false,
    status: SellerVerificationStatus.notSubmitted,
    rejectionReason: null,
    documents: [],
  );
}

/// Repository for seller verification operations using V2 API
class SellerVerificationRepositoryV2 {
  final VerificationV2Datasource _datasource;
  final ILoggerService _logger;

  SellerVerificationRepositoryV2({
    required VerificationV2Datasource datasource,
    required ILoggerService logger,
  }) : _datasource = datasource,
       _logger = logger;

  /// Get seller verification status
  Future<Result<SellerVerificationData>> getVerificationStatus() async {
    try {
      _logger.info('Getting seller verification status');

      final dto = await _datasource.getVerificationStatus();
      if (dto == null) {
        return Result.success(SellerVerificationData.notSubmitted);
      }

      final data = SellerVerificationData.fromDto(dto);
      return Result.success(data);
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to get verification status',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Failed to get verification status: $e');
    }
  }

  /// Submit KYC documents (KTP + selfie atomically)
  Future<Result<void>> submitKYC({
    required String fullName,
    required String nationalId,
    required String ktpStorageKey,
    required String selfieStorageKey,
  }) async {
    try {
      _logger.info(
        'Submitting KYC documents',
        extra: {'fullName': fullName, 'nationalId': nationalId},
      );

      final request = SubmitKYCRequest(
        fullName: fullName,
        nationalId: nationalId,
        ktpStorageKey: ktpStorageKey,
        selfieStorageKey: selfieStorageKey,
      );

      await _datasource.submitKYC(request);

      _logger.info('KYC documents submitted successfully');
      return Result.success(null);
    } on DioException catch (e, stackTrace) {
      // Preserve the machine-readable error code so the call site can react
      // to known codes like EMAIL_VERIFICATION_REQUIRED / ACCOUNT_SUSPENDED /
      // ACCOUNT_BANNED / MISSING_REQUIREMENTS.
      final apiEx = e.error is ApiException ? e.error as ApiException : null;
      _logger.error(
        'Failed to submit KYC documents',
        extra: {'error': e.toString(), 'code': apiEx?.code},
        stackTrace: stackTrace,
      );
      if (apiEx != null) {
        return Result.error(
          apiEx.message,
          code: apiEx.code,
          statusCode: apiEx.statusCode,
        );
      }
      return Result.error('Failed to submit KYC: ${e.message ?? e}');
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to submit KYC documents',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Failed to submit KYC: $e');
    }
  }

  /// Get verification documents from canonical status endpoint.
  Future<Result<List<Map<String, dynamic>>>> getDocuments() async {
    try {
      final dto = await _datasource.getVerificationStatus();
      if (dto == null) {
        return Result.success(<Map<String, dynamic>>[]);
      }
      return Result.success(dto.documents);
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to get documents',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Failed to get documents: $e');
    }
  }

  /// Delete verification document
  Future<Result<void>> deleteDocument(String documentId) async {
    _logger.warning(
      'Delete verification document is unsupported by backend contract',
      extra: {'documentId': documentId},
    );
    throw UnsupportedError(
      'Deleting verification documents is not supported by backend contract.',
    );
  }

  /// Check if seller is verified (approved status only)
  Future<Result<bool>> isSellerVerified() async {
    final result = await getVerificationStatus();
    return result.fold(
      (error) => Result.success(false),
      (data) => Result.success(data.isVerified),
    );
  }
}
