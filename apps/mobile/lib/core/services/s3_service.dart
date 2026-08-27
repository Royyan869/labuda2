import 'dart:io';
import 'package:flutter/foundation.dart';
import 'package:dio/dio.dart';
import 'package:blurhash_dart/blurhash_dart.dart';
import 'package:image/image.dart' as img;
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'blurhash_cache_service.dart';

/// Result of an S3 upload operation containing both the object key and public URL.
///
/// [key] — the raw S3 object key (e.g. `images/1749600000000_photo.jpg`).
///          Does not contain scheme or host. Suitable as a DB storage_key.
/// [url] — the public CDN or S3 URL for display (contains https:// scheme).
class S3UploadResult {
  final String key;
  final String url;

  const S3UploadResult({required this.key, required this.url});
}

/// S3Service — client-side S3 upload/delete service.
///
/// ## Credential model (SECURE_STORAGE_SECOND_PASS)
/// AWS credentials are NEVER held by this class. All uploads go through a
/// two-step backend-presigned-URL flow:
///
///   1. Mobile calls the backend presign endpoint to get a short-lived PUT URL
///      and a storage_key.
///   2. Mobile PUTs the file bytes directly to the presigned URL (only the
///      Content-Type header is required — no AWS Authorization header).
///
/// The [ApiClient] is injected once at app startup via [s3ServiceProvider] and
/// cached in the static [_sharedApiClient] field so that call sites that
/// create `S3Service()` directly (e.g. static helpers) also benefit.
///
/// ## KYC uploads
/// Use [uploadKYCDocument] — it calls the dedicated verification presign
/// endpoint (`POST /seller/verification/documents/upload-url`) and returns
/// only the [storage_key]. No document URL is stored for KYC docs; admin
/// views use server-generated presigned GET URLs.
class S3Service {
  // Shared ApiClient — set once from [s3ServiceProvider] at first watch.
  static ApiClient? _sharedApiClient;

  // Plain Dio for S3 PUT requests. No auth interceptors; the presigned URL
  // carries the AWS credential in its query string.
  final Dio _rawDio = Dio();

  S3Service();

  /// Must be called by [s3ServiceProvider] before any upload.
  static void setApiClient(ApiClient client) => _sharedApiClient = client;

  ApiClient get _apiClient {
    final c = _sharedApiClient;
    assert(
      c != null,
      'S3Service: s3ServiceProvider must be watched before use',
    );
    return c!;
  }

  // ──────────────────────────────────────────────────────────────────────────
  // KYC Upload (private bucket — presigned PUT, no public URL)
  // ──────────────────────────────────────────────────────────────────────────

  /// Uploads a KYC document (KTP or selfie) to S3 via a backend-issued
  /// presigned PUT URL and returns the storage_key.
  ///
  /// [documentType] must be `"identity_ktp"` or `"identity_selfie"`.
  ///
  /// The bucket is private; no public URL is returned. Admin views use
  /// presigned GET URLs generated on demand by the backend.
  Future<Result<String>> uploadKYCDocument(
    File imageFile,
    String documentType,
  ) async {
    try {
      final fileName = imageFile.path.split(Platform.pathSeparator).last;
      final ext = fileName.split('.').last.toLowerCase();
      final contentType = _contentTypeFromExt(ext) ?? 'image/jpeg';

      // Step 1: Request presigned PUT URL from backend.
      final presignResp = await _apiClient.post<Map<String, dynamic>>(
        '/seller/verification/documents/upload-url',
        data: {'document_type': documentType, 'content_type': contentType},
      );

      final presignData = presignResp.data?['data'] as Map<String, dynamic>?;
      if (presignData == null) {
        return Result.error('Gagal mendapatkan URL upload KYC');
      }
      final uploadUrl = presignData['upload_url'] as String?;
      final storageKey = presignData['storage_key'] as String?;
      if (uploadUrl == null || storageKey == null) {
        return Result.error('Respons URL upload tidak valid');
      }

      // Step 2: PUT file bytes to presigned URL (no AWS auth headers needed).
      final fileBytes = await imageFile.readAsBytes();
      final putResp = await _rawDio.put(
        uploadUrl,
        data: fileBytes,
        options: Options(headers: {'Content-Type': contentType}),
      );

      if (putResp.statusCode == 200 || putResp.statusCode == 204) {
        return Result.success(storageKey);
      }
      return Result.error('Upload KYC gagal: ${putResp.statusCode}');
    } catch (e) {
      return Result.error('Gagal upload dokumen KYC');
    }
  }

  // ──────────────────────────────────────────────────────────────────────────
  // General Media Uploads (presigned PUT, public CDN URL returned)
  // ──────────────────────────────────────────────────────────────────────────

  /// Requests a presigned PUT URL from the backend for a general media file.
  /// Returns [_MediaPresignResult] with upload_url, storage_key, and public_url.
  ///
  /// When [storageKey] is provided it is passed to the backend as the desired
  /// canonical fixed key (avatars/stores/profile-covers) — the backend
  /// validates ownership and either honors it or rejects the request.
  Future<_MediaPresignResult?> _requestMediaPresignURL(
    String contentType,
    String folder, {
    String? storageKey,
  }) async {
    try {
      final resp = await _apiClient.post<Map<String, dynamic>>(
        '/media/upload-url',
        data: {
          'content_type': contentType,
          'folder': folder,
          'storage_key': ?storageKey,
        },
      );
      final data = resp.data?['data'] as Map<String, dynamic>?;
      if (data == null) return null;
      final uploadUrl = data['upload_url'] as String?;
      final key = data['storage_key'] as String?;
      final publicUrl = data['public_url'] as String?;
      final readUrl = data['read_url'] as String?;
      if (uploadUrl == null || key == null) {
        return null;
      }
      return _MediaPresignResult(
        uploadUrl: uploadUrl,
        storageKey: key,
        publicUrl: publicUrl ?? readUrl ?? '',
        readUrl: readUrl ?? '',
      );
    } catch (_) {
      return null;
    }
  }

  /// PUT file bytes to an S3 presigned URL. Returns true on success.
  Future<bool> _putToPresignedUrl(
    String presignedUrl,
    Uint8List bytes,
    String contentType,
  ) async {
    try {
      final resp = await _rawDio.put(
        presignedUrl,
        data: bytes,
        options: Options(headers: {'Content-Type': contentType}),
      );
      return resp.statusCode == 200 || resp.statusCode == 204;
    } catch (_) {
      return false;
    }
  }

  /// Upload video file to S3 (web blob not supported).
  Future<Result<String>> uploadVideo(File videoFile) async {
    try {
      if (videoFile.path.startsWith('blob:')) {
        return Result.error(
          'Web upload temporarily disabled. Please use mobile app for media upload.',
        );
      }
      const contentType = 'video/mp4';
      final presign = await _requestMediaPresignURL(contentType, 'videos');
      if (presign == null)
        return Result.error('Gagal mendapatkan URL upload video');

      final bytes = await videoFile.readAsBytes();
      final ok = await _putToPresignedUrl(
        presign.uploadUrl,
        bytes,
        contentType,
      );
      if (!ok) return Result.error('Upload video gagal');

      return Result.success(presign.publicUrl);
    } catch (e) {
      return Result.error('Video upload error');
    }
  }

  /// Upload image and return both the S3 object key and the public URL.
  Future<Result<S3UploadResult>> uploadImageWithMeta(File imageFile) async {
    try {
      if (imageFile.path.startsWith('blob:')) {
        return Result.error(
          'Web upload temporarily disabled. Please use mobile app for media upload.',
        );
      }
      final fileName = imageFile.path.split(Platform.pathSeparator).last;
      final contentType =
          _contentTypeFromExt(fileName.split('.').last.toLowerCase()) ??
          'image/jpeg';

      final presign = await _requestMediaPresignURL(contentType, 'images');
      if (presign == null)
        return Result.error('Gagal mendapatkan URL upload gambar');

      final bytes = await imageFile.readAsBytes();
      final ok = await _putToPresignedUrl(
        presign.uploadUrl,
        bytes,
        contentType,
      );
      if (!ok) return Result.error('Upload gambar gagal');

      await _generateAndCacheBlurhash(bytes, presign.publicUrl);
      return Result.success(
        S3UploadResult(key: presign.storageKey, url: presign.publicUrl),
      );
    } catch (e) {
      return Result.error('Image upload error');
    }
  }

  /// Upload video and return both the S3 object key and the public URL.
  Future<Result<S3UploadResult>> uploadVideoWithMeta(File videoFile) async {
    try {
      if (videoFile.path.startsWith('blob:')) {
        return Result.error(
          'Web upload temporarily disabled. Please use mobile app for media upload.',
        );
      }
      const contentType = 'video/mp4';
      final presign = await _requestMediaPresignURL(contentType, 'videos');
      if (presign == null)
        return Result.error('Gagal mendapatkan URL upload video');

      final bytes = await videoFile.readAsBytes();
      final ok = await _putToPresignedUrl(
        presign.uploadUrl,
        bytes,
        contentType,
      );
      if (!ok) return Result.error('Upload video gagal');

      return Result.success(
        S3UploadResult(key: presign.storageKey, url: presign.publicUrl),
      );
    } catch (e) {
      return Result.error('Video upload error');
    }
  }

  /// Upload image to S3 with blurhash generation.
  Future<Result<MediaEntity>> uploadImageWithBlurhash(File imageFile) async {
    try {
      if (imageFile.path.startsWith('blob:')) {
        return Result.error(
          'Web upload temporarily disabled. Please use mobile app for media upload.',
        );
      }
      final fileName = imageFile.path.split(Platform.pathSeparator).last;
      final contentType =
          _contentTypeFromExt(fileName.split('.').last.toLowerCase()) ??
          'image/jpeg';

      final presign = await _requestMediaPresignURL(contentType, 'images');
      if (presign == null)
        return Result.error('Gagal mendapatkan URL upload gambar');

      final bytes = await imageFile.readAsBytes();
      final blurhash = await _generateBlurhash(bytes);
      final ok = await _putToPresignedUrl(
        presign.uploadUrl,
        bytes,
        contentType,
      );
      if (!ok) return Result.error('Upload gambar gagal');

      final mediaEntity = MediaEntity(
        id: DateTime.now().millisecondsSinceEpoch.toString(),
        originalUrl: presign.publicUrl,
        type: MediaType.image,
        blurhash: blurhash,
        createdAt: DateTime.now(),
      );
      return Result.success(mediaEntity);
    } catch (e) {
      return Result.error('Image upload error');
    }
  }

  /// Upload image to S3 (legacy method — returns URL only).
  Future<Result<String>> uploadImage(File imageFile) async {
    try {
      if (imageFile.path.startsWith('blob:')) {
        return Result.error(
          'Web upload temporarily disabled. Please use mobile app for media upload.',
        );
      }
      final fileName = imageFile.path.split(Platform.pathSeparator).last;
      final contentType =
          _contentTypeFromExt(fileName.split('.').last.toLowerCase()) ??
          'image/jpeg';

      final presign = await _requestMediaPresignURL(contentType, 'images');
      if (presign == null)
        return Result.error('Gagal mendapatkan URL upload gambar');

      final bytes = await imageFile.readAsBytes();
      final ok = await _putToPresignedUrl(
        presign.uploadUrl,
        bytes,
        contentType,
      );
      if (!ok) return Result.error('Upload gambar gagal');

      await _generateAndCacheBlurhash(bytes, presign.publicUrl);
      return Result.success(presign.publicUrl);
    } catch (e) {
      return Result.error('Image upload error');
    }
  }

  /// Upload image with a fixed key (avatar/cover replacement pattern).
  ///
  /// NOTE: fixed-key uploads are not supported by the presigned URL flow
  /// (backend controls key generation). This falls through to [uploadImage]
  /// and the [key] parameter is ignored — storage_key is backend-assigned.
  /// Update callers to use [uploadImageWithMeta] and store the returned key.
  ///
  /// @deprecated Use [uploadImageWithFixedKey] which actually requests the
  /// canonical fixed key from the backend.
  Future<Result<String>> uploadImageWithKey(File imageFile, String key) async {
    return uploadImage(imageFile);
  }

  /// Upload an image to a canonical fixed storage key (avatar / cover /
  /// store photo replacement pattern).
  ///
  /// The [key] is passed to the backend `/media/upload-url` endpoint, which
  /// validates caller ownership and mints a presigned PUT for exactly that
  /// key. Returns the backend-confirmed storage key and the read URL.
  Future<Result<S3UploadResult>> uploadImageWithFixedKey(
    File imageFile,
    String key, {
    String mediaLabel = 'gambar',
  }) async {
    try {
      if (imageFile.path.startsWith('blob:')) {
        return Result.error(
          'Web upload temporarily disabled. Please use mobile app for media upload.',
        );
      }
      final fileName = imageFile.path.split(Platform.pathSeparator).last;
      final contentType =
          _contentTypeFromExt(fileName.split('.').last.toLowerCase()) ??
          'image/jpeg';

      final folder = key.contains('/') ? key.split('/').first : 'images';
      final presign = await _requestMediaPresignURL(
        contentType,
        folder,
        storageKey: key,
      );
      if (presign == null) {
        return Result.error('Gagal mendapatkan URL upload $mediaLabel');
      }

      final bytes = await imageFile.readAsBytes();
      final ok = await _putToPresignedUrl(
        presign.uploadUrl,
        bytes,
        contentType,
      );
      if (!ok) return Result.error('Upload $mediaLabel gagal');

      return Result.success(
        S3UploadResult(
          key: presign.storageKey,
          url: presign.readUrl.isNotEmpty ? presign.readUrl : presign.publicUrl,
        ),
      );
    } catch (e) {
      return Result.error('Upload $mediaLabel error');
    }
  }

  /// Upload image bytes with a fixed key (e.g. QR code generation).
  /// Falls through to the general image upload path; [key] is ignored.
  Future<Result<String>> uploadImageBytesWithKey(
    Uint8List imageBytes,
    String key, {
    String contentType = 'image/png',
  }) async {
    try {
      final presign = await _requestMediaPresignURL(contentType, 'images');
      if (presign == null)
        return Result.error('Gagal mendapatkan URL upload gambar');

      final ok = await _putToPresignedUrl(
        presign.uploadUrl,
        imageBytes,
        contentType,
      );
      if (!ok) return Result.error('Upload gambar gagal');
      return Result.success(presign.publicUrl);
    } catch (e) {
      return Result.error('Image upload error');
    }
  }

  /// Delete a file from S3.
  ///
  /// File deletion is handled server-side via S3 lifecycle policies.
  /// This method is a no-op stub that returns success immediately.
  Future<Result<void>> deleteFile(String fileUrl) async {
    // Deletion without AWS credentials is not supported client-side.
    // Server-side lifecycle policies or a future backend DELETE endpoint
    // should handle cleanup.
    return Result.success(null);
  }

  /// Delete multiple files from S3 (stub — see [deleteFile]).
  Future<Result<void>> deleteFiles(List<String> fileUrls) async {
    return Result.success(null);
  }

  // ──────────────────────────────────────────────────────────────────────────
  // Helpers
  // ──────────────────────────────────────────────────────────────────────────

  String? _contentTypeFromExt(String ext) {
    switch (ext) {
      case 'jpg':
      case 'jpeg':
        return 'image/jpeg';
      case 'png':
        return 'image/png';
      case 'webp':
        return 'image/webp';
      case 'gif':
        return 'image/gif';
      case 'mp4':
        return 'video/mp4';
      default:
        return null;
    }
  }

  Future<String?> _generateBlurhash(Uint8List imageBytes) async {
    try {
      final image = img.decodeImage(imageBytes);
      if (image == null) return null;
      final resized = img.copyResize(image, width: 32, height: 32);
      final blurhash = BlurHash.encode(resized, numCompX: 4, numCompY: 3);
      return blurhash.hash;
    } catch (_) {
      return null;
    }
  }

  Future<void> _generateAndCacheBlurhash(
    List<int> fileBytes,
    String imageUrl,
  ) async {
    try {
      final image = img.decodeImage(Uint8List.fromList(fileBytes));
      if (image == null) return;
      final resized = img.copyResize(image, width: 64, height: 64);
      final blurhash = BlurHash.encode(resized, numCompX: 4, numCompY: 3);
      await BlurhashCacheService.instance.setBlurhash(imageUrl, blurhash.hash);
    } catch (_) {
      // Blurhash is optional — never throw.
    }
  }
}

/// Internal result of a backend media presign request.
class _MediaPresignResult {
  final String uploadUrl;
  final String storageKey;
  final String publicUrl;

  /// Canonical read URL (CDN-resolved) returned by the backend; may be empty
  /// when the endpoint only provides public_url.
  final String readUrl;
  const _MediaPresignResult({
    required this.uploadUrl,
    required this.storageKey,
    required this.publicUrl,
    this.readUrl = '',
  });
}
