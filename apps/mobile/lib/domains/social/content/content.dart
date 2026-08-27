// Content Refactor Module
// Main barrel file for content_refactor feature

// Domain Layer
export 'domain/entities/content.dart';
export 'domain/repositories/content_repository.dart';

// Data Layer - Providers (exported for cross-feature use)
// Exported via data/content_providers.dart - the clean Riverpod providers
export 'data/content_providers.dart';

// Presentation Layer - Notifier (main entry point for providers)
export 'presentation/providers/content_notifier.dart';

// Presentation Layer - State (hide ContentList to avoid conflict with Notifier)
// FEED OWNERSHIP LOCK (BATCH C2): FeedState removed - Content domain no longer provides feed
export 'presentation/providers/content_state.dart'
    show
        ContentState,
        ContentDetailState,
        ContentListState,
        ContentFormState,
        SearchState;

// Presentation Layer - Screens
export 'presentation/screens/content_detail_screen.dart'
    show ContentDetailScreen;
export 'presentation/screens/create_content_screen.dart'
    show CreateContentScreen;
