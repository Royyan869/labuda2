/// Comment Module
///
/// Refactored Comment module following Clean Architecture principles.
///
/// ## Architecture
/// - **Domain**: Pure business entities and repository interfaces
/// - **Application**: Riverpod Notifiers (UseCase logic moved here)
/// - **Data**: API datasources, DTOs, mappers, repository implementations
/// - **Presentation**: UI widgets (dumb, only render state)
///
/// ## Key Changes
/// - No UseCase classes (logic in CommentNotifier)
/// - Riverpod for internal DI (no get_it for module internal)
/// - Firebase/HTTP isolated to data layer
/// - Clear separation of concerns
///
/// ## Usage
///
/// ```dart
/// // Watch comments state
/// final state = ref.watch(commentNotifierProvider);
///
/// // Load comments
/// ref.read(commentNotifierProvider.notifier).loadComments(
///   targetId: postId,
///   targetType: CommentTargetType.post,
/// );
///
/// // Create comment
/// await ref.read(commentNotifierProvider.notifier).createComment(
///   targetId: postId,
///   targetType: CommentTargetType.post,
///   content: 'Great post!',
/// );
/// ```

library;

// Domain exports - public API
// Note: CommentAttachment types NOT exported to avoid conflict with shared/attachment
// Code should use shared/attachment/attachment.dart for CommentAttachment types
export 'domain/entities/comment.dart' show Comment, CommentPage, CommentTargetType;
export 'domain/repositories/comment_repository.dart';

// Presentation exports - public API (State, Notifier, Providers)
export 'presentation/providers/comment_state.dart';
export 'presentation/providers/comment_notifier.dart';
export 'presentation/providers/comment_providers.dart';

// Data exports are internal - not exported
// Use providers from presentation layer instead

// Presentation exports - re-exports from old module for now
export 'presentation/comment_widgets.dart';

// Screens
export 'presentation/screens/discussion_screen.dart';
