// Content Notifier
// Application layer - Riverpod Notifier menggantikan UseCase classes

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'content_state.dart';
import 'package:labuda/domains/social/content/data/content_providers.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/domains/social/content/domain/repositories/content_repository.dart';

// Re-export providers from data layer to avoid duplication
// These are now defined in content_providers.dart without sl<>
export 'content_providers.dart';

part 'content_notifier.g.dart';

// ============================================================================
// Content Detail Notifier
// ============================================================================

/// Notifier untuk single content operations
@riverpod
class ContentDetail extends _$ContentDetail {
  @override
  ContentDetailState build() {
    return const ContentDetailState.initial();
  }

  /// Get content by ID
  Future<void> fetchContent(String contentId) async {
    state = const ContentDetailState.loading();
    final repo = ref.read(contentRepositoryProvider);

    final result = await repo.getContentById(contentId);

    result.fold(
      (error) => state = ContentDetailState.error(error),
      (content) => state = ContentDetailState.loaded(content),
    );
  }

  /// Reset state
  void reset() {
    state = const ContentDetailState.initial();
  }
}

// ============================================================================
// Content List Notifier
// ============================================================================

/// Notifier untuk content list operations
@riverpod
class ContentList extends _$ContentList {
  @override
  ContentListState build() {
    return const ContentListState.initial();
  }

  /// Get contents with filters
  Future<void> fetchContents({
    int? limit,
    int? offset,
    String? location,
    ContentStatus? status,
  }) async {
    state = const ContentListState.loading();
    final repo = ref.read(contentRepositoryProvider);

    final result = await repo.getContents(
      limit: limit,
      offset: offset,
      location: location,
      status: status,
    );

    result.fold((error) => state = ContentListState.error(error), (contents) {
      state = ContentListState.loaded(
        contents,
        total: contents.length,
        hasMore: limit != null && contents.length >= limit,
      );
    });
  }

  /// Get contents by author
  Future<void> fetchByAuthor(String authorId, {int? limit, int? offset}) async {
    state = const ContentListState.loading();
    final repo = ref.read(contentRepositoryProvider);

    final result = await repo.getContentsByAuthor(
      authorId,
      limit: limit,
      offset: offset,
    );

    result.fold((error) => state = ContentListState.error(error), (contents) {
      state = ContentListState.loaded(
        contents,
        total: contents.length,
        hasMore: limit != null && contents.length >= limit,
      );
    });
  }

  /// Get trending contents
  Future<void> fetchTrending({int? limit}) async {
    state = const ContentListState.loading();
    final repo = ref.read(contentRepositoryProvider);

    final result = await repo.getTrendingContents(limit: limit);

    result.fold((error) => state = ContentListState.error(error), (contents) {
      state = ContentListState.loaded(
        contents,
        total: contents.length,
        hasMore: false,
      );
    });
  }

  /// Get contents by location
  Future<void> fetchByLocation(String location, {int? limit}) async {
    state = const ContentListState.loading();
    final repo = ref.read(contentRepositoryProvider);

    final result = await repo.getContentsByLocation(
      location: location,
      limit: limit,
    );

    result.fold((error) => state = ContentListState.error(error), (contents) {
      state = ContentListState.loaded(
        contents,
        total: contents.length,
        hasMore: false,
      );
    });
  }

  /// Reset state
  void reset() {
    state = const ContentListState.initial();
  }
}

// ============================================================================
// Content Actions Notifier
// ============================================================================

/// Notifier untuk content actions (create, update, delete)
@riverpod
class ContentActions extends _$ContentActions {
  @override
  ContentFormState build() {
    return const ContentFormState.initial();
  }

  /// Create new content
  Future<ContentRepositoryResult<Content>> createContent({
    required String authorId,
    String? authorUsername,
    String? authorAvatarUrl,
    required String content,
    List<MediaEntity> media = const [],
    List<String> tags = const [],
    List<String> taggedUsers = const [],
    ContentSettings settings = const ContentSettings(),
    ContentLocation? location,
  }) async {
    state = const ContentFormState.creating();
    final repo = ref.read(contentRepositoryProvider);

    final result = await repo.createContent(
      authorId: authorId,
      authorUsername: authorUsername,
      authorAvatarUrl: authorAvatarUrl,
      content: content,
      media: media,
      tags: tags,
      taggedUsers: taggedUsers,
      settings: settings,
      location: location,
    );

    result.fold(
      (error) => state = ContentFormState.error(error),
      (data) => state = ContentFormState.success(data),
    );

    return result;
  }

  /// Update content
  Future<ContentRepositoryResult<Content>> updateContent(
    String contentId,
    Content content,
  ) async {
    state = const ContentFormState.updating();
    final repo = ref.read(contentRepositoryProvider);

    final result = await repo.updateContent(contentId, content);

    result.fold(
      (error) => state = ContentFormState.error(error),
      (data) => state = ContentFormState.success(data),
    );

    return result;
  }

  /// Delete content
  Future<ContentRepositoryResult<void>> deleteContent(String contentId) async {
    final repo = ref.read(contentRepositoryProvider);
    return await repo.deleteContent(contentId);
  }

  // Note: View tracking is handled by backend automatically via GET /content/contents/:id
  // No explicit incrementViewCount needed

  /// Reset state
  void reset() {
    state = const ContentFormState.initial();
  }
}

// ============================================================================
// Feed Notifier REMOVED (BATCH C2)
// ============================================================================
// Content domain no longer provides Feed notifier.
// For social timeline feed, use Home/Feed domain's feedProvider from:
// apps/mobile/lib/features/home/presentation/providers/feed/feed_notifier.dart

// ============================================================================
// Search Notifier
// ============================================================================

/// Notifier untuk search operations
@riverpod
class Search extends _$Search {
  @override
  SearchState build() {
    return const SearchState.initial();
  }

  /// Search contents
  Future<void> search(
    String query, {
    int? limit,
    int? offset,
    String? location,
  }) async {
    state = const SearchState.searching();
    final repo = ref.read(contentRepositoryProvider);

    final result = await repo.searchContents(
      query: query,
      limit: limit,
      offset: offset,
      location: location,
    );

    result.fold((error) => state = SearchState.error(error), (search) {
      state = SearchState.results(
        search.contents,
        total: search.total,
        query: search.query,
      );
    });
  }

  /// Reset state
  void reset() {
    state = const SearchState.initial();
  }
}
