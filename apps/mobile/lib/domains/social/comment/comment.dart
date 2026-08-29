/// Comment Module
///
/// Clean Architecture:
/// - **Domain**: Pure business entities and repository interfaces
/// - **Application**: Riverpod Notifiers (UseCase logic moved here)
/// - **Data**: API datasources, DTOs, mappers, repository implementations
/// - **Presentation**: UI widgets (dumb, only render state)

library;

// Domain exports - public API
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
