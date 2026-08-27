/// Verification V2 API Datasource (Go Backend)
///
/// Uses the new V2 verification endpoints for identity verification.
library;

import 'package:dio/dio.dart';
import 'package:labuda/core/api/api_client.dart';

/// Verification status DTO from backend canonical response.
///
/// Backend shape: GET /api/v1/seller/verification/status
///   { "status": "<8-state-lifecycle>", "seller_id": "...",
///     "submitted_at": "...", "reviewed_at": "...", "reason": "...",
///     "documents": [...] }
class VerificationStatusDto {
  /// Canonical 8-state lifecycle value (see SellerVerificationStatus entity).
  final String status;
  final String? submittedAt;
  final String? reviewedAt;
  final String? reason;
  final List<Map<String, dynamic>> documents;

  const VerificationStatusDto({
    required this.status,
    this.submittedAt,
    this.reviewedAt,
    this.reason,
    this.documents = const [],
  });

  factory VerificationStatusDto.fromJson(Map<String, dynamic> json) {
    final rawDocuments = json['documents'] as List<dynamic>? ?? const [];
    final documents = rawDocuments
        .whereType<Map<String, dynamic>>()
        .map(
          (doc) => <String, dynamic>{
            'id': doc['id'],
            'document_type': doc['document_type'],
            'document_url': doc['document_url'],
            'status': doc['status'],
            'rejection_note': doc['rejection_note'],
            // Backend canonical is submitted_at/reviewed_at. Keep uploaded_at
            // alias for older UI call sites that still expect it.
            'submitted_at': doc['submitted_at'],
            'uploaded_at': doc['submitted_at'] ?? doc['uploaded_at'],
            'reviewed_at': doc['reviewed_at'],
          },
        )
        .toList();
    return VerificationStatusDto(
      status: json['status'] as String? ?? 'not_submitted',
      submittedAt: json['submitted_at'] as String?,
      reviewedAt: json['reviewed_at'] as String?,
      reason: json['reason'] as String?,
      documents: documents,
    );
  }
}

/// Request upload URL from the backend (KYC documents).
///
/// POST /seller/verification/documents/upload-url
/// Returns a short-lived presigned PUT URL and the backend-assigned storage_key.
class UploadURLRequest {
  final String documentType; // 'identity_ktp' or 'identity_selfie'
  final String contentType; // 'image/jpeg', 'image/png', or 'image/webp'

  const UploadURLRequest({
    required this.documentType,
    required this.contentType,
  });

  Map<String, dynamic> toJson() => {
    'document_type': documentType,
    'content_type': contentType,
  };
}

/// Backend response for upload URL request.
class UploadURLResponse {
  final String storageKey;
  final String uploadUrl;
  final String expiresAt;

  const UploadURLResponse({
    required this.storageKey,
    required this.uploadUrl,
    required this.expiresAt,
  });

  factory UploadURLResponse.fromJson(Map<String, dynamic> json) {
    final data = json['data'] as Map<String, dynamic>? ?? json;
    return UploadURLResponse(
      storageKey: data['storage_key'] as String,
      uploadUrl: data['upload_url'] as String,
      expiresAt: data['expires_at'] as String? ?? '',
    );
  }
}

/// Submit KYC request DTO
///
/// KYC scope (owner decision): KTP + selfie only.
/// Both documents must be uploaded via /seller/verification/documents/upload-url
/// before calling this endpoint. Pass storage_key values returned by that endpoint.
/// No document URLs are accepted — admin reads use presigned GET URLs.
class SubmitKYCRequest {
  final String fullName;
  final String nationalId;
  final String ktpStorageKey;
  final String selfieStorageKey;

  const SubmitKYCRequest({
    required this.fullName,
    required this.nationalId,
    required this.ktpStorageKey,
    required this.selfieStorageKey,
  });

  Map<String, dynamic> toJson() {
    return {
      'full_name': fullName,
      'national_id': nationalId,
      'ktp_storage_key': ktpStorageKey,
      'selfie_storage_key': selfieStorageKey,
    };
  }
}

/// Submit KYC response DTO
class SubmitKYCResponse {
  final String sellerStatus;
  final String message;

  const SubmitKYCResponse({required this.sellerStatus, required this.message});

  factory SubmitKYCResponse.fromJson(Map<String, dynamic> json) {
    final data = json['data'] as Map<String, dynamic>? ?? json;
    return SubmitKYCResponse(
      sellerStatus: data['seller_status'] as String? ?? 'pending_review',
      message: data['message'] as String? ?? json['message'] as String? ?? '',
    );
  }
}

/// Verification V2 Datasource
///
/// API Endpoints:
/// - GET  /seller/verification/status
/// - POST /seller/verification/submit
class VerificationV2Datasource {
  final ApiClient _apiClient;

  VerificationV2Datasource(this._apiClient);

  /// Get verification status
  ///
  /// GET /seller/verification/status
  Future<VerificationStatusDto?> getVerificationStatus() async {
    try {
      final response = await _apiClient.get<Map<String, dynamic>>(
        '/seller/verification/status',
      );

      if (response.data != null) {
        final data = response.data!['data'] as Map<String, dynamic>?;
        if (data != null) {
          return VerificationStatusDto.fromJson(data);
        }
      }
      return null;
    } on DioException catch (e) {
      if (e.response?.statusCode == 404) {
        // Seller has no verification row yet — treat as not_submitted.
        return const VerificationStatusDto(status: 'not_submitted');
      }
      return null;
    }
  }

  /// Request a presigned PUT URL for a KYC document.
  ///
  /// POST /seller/verification/documents/upload-url
  Future<UploadURLResponse> requestUploadURL(UploadURLRequest request) async {
    final response = await _apiClient.post<Map<String, dynamic>>(
      '/seller/verification/documents/upload-url',
      data: request.toJson(),
    );
    if (response.data != null) {
      return UploadURLResponse.fromJson(response.data!);
    }
    throw Exception('Failed to get upload URL');
  }

  /// Submit KYC documents (KTP + selfie atomically)
  ///
  /// POST /seller/verification/submit
  Future<SubmitKYCResponse> submitKYC(SubmitKYCRequest request) async {
    final response = await _apiClient.post<Map<String, dynamic>>(
      '/seller/verification/submit',
      data: request.toJson(),
    );

    if (response.data != null) {
      return SubmitKYCResponse.fromJson(response.data!);
    }
    throw Exception('Failed to submit KYC documents');
  }
}
