/// C1B2 — Canonical new-chat user-search provider.
///
/// Consumes the existing [SearchApiService.searchUsers] which calls the
/// operational `GET /api/v1/search/users` endpoint.  Results are returned as
/// [UserSearch] domain entities carrying `userId`, `username`, and `avatarUrl`.
///
/// Principal-switch safety: the provider watches [currentUserIdProvider] so it
/// rebuilds when the authenticated principal changes, and self-exclusion uses
/// the live principal.  An in-flight request for principal A that completes
/// after a switch to B is discarded by Riverpod's FutureProvider lifecycle —
/// the old future is not associated with the new provider instance.
///
/// Protected scopes: this provider does NOT touch ProfileEntity, the old
/// /users/search endpoint, or the mention search chain.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/features/search/search/data/dto/search_dto.dart';
import 'package:labuda/features/search/search/domain/entities/user_search.dart';
import 'package:labuda/features/search/search/presentation/providers/providers.dart'
    show searchApiServiceProvider;
import 'package:labuda/features/search/search/data/remote/search_api_service.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart'
    show currentUserIdProvider;

/// Canonical new-chat user-search provider.
///
/// Family key is the raw search query.  The provider:
/// - trims and skips the network call for blank input;
/// - calls [SearchApiService.searchUsers] (canonical /search/users);
/// - maps DTOs to [UserSearch] through the existing mapper extension;
/// - excludes the live current principal by stable ID;
/// - preserves loading / error / empty states.
final newChatUserSearchProvider =
    FutureProvider.family<List<UserSearch>, String>((ref, query) async {
      final trimmed = query.trim();
      if (trimmed.isEmpty) return <UserSearch>[];

      final currentUserId = ref.watch(currentUserIdProvider);
      final apiService = ref.read(searchApiServiceProvider);

      final response = await apiService.searchUsers(
        query: trimmed,
        limit: 20,
        offset: 0,
      );

      final users = <UserSearch>[];
      for (final dto in response.users) {
        final user = dto.toUserSearch();
        if (user.userId != currentUserId) {
          users.add(user);
        }
      }
      return users;
    });
