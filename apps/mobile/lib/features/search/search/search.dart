/// Search Refactor Module
///
/// A clean architecture implementation of the search feature.
/// No Firebase in domain/application, no get_it, no UseCase classes.
/// Uses Riverpod providers for all dependencies (no ServiceLocator).
///
/// Structure:
/// - domain/: Entities + Repository interfaces
/// - data/: DTOs + Mappers + API Service + Repository implementations
/// - presentation/: Riverpod Notifiers + Widgets (UI)
///
/// Migration Status: ✅ MIGRATED to pure Riverpod
/// - Uses core.providers.apiClientProvider
/// - Uses core.providers.loggerServiceProvider
/// - Uses core.providers.navigationHandlerProvider
/// - Uses shared.providers.currentUserIdProvider
library;

// Domain Layer
export 'domain/entities/search_filters.dart' show SearchFilters, SearchSortBy;
export 'domain/entities/search_result.dart'
    show SearchResult, SearchResultType, UnifiedSearchResults;
export 'domain/entities/search_history.dart' show SearchHistory;
export 'domain/entities/user_search.dart' show UserSearch;
export 'domain/repositories/search_repository.dart'
    show SearchRepository, ApiResult, ContentSearchResult, UserSearchResult;
export 'domain/repositories/search_history_repository.dart'
    show SearchHistoryRepository;

// Application Layer
export 'presentation/providers/search_state.dart' show SearchState;
export 'presentation/providers/search_notifier.dart' show SearchNotifier;
export 'presentation/providers/search_history_state.dart'
    show SearchHistoryState;
export 'presentation/providers/search_history_notifier.dart'
    show SearchHistoryNotifier;
// R3.1: Export full providers.dart to include generated providers (part files)
export 'presentation/providers/providers.dart';
// **R2.2 MIGRATED**: Mention providers moved from shared to search domain
export 'presentation/providers/mention_providers.dart'
    show
        mentionUserSearchProvider,
        mentionResolverProvider,
        MentionResolver,
        MentionSearchParams;

// Presentation Layer
export 'presentation/screens/search_screen.dart' show SearchScreen;
export 'presentation/screens/search_results_screen.dart'
    show SearchResultsScreen;
export 'presentation/widgets/global_search_bar.dart' show GlobalSearchBar;
export 'presentation/widgets/search_history_list.dart' show SearchHistoryList;
export 'presentation/widgets/search_result_item.dart' show SearchResultItem;
export 'presentation/widgets/search_result_extra_info.dart'
    show SearchResultExtraInfo;
export 'presentation/widgets/search_suggestions_list.dart'
    show SearchSuggestionsList;
export 'presentation/utils/search_result_type_helper.dart'
    show SearchResultTypeHelper;

// Data Layer (limited exports - for specific consumers)
// SearchApiService is exported for user_search_bottom_sheet.dart usage
// ⚠️ R3.1: Direct API service access from UI layer - consider refactoring to use repository
export 'data/remote/search_api_service.dart' show SearchApiService;
// R3.1: Export search_dto for UserSearchResultDto.toUserSearch() extension
export 'data/dto/search_dto.dart'
    show UserSearchResponseDto, UserSearchResultDto;

// Data Layer (internal - should not be imported outside this module)
// - data/dto/
// - data/mappers/
// - data/*_repository_impl.dart
