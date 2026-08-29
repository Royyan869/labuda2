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

  /// Execute the use case.
  ///
  /// Validates comment content: must not be empty, must pass length check.
  Future<Result<void>> call({
    required String content,
  }) async {
    if (content.trim().isEmpty) {
      return Result.error('Comment must have content');
    }

    final validationResult = await _repository.validateContent(content);
    if (validationResult.isError) {
      return Result.error(
        validationResult.error ?? 'Content validation failed',
      );
    }

    return Result.success(null);
  }
}
