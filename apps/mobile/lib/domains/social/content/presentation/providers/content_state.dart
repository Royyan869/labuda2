// Content State
// Application layer state untuk Riverpod Notifier

import 'package:freezed_annotation/freezed_annotation.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/domains/social/content/domain/repositories/content_repository.dart';

part 'content_state.freezed.dart';

// FEED OWNERSHIP LOCK (BATCH C2):
// ContentState.feed removed - Content domain does not provide social timeline.
// For social timeline feed, use Home/Feed domain's feedProvider.
// For content listing, use ContentState.list with specific methods.

/// Base state untuk Content operations
@freezed
class ContentState with _$ContentState {
  const factory ContentState.initial() = ContentInitial;
  const factory ContentState.loading() = ContentLoading;
  const factory ContentState.data(Content content) = ContentData;
  const factory ContentState.list(List<Content> contents) = ContentList;
  const factory ContentState.search(ContentSearchResult search) = ContentSearch;
  const factory ContentState.error(String message) = ContentError;
}

/// Specific states untuk different operations
@freezed
class ContentDetailState with _$ContentDetailState {
  const factory ContentDetailState.initial() = ContentDetailInitial;
  const factory ContentDetailState.loading() = ContentDetailLoading;
  const factory ContentDetailState.loaded(Content content) =
      ContentDetailLoaded;
  const factory ContentDetailState.error(String message) = ContentDetailError;
}

@freezed
class ContentListState with _$ContentListState {
  const factory ContentListState.initial() = ContentListInitial;
  const factory ContentListState.loading() = ContentListLoading;
  const factory ContentListState.loaded(
    List<Content> contents, {
    int? total,
    @Default(false) bool hasMore,
  }) = ContentListLoaded;
  const factory ContentListState.error(String message) = ContentListError;
}

@freezed
class ContentFormState with _$ContentFormState {
  const factory ContentFormState.initial() = ContentFormInitial;
  const factory ContentFormState.creating() = ContentFormCreating;
  const factory ContentFormState.updating() = ContentFormUpdating;
  const factory ContentFormState.success(Content content) = ContentFormSuccess;
  const factory ContentFormState.error(String message) = ContentFormError;
}

// FeedState removed (BATCH C2): Content domain no longer provides feed state.
// For social timeline, use Home/Feed domain's FeedState from feed_state.dart.

@freezed
class SearchState with _$SearchState {
  const factory SearchState.initial() = SearchInitial;
  const factory SearchState.searching() = SearchSearching;
  const factory SearchState.results(
    List<Content> contents, {
    int? total,
    String? query,
  }) = SearchResults;
  const factory SearchState.error(String message) = SearchError;
}
