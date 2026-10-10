// Share Repository API Implementation
// Implements ShareRepository using Go Backend API

import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/social/share/data/datasources/share_api_datasource.dart';
import 'package:hishumi/domains/social/share/data/remote/native_share_service.dart';
import 'package:hishumi/domains/social/share/domain/domain.dart';

/// Share Repository API Implementation
///
/// Menggunakan ShareApiDatasource untuk Go Backend API operations.
/// Native share tetap menggunakan NativeShareService (OS-level).
class ShareRepositoryApi implements ShareRepository {
  final ShareApiDatasource _datasource;
  final NativeShareService _nativeShareService;

  ShareRepositoryApi({
    required ShareApiDatasource datasource,
    required NativeShareService nativeShareService,
  }) : _datasource = datasource,
       _nativeShareService = nativeShareService;

  // ==========================================================================
  // Error Handling Helper
  // ==========================================================================

  String _mapApiError(Object error) {
    if (error is ApiException) {
      return error.message;
    }
    return 'Failed to complete share operation: $error';
  }

  // ==========================================================================
  // Repository Interface Implementation
  // ==========================================================================

  @override
  Future<Result<ShareResult>> shareViaExternal({
    required ShareTarget target,
    required ShareDestinationType destination,
  }) async {
    // Validate destination
    if (_isInternalDestination(destination)) {
      return Result.error('Use internal share methods for $destination');
    }

    // Route to appropriate native share method
    final result = await _routeToNativeService(target, destination);

    return result.fold(
      (error) => Result.error(error),
      (success) => success
          ? Result.success(ShareResult.success(destination))
          : Result.success(ShareResult.failure(destination, 'Share failed')),
    );
  }

  @override
  Future<Result<String>> shareAsPost({
    required ShareTarget target,
    required String authorId,
    String? caption,
  }) async {
    // Validation
    if (authorId.trim().isEmpty) {
      return Result.error('Author ID is required');
    }

    if (caption != null && caption.trim().isEmpty) {
      return Result.error('Caption cannot be empty');
    }

    if (caption != null && caption.length > 5000) {
      return Result.error('Caption too long (max 5000)');
    }

    try {
      // SHARE CONTRACT V1: Single canonical path for ALL share types.
      // All shares go through POST /contents/{id}/repost.
      // Content shares: target_type=content (implicit from repost endpoint).
      // Non-content shares: target_type + target_id for backend routing.
      final content = caption?.trim().isEmpty ?? true ? '' : caption!.trim();

      // Map ExternalShareType to backend ShareTargetType
      final backendTargetType = _mapToBackendTargetType(target.type);

      final response = await _datasource.createRepost(
        originalContentId: target.id,
        authorId: authorId,
        caption: content.isEmpty ? null : content,
        originalContentTitle: target.title,
        originalContentImageURL: target.imageUrl,
        targetType: backendTargetType,
        targetId: target.id,
      );

      final postId = response['id'] as String?;
      if (postId == null || postId.isEmpty) {
        return Result.error('Failed to get post ID from response');
      }

      return Result.success(postId);
    } on ApiException catch (e) {
      return Result.error(e.message);
    } catch (e) {
      return Result.error(_mapApiError(e));
    }
  }

  @override
  Future<Result<ShareResult>> sendToChat({
    required ShareTarget target,
    required String recipientUserId,
    String? message,
  }) async {
    // Coming soon - will integrate with chat module
    return Result.success(
      ShareResult.failure(ShareDestinationType.sendToChat, 'Coming soon'),
    );
  }

  // ==========================================================================
  // Private Helpers
  // ==========================================================================

  bool _isInternalDestination(ShareDestinationType destination) {
    return destination == ShareDestinationType.shareToFeed ||
        destination == ShareDestinationType.sendToChat;
  }

  Future<Result<bool>> _routeToNativeService(
    ShareTarget target,
    ShareDestinationType destination,
  ) async {
    switch (destination) {
      case ShareDestinationType.whatsapp:
        return _nativeShareService.shareToWhatsApp(text: target.shareText);

      case ShareDestinationType.telegram:
        return _nativeShareService.shareToTelegram(text: target.shareText);

      case ShareDestinationType.instagram:
        if (target.imageUrl == null) {
          return Result.error('Image required for Instagram');
        }
        return _nativeShareService.shareToInstagramStory(
          imageUrl: target.imageUrl!,
        );

      case ShareDestinationType.copyLink:
        return _nativeShareService.copyToClipboard(text: target.publicShareUrl);

      case ShareDestinationType.email:
        return _nativeShareService.shareViaEmail(
          subject: target.title,
          body: target.shareText,
        );

      case ShareDestinationType.more:
        return _nativeShareService.shareViaDialog(
          text: target.shareText,
          subject: target.title,
        );

      case ShareDestinationType.shareToFeed:
        return Result.error('Use shareAsPost instead');

      case ShareDestinationType.sendToChat:
        return Result.error('Use sendToChat instead');
    }
  }

  /// Map mobile ExternalShareType to backend ShareTargetType.
  String _mapToBackendTargetType(ExternalShareType type) {
    switch (type) {
      case ExternalShareType.post:
      case ExternalShareType.request:
        return 'content';
      case ExternalShareType.forSale:
        return 'for_sale';
      case ExternalShareType.auction:
        return 'auction';
      case ExternalShareType.profile:
        return 'profile';
    }
  }
}
