// Content Refactor Providers
// Presentation layer - Riverpod DI exports

// Export all application layer providers
export 'package:labuda/domains/social/content/presentation/providers/content_notifier.dart';

// Export state classes (hide ContentList to avoid conflict with Notifier)
// FEED OWNERSHIP LOCK (BATCH C2): FeedState removed - Content domain no longer provides feed
export 'package:labuda/domains/social/content/presentation/providers/content_state.dart'
    show
        ContentState,
        ContentDetailState,
        ContentListState,
        ContentFormState,
        SearchState;

// Re-export domain entities for convenience
export 'package:labuda/domains/social/content/domain/entities/content.dart';

// Re-export repository interface
export 'package:labuda/domains/social/content/domain/repositories/content_repository.dart';
