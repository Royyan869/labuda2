import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/social/comment/domain/repositories/comment_repository.dart';

/// Validate Comment Content Use Case
///
/// **DOMAIN:** Social - Comment domain
/// **RESPONSIBILITY:** Validate comment content before creation
/// **BOUNDARY:** Encapsulates all comment validation business rules
class ValidateCommentContentUseCase {
  final CommentRepository _repository;

  ValidateCommentContentUseCase(this._repository);

  /// Execute the use case
  ///
  /// Validates comment content according to business rules:
  /// - Content must not be empty OR media must be provided
  /// - Content must pass validation rules (profanity, length, etc.)
  Future<Result<void>> call({
    required String content,
    required List<String> mediaUrls,
  }) async {
    // Business Rule: Comment must have content OR media
    if (content.trim().isEmpty && mediaUrls.isEmpty) {
      return Result.error('Comment must have content or media');
    }

    // Business Rule: If content is provided, validate it
    if (content.trim().isNotEmpty) {
      final validationResult = await _repository.validateContent(content);
      if (validationResult.isError) {
        return Result.error(
          validationResult.error ?? 'Content validation failed',
        );
      }
    }

    return Result.success(null);
  }
}
