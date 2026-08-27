import 'package:labuda/core/common/result.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';
import 'package:labuda/shared/chat/chat_gateway.dart';

/// Get For-Sale Share Reference Use Case
///
/// **DOMAIN:** Chat domain
/// **RESPONSIBILITY:** Get for-sale share references for chat attachments
/// **BOUNDARY:** Uses ChatGateway to avoid direct commerce dependency
class GetForSaleShareReferenceUseCase {
  final ChatGateway _chatGateway;

  GetForSaleShareReferenceUseCase(this._chatGateway);

  /// Get share reference for a for-sale item
  ///
  /// Returns ShareReference that can be attached to chat messages
  Future<Result<ShareReference>> call(String forSaleId) async {
    try {
      final shareReference = await _chatGateway.getForSaleShareReference(
        forSaleId,
      );

      if (shareReference != null) {
        return Result.success(shareReference);
      } else {
        return Result.error('Failed to get for-sale share reference');
      }
    } catch (e) {
      return Result.error('Failed to get for-sale share reference: $e');
    }
  }
}
