import 'dart:io';
import 'package:flutter/foundation.dart';
import 'package:dio/dio.dart';
import 'package:blurhash_dart/blurhash_dart.dart';
import 'package:image/image.dart' as img;
import 'package:hishumi/core/api/api_client.dart';
import 'package:hishumi/core/api/exceptions/api_exception.dart';
import 'package:hishumi/core/common/result.dart';
import 'package:hishumi/domains/social/content/domain/entities/content.dart';
import 'blurhash_cache_service.dart';

/// Result of an S3 upload operation containing both the object key and read URL.
///
/// [key] — the raw S3 object key (e.g. `images/1749600000000_photo.jpg`).
///          Does not contain scheme or host. Suitable as a DB storage_key.
/// [url] — the canonical read/display URL (`read_url` from `/media/upload-url`,
///          CDN-resolved when configured, else raw S3). Contains https:// scheme.
class S3UploadResult {
  final String key;
  final String url;

  const S3UploadResult({required this.key, required this.url});
}

/// S3Service — client-side S3 upload service.
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
  Dio _rawDio = Dio();

  S3Service();

  /// Must be called by [s3ServiceProvider] before any upload.
  static void setApiClient(ApiClient client) => _sharedApiClient = client;

  /// Test seam: inject the transport used for presigned PUT requests.
  ///
  /// Presigned PUTs deliberately bypass [ApiClient] (they must not carry the
  /// Labuda JWT), so they cannot be isolated through `ApiClient.dio`. Production
  /// never calls this; the default remains a plain [Dio].
  @visibleForTesting
  void setRawDioForTest(Dio dio) => _rawDio = dio;

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
      final putResult = await _putToPresignedUrl(uploadUrl, fileBytes, contentType);
      if (putResult.isError) {
        return Result.error(
          putResult.error ?? 'Upload KYC gagal',
          code: putResult.errorCode,
          statusCode: putResult.statusCode,
        );
      }
      return Result.success(storageKey);
    } on DioException catch (e) {
      final ex = _extractApiException(e);
      return Result.error(
        ex.message,
        code: ex.code,
        statusCode: ex.statusCode,
      );
    } catch (e) {
      return Result.error('Gagal upload dokumen KYC');
    }
  }

  // ──────────────────────────────────────────────────────────────────────────
  // General Media Uploads (presigned PUT, canonical read_url)
  // ──────────────────────────────────────────────────────────────────────────

  /// Requests a presigned PUT URL from the backend for a general media file.
  /// Returns a [Result] with [_MediaPresignResult] on success, preserving
  /// backend error code/status when available.
  ///
  /// When [storageKey] is provided it is passed to the backend as the desired
  /// canonical fixed key (avatars/stores/profile-covers) — the backend
  /// validates ownership and either honors it or rejects the request.
  Future<Result<_MediaPresignResult>> _requestMediaPresignURL(
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
      final raw = resp.data;
      // Handle backend error envelope for non-thrown 4xx (validateStatus <500)
      if (raw is Map<String, dynamic>) {
        final success = raw['success'] as bool?;
        if (success == false && raw['error'] is Map<String, dynamic>) {
          final err = raw['error'] as Map<String, dynamic>;
          final code = err['code'] as String?;
          final message = err['message'] as String? ?? 'Presign failed';
          return Result.error(
            message,
            code: code,
            statusCode: resp.statusCode,
          );
        }
      }
      final data = raw?['data'] as Map<String, dynamic>?;
      if (data == null) {
        return Result.error(
          'Gagal mendapatkan URL upload',
          code: 'EMPTY_DATA',
          statusCode: resp.statusCode,
        );
      }
      final uploadUrl = data['upload_url'] as String?;
      final key = data['storage_key'] as String?;
      final readUrl = data['read_url'] as String?;
      if (uploadUrl == null || key == null) {
        return Result.error(
          'Respons URL upload tidak valid',
          code: 'INVALID_RESPONSE',
          statusCode: resp.statusCode,
        );
      }
      if (readUrl == null || readUrl.isEmpty) {
        return Result.error(
          'Respons read_url tidak valid',
          code: 'INVALID_RESPONSE',
          statusCode: resp.statusCode,
        );
      }
      return Result.success(
        _MediaPresignResult(
          uploadUrl: uploadUrl,
          storageKey: key,
          readUrl: readUrl,
        ),
      );
    } on DioException catch (e) {
      final ex = _extractApiException(e);
      return Result.error(
        ex.message,
        code: ex.code,
        statusCode: ex.statusCode,
        details: ex.details is Map<String, dynamic>
            ? ex.details as Map<String, dynamic>
            : null,
      );
    } catch (e) {
      return Result.error('Gagal mendapatkan URL upload: ${e.toString()}');
    }
  }

  ApiException _extractApiException(DioException e) {
    try {
      return _apiClient.extractException(e);
    } catch (_) {
      return UnknownApiException(
        message: e.message ?? 'Presign failed',
        statusCode: e.response?.statusCode,
      );
    }
  }

  /// Byte progress of the CURRENT presigned PUT, or null when nobody asked.
  ///
  /// Every upload helper funnels into `_putToPresignedUrl`, so this is the ONE
  /// place upload progress exists — image, video, chat media, KYC, grid. It is
  /// installed for the length of a batch and cleared in `finally` by the
  /// orchestrator, so it can never outlive the batch it belongs to.
  void Function(int sent, int total)? onPutProgress;

  /// PUT file bytes to an S3 presigned URL. Returns `Result<void>` preserving
  /// HTTP status on failure. No GET/HEAD verification is performed.
  Future<Result<void>> _putToPresignedUrl(
    String presignedUrl,
    Uint8List bytes,
    String contentType,
  ) async {
    try {
      final resp = await _rawDio.put(
        presignedUrl,
        data: bytes,
        options: Options(headers: {'Content-Type': contentType}),
        onSendProgress: onPutProgress,
      );
      final code = resp.statusCode;
      if (code == 200 || code == 204) {
        return Result.success(null);
      }
      return Result.error(
        'Upload gagal: $code',
        statusCode: code,
      );
    } on DioException catch (e) {
      final statusCode = e.response?.statusCode;
      final message = e.message ?? 'Upload gagal';
      // Raw Dio has no ErrorInterceptor, so use DioException details directly.
      // For body-provided errors, try to surface S3 message without leaking URL.
      String? bodyMessage;
      final data = e.response?.data;
      if (data is String && data.isNotEmpty && data.length < 500) {
        bodyMessage = data;
      }
      return Result.error(
        bodyMessage != null ? 'Upload gagal: $bodyMessage' : 'Upload gagal: $message',
        statusCode: statusCode,
      );
    } catch (e) {
      return Result.error('Upload error: ${e.toString()}');
    }
  }

  /// Registers AND uploads ONE chat media file, returning the asset id the
  /// message must reference.
  ///
  /// This is the canonical chat media pipeline (register → upload → attach):
  ///   1. POST /chat/rooms/:room_id/media creates a PENDING, room-scoped asset
  ///      and returns the presigned PUT URL for its server-minted key.
  ///   2. The bytes are PUT to that URL here.
  ///   3. The caller sends the message with `media_asset_ids: [assetId, ...]`.
  ///
  /// Nothing is attached by uploading: an abandoned composer leaves a PENDING
  /// asset that expires on its own, so a cancel costs nothing and no orphan
  /// authority survives.
  Future<Result<ChatMediaUpload>> uploadChatMedia({
    required String roomId,
    required File file,
  }) async {
    try {
      if (file.path.startsWith('blob:')) {
        return Result.error(
          'Web upload temporarily disabled. Please use mobile app for media upload.',
        );
      }
      final contentType = _extractExtension(file.path);
      final mimeType = contentType == null
          ? null
          : _contentTypeFromExt(contentType);
      if (mimeType == null) {
        return Result.error('Tipe media tidak didukung');
      }
      final byteSize = await file.length();

      final resp = await _apiClient.post<Map<String, dynamic>>(
        '/chat/rooms/$roomId/media',
        data: {'content_type': mimeType, 'byte_size': byteSize},
      );
      final raw = resp.data;
      if (raw is Map<String, dynamic>) {
        final success = raw['success'] as bool?;
        if (success == false && raw['error'] is Map<String, dynamic>) {
          final err = raw['error'] as Map<String, dynamic>;
          return Result.error(
            err['message'] as String? ?? 'Gagal mendaftarkan media',
            code: err['code'] as String?,
            statusCode: resp.statusCode,
          );
        }
      }

      final data = raw?['data'] as Map<String, dynamic>?;
      final assetId = data?['asset_id'] as String?;
      final uploadUrl = data?['upload_url'] as String?;
      final readUrl = data?['read_url'] as String?;
      if (assetId == null || uploadUrl == null || readUrl == null) {
        return Result.error(
          'Respons pendaftaran media tidak valid',
          code: 'INVALID_RESPONSE',
          statusCode: resp.statusCode,
        );
      }

      final putResult = await _putToPresignedUrl(
        uploadUrl,
        await file.readAsBytes(),
        mimeType,
      );
      if (putResult.isError) {
        return Result.error(
          putResult.error ?? 'Upload media gagal',
          code: putResult.errorCode,
          statusCode: putResult.statusCode,
        );
      }

      return Result.success(ChatMediaUpload(assetId: assetId, readUrl: readUrl));
    } on DioException catch (e) {
      final ex = _extractApiException(e);
      return Result.error(
        ex.message,
        code: ex.code,
        statusCode: ex.statusCode,
        details: ex.details is Map<String, dynamic>
            ? ex.details as Map<String, dynamic>
            : null,
      );
    } catch (e) {
      return Result.error('Gagal upload media: ${e.toString()}');
    }
  }

  /// Lowercased file extension without the dot, or null when absent.
  static String? _extractExtension(String path) {
    final name = path.split('/').last;
    final dot = name.lastIndexOf('.');
    if (dot < 0 || dot == name.length - 1) return null;
    return name.substring(dot + 1).toLowerCase();
  }

  /// Upload video file to S3 (web blob not supported).
  Future<Result<String>> uploadVideo(File videoFile, {required String folder}) async {
    try {
      if (videoFile.path.startsWith('blob:')) {
        return Result.error(
          'Web upload temporarily disabled. Please use mobile app for media upload.',
        );
      }
      const contentType = 'video/mp4';
      final presignResult = await _requestMediaPresignURL(contentType, folder);
      if (presignResult.isError) {
        return Result.error(
          presignResult.error ?? 'Gagal mendapatkan URL upload video',
          code: presignResult.errorCode,
          statusCode: presignResult.statusCode,
          details: presignResult.errorDetails,
        );
      }
      final presign = presignResult.data!;
      final bytes = await videoFile.readAsBytes();
      final putResult = await _putToPresignedUrl(
        presign.uploadUrl,
        bytes,
        contentType,
      );
      if (putResult.isError) {
        return Result.error(
          putResult.error ?? 'Upload video gagal',
          code: putResult.errorCode,
          statusCode: putResult.statusCode,
        );
      }

      return Result.success(presign.readUrl);
    } catch (e) {
      return Result.error('Video upload error');
    }
  }

  /// Upload image and return both the S3 object key and the read URL.
  Future<Result<S3UploadResult>> uploadImageWithMeta(File imageFile, {required String folder}) async {
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

      final presignResult = await _requestMediaPresignURL(contentType, folder);
      if (presignResult.isError) {
        return Result.error(
          presignResult.error ?? 'Gagal mendapatkan URL upload gambar',
          code: presignResult.errorCode,
          statusCode: presignResult.statusCode,
          details: presignResult.errorDetails,
        );
      }
      final presign = presignResult.data!;
      final bytes = await imageFile.readAsBytes();
      final putResult = await _putToPresignedUrl(
        presign.uploadUrl,
        bytes,
        contentType,
      );
      if (putResult.isError) {
        return Result.error(
          putResult.error ?? 'Upload gambar gagal',
          code: putResult.errorCode,
          statusCode: putResult.statusCode,
        );
      }

      await _generateAndCacheBlurhash(bytes, presign.readUrl);
      return Result.success(
        S3UploadResult(key: presign.storageKey, url: presign.readUrl),
      );
    } catch (e) {
      return Result.error('Image upload error');
    }
  }

  /// Upload video and return both the S3 object key and the read URL.
  Future<Result<S3UploadResult>> uploadVideoWithMeta(File videoFile, {required String folder}) async {
    try {
      if (videoFile.path.startsWith('blob:')) {
        return Result.error(
          'Web upload temporarily disabled. Please use mobile app for media upload.',
        );
      }
      const contentType = 'video/mp4';
      final presignResult = await _requestMediaPresignURL(contentType, folder);
      if (presignResult.isError) {
        return Result.error(
          presignResult.error ?? 'Gagal mendapatkan URL upload video',
          code: presignResult.errorCode,
          statusCode: presignResult.statusCode,
          details: presignResult.errorDetails,
        );
      }
      final presign = presignResult.data!;
      final bytes = await videoFile.readAsBytes();
      final putResult = await _putToPresignedUrl(
        presign.uploadUrl,
        bytes,
        contentType,
      );
      if (putResult.isError) {
        return Result.error(
          putResult.error ?? 'Upload video gagal',
          code: putResult.errorCode,
          statusCode: putResult.statusCode,
        );
      }

      return Result.success(
        S3UploadResult(key: presign.storageKey, url: presign.readUrl),
      );
    } catch (e) {
      return Result.error('Video upload error');
    }
  }

  /// Upload image to S3 with blurhash generation.
  Future<Result<MediaEntity>> uploadImageWithBlurhash(File imageFile, {required String folder}) async {
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

      final presignResult = await _requestMediaPresignURL(contentType, folder);
      if (presignResult.isError) {
        return Result.error(
          presignResult.error ?? 'Gagal mendapatkan URL upload gambar',
          code: presignResult.errorCode,
          statusCode: presignResult.statusCode,
          details: presignResult.errorDetails,
        );
      }
      final presign = presignResult.data!;
      final bytes = await imageFile.readAsBytes();
      final blurhash = await _generateBlurhash(bytes);
      final putResult = await _putToPresignedUrl(
        presign.uploadUrl,
        bytes,
        contentType,
      );
      if (putResult.isError) {
        return Result.error(
          putResult.error ?? 'Upload gambar gagal',
          code: putResult.errorCode,
          statusCode: putResult.statusCode,
        );
      }

      final mediaEntity = MediaEntity(
        id: DateTime.now().millisecondsSinceEpoch.toString(),
        originalUrl: presign.readUrl,
        type: MediaType.image,
        blurhash: blurhash,
        createdAt: DateTime.now(),
      );
      return Result.success(mediaEntity);
    } catch (e) {
      return Result.error('Image upload error');
    }
  }

  /// Upload image to S3 (legacy method — returns URL only, canonical read_url).
  Future<Result<String>> uploadImage(File imageFile, {required String folder}) async {
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

      final presignResult = await _requestMediaPresignURL(contentType, folder);
      if (presignResult.isError) {
        return Result.error(
          presignResult.error ?? 'Gagal mendapatkan URL upload gambar',
          code: presignResult.errorCode,
          statusCode: presignResult.statusCode,
          details: presignResult.errorDetails,
        );
      }
      final presign = presignResult.data!;
      final bytes = await imageFile.readAsBytes();
      final putResult = await _putToPresignedUrl(
        presign.uploadUrl,
        bytes,
        contentType,
      );
      if (putResult.isError) {
        return Result.error(
          putResult.error ?? 'Upload gambar gagal',
          code: putResult.errorCode,
          statusCode: putResult.statusCode,
        );
      }

      await _generateAndCacheBlurhash(bytes, presign.readUrl);
      return Result.success(presign.readUrl);
    } catch (e) {
      return Result.error('Image upload error');
    }
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

      final folder = _namespaceFolderForKey(key);
      final presignResult = await _requestMediaPresignURL(
        contentType,
        folder,
        storageKey: key,
      );
      if (presignResult.isError) {
        return Result.error(
          presignResult.error ?? 'Gagal mendapatkan URL upload $mediaLabel',
          code: presignResult.errorCode,
          statusCode: presignResult.statusCode,
          details: presignResult.errorDetails,
        );
      }
      final presign = presignResult.data!;
      final bytes = await imageFile.readAsBytes();
      final putResult = await _putToPresignedUrl(
        presign.uploadUrl,
        bytes,
        contentType,
      );
      if (putResult.isError) {
        return Result.error(
          putResult.error ?? 'Upload $mediaLabel gagal',
          code: putResult.errorCode,
          statusCode: putResult.statusCode,
        );
      }

      return Result.success(
        S3UploadResult(
          key: presign.storageKey,
          url: presign.readUrl,
        ),
      );
    } catch (e) {
      return Result.error('Upload $mediaLabel error');
    }
  }

  /// Upload image bytes to a canonical fixed storage key (e.g. cropped avatar bytes).
  ///
  /// Honors the supplied [key] via the same fixed-key backend contract as
  /// [uploadImageWithFixedKey]. Used by [AvatarImageProcessor] for Uint8List payloads.
  Future<Result<S3UploadResult>> uploadImageBytesWithFixedKey(
    Uint8List imageBytes,
    String key, {
    String contentType = 'image/png',
  }) async {
    try {
      final folder = _namespaceFolderForKey(key);
      final presignResult = await _requestMediaPresignURL(
        contentType,
        folder,
        storageKey: key,
      );
      if (presignResult.isError) {
        return Result.error(
          presignResult.error ?? 'Gagal mendapatkan URL upload gambar',
          code: presignResult.errorCode,
          statusCode: presignResult.statusCode,
          details: presignResult.errorDetails,
        );
      }
      final presign = presignResult.data!;
      final putResult = await _putToPresignedUrl(
        presign.uploadUrl,
        imageBytes,
        contentType,
      );
      if (putResult.isError) {
        return Result.error(
          putResult.error ?? 'Upload gambar gagal',
          code: putResult.errorCode,
          statusCode: putResult.statusCode,
        );
      }
      return Result.success(
        S3UploadResult(key: presign.storageKey, url: presign.readUrl),
      );
    } catch (e) {
      return Result.error('Image upload error');
    }
  }

  // ──────────────────────────────────────────────────────────────────────────
  // Helpers
  // ──────────────────────────────────────────────────────────────────────────

  /// First two segments of a storage key (e.g. `images/avatars`).
  /// Fixed-key uploads hint their namespace; the key itself is the authority.
  static String _namespaceFolderForKey(String key) {
    final segments = key.split('/').where((s) => s.isNotEmpty).toList();
    if (segments.length >= 2) return '${segments[0]}/${segments[1]}';
    if (segments.length == 1) return segments[0];
    return 'images/content';
  }

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

/// Result of the canonical chat media register+upload step: the asset the
/// message must reference, plus the read URL for the local preview/optimism.
class ChatMediaUpload {
  final String assetId;
  final String readUrl;

  const ChatMediaUpload({required this.assetId, required this.readUrl});
}

/// Internal result of a backend media presign request.
class _MediaPresignResult {
  final String uploadUrl;
  final String storageKey;
  final String readUrl;

  const _MediaPresignResult({
    required this.uploadUrl,
    required this.storageKey,
    required this.readUrl,
  });
}
