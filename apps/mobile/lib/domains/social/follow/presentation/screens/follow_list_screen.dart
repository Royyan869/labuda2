import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/widgets/empty_state.dart';
import 'package:hishumi/shared/widgets/loading_indicator.dart';
import 'package:hishumi/shared/widgets/page_error_state.dart';
import '../../domain/entities/follow_entity.dart';
import '../providers/follow_stream_provider.dart';
import '../widgets/user_card.dart';

/// Follow List Screen - Display followers atau following list
///
/// Features:
/// - Tampilkan list followers atau following
/// - Search users
/// - Follow/Unfollow button per user
/// - Refresh data
/// - Empty state
///
/// Size: <300 lines (GUIDELINES compliant)
class FollowListScreen extends ConsumerStatefulWidget {
  final String userId;
  final FollowListType type;
  final String? username; // For title display

  const FollowListScreen({
    super.key,
    required this.userId,
    required this.type,
    this.username,
  });

  @override
  ConsumerState<FollowListScreen> createState() => _FollowListScreenState();
}

class _FollowListScreenState extends ConsumerState<FollowListScreen> {
  final _searchController = TextEditingController();
  String _searchQuery = '';

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  /// Single canonical reload: initial load, pull-to-refresh, and every
  /// retry are this one operation. Failure is never rethrown or rendered
  /// raw — it stays in the provider state and renders as [PageErrorState]
  /// (no data yet) or the inline refresh banner (existing data preserved).
  Future<void> _reload() async {
    final pending = widget.type == FollowListType.followers
        ? followersStreamProvider(widget.userId).future
        : followingStreamProvider(widget.userId).future;
    try {
      // The reloaded collection reaches the screen through the provider
      // state, not through this future — `.then((_) {})` adapts it to
      // `Future<void>` so the `unused_result` contract is satisfied while
      // failures still propagate to the `catch` below.
      await ref.refresh(pending).then((_) {});
    } catch (_) {
      // No-op: the failure remains observable via async.hasError with the
      // last-known-good collection preserved in async.value.
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final authState = ref.watch(authControllerProvider);

    // Canonical list authority: the per-relationship stream providers.
    // `value` is the last-known-good collection — present once the first
    // emission settles, including during a refresh and after a failed
    // refresh, where the previous value is kept.
    final usersAsync = widget.type == FollowListType.followers
        ? ref.watch(followersStreamProvider(widget.userId))
        : ref.watch(followingStreamProvider(widget.userId));
    final users = usersAsync.value ?? const <FollowableUser>[];

    return Scaffold(
      appBar: AppBar(
        title: Text(
          widget.type == FollowListType.followers
              ? '${widget.username ?? 'User'}\'s Followers'
              : '${widget.username ?? 'User'}\'s Following',
        ),
        bottom: PreferredSize(
          preferredSize: const Size.fromHeight(60),
          child: Padding(
            padding: const EdgeInsets.fromLTRB(
              AppMetrics.p16,
              AppMetrics.p0,
              AppMetrics.p16,
              AppMetrics.p8,
            ),
            child: TextField(
              controller: _searchController,
              decoration:
                  AppTheme.searchDecoration(
                    scheme,
                    hintText: 'Search users...',
                  ).copyWith(
                    prefixIcon: const Icon(Icons.search),
                    suffixIcon: _searchQuery.isNotEmpty
                        ? IconButton(
                            icon: const Icon(
                              Icons.clear,
                              semanticLabel: 'Bersihkan',
                            ),
                            onPressed: () {
                              _searchController.clear();
                              setState(() => _searchQuery = '');
                            },
                          )
                        : null,
                  ),
              onChanged: (value) {
                setState(() => _searchQuery = value);
              },
            ),
          ),
        ),
      ),
      // LOADING FOUNDATION (owner-locked):
      // - No collection yet → first-load states only: LoadingIndicator,
      //   PageErrorState, or EmptyState.
      // - Collection present → it stays visible during refresh; the update
      //   indicator and refresh failure render inline, never as full-page
      //   loading/error.
      // SAFE-AREA-29 — the ONE canonical bottom system-window authority
      // on this standalone (shell-less) route: the body SafeArea wraps
      // the RefreshIndicator/CustomScrollView so every branch (loading,
      // error, empty, populated) ends at the system-region start at
      // every inset.
      body: SafeArea(
        child: RefreshIndicator(
          onRefresh: _reload,
          child: CustomScrollView(
            physics: const AlwaysScrollableScrollPhysics(),
            slivers: [
              if (usersAsync.isLoading && users.isEmpty)
                // First request with no data → LoadingIndicator. Never
                // EmptyState (not yet loaded) and never a raw spinner.
                const SliverFillRemaining(
                  hasScrollBody: false,
                  child: Center(child: LoadingIndicator()),
                )
              else if (usersAsync.hasError && users.isEmpty)
                // CANONICAL page-level load error (PageErrorState): safe
                // localized copy only; the raw provider error never reaches
                // the screen. Retry re-executes the canonical reload.
                SliverFillRemaining(
                  hasScrollBody: false,
                  child: PageErrorState(onRetry: _reload),
                )
              else
                ..._buildCollectionSlivers(
                  context,
                  usersAsync,
                  users,
                  authState,
                ),
            ],
          ),
        ),
      ),
    );
  }

  /// Collection branch: runs only when a settled collection exists
  /// (possibly preserved across a failed refresh). Applies the local search
  /// filter, then renders EmptyState (zero-result success) or the rows with
  /// the inline refresh indicator / refresh-error banner on top.
  List<Widget> _buildCollectionSlivers(
    BuildContext context,
    AsyncValue<List<FollowableUser>> usersAsync,
    List<FollowableUser> users,
    AuthState authState,
  ) {
    final filteredUsers = _searchQuery.isEmpty
        ? users
        : users
              .where(
                (u) => u.username.toLowerCase().contains(
                  _searchQuery.toLowerCase(),
                ),
              )
              .toList();

    if (filteredUsers.isEmpty) {
      return [
        SliverFillRemaining(hasScrollBody: false, child: _buildEmptyState()),
      ];
    }

    return [
      // Refresh with existing data: rows stay, update indication on top.
      if (usersAsync.isLoading)
        const SliverToBoxAdapter(child: LinearProgressIndicator(minHeight: 2)),
      // Refresh failure: rows stay, inline banner with retry that
      // re-executes the canonical reload. Never a full-page error here.
      if (usersAsync.hasError)
        SliverToBoxAdapter(child: _buildRefreshErrorBanner()),
      SliverPadding(
        padding: const EdgeInsets.symmetric(
          horizontal: AppMetrics.p16,
          vertical: AppMetrics.p8,
        ),
        sliver: SliverList(
          delegate: SliverChildBuilderDelegate((context, index) {
            final user = filteredUsers[index];
            return Padding(
              padding: EdgeInsets.only(
                bottom: index == filteredUsers.length - 1 ? 0 : AppMetrics.p8,
              ),
              child: UserCard(
                user: user,
                showFollowButton:
                    authState is AuthStateAuthenticated &&
                    authState.user.id != user.id,
                onTap: () {
                  // Navigate to user profile using NavigationHandler
                  ref
                      .read(navigationHandlerProvider)
                      .navigateToUserProfile(user.id);
                },
              ),
            );
          }, childCount: filteredUsers.length),
        ),
      ),
    ];
  }

  /// Minimum bounded refresh-failure indication: persistent inline banner
  /// with safe localized copy ([pageErrorMessage]) and a retry action that
  /// re-executes the canonical reload. Not a new foundation — composition
  /// of canonical tokens for this screen, matching the Home / Chat / Coin /
  /// Search / Seller Auctions / My For Sales refresh banners.
  Widget _buildRefreshErrorBanner() {
    final scheme = Theme.of(context).colorScheme;
    final l10n = context.l10n;
    return Semantics(
      container: true,
      liveRegion: true,
      child: Container(
        width: double.infinity,
        margin: const EdgeInsets.fromLTRB(
          AppMetrics.p16,
          AppMetrics.p12,
          AppMetrics.p16,
          AppMetrics.p4,
        ),
        padding: const EdgeInsets.symmetric(
          horizontal: AppMetrics.p12,
          vertical: AppMetrics.p8,
        ),
        decoration: BoxDecoration(
          color: scheme.errorContainer,
          borderRadius: BorderRadius.circular(AppShape.r12),
          border: Border.all(color: scheme.error),
        ),
        child: Row(
          children: [
            Icon(
              Icons.refresh_outlined,
              size: AppIconSize.action,
              color: scheme.onErrorContainer,
            ),
            const SizedBox(width: AppMetrics.p8),
            Expanded(
              child: Text(
                l10n.pageErrorMessage,
                style: context.typeRoles.bodyDense.copyWith(
                  color: scheme.onErrorContainer,
                ),
              ),
            ),
            TextButton(onPressed: _reload, child: Text(l10n.retryAction)),
          ],
        ),
      ),
    );
  }

  Widget _buildEmptyState() {
    final l10n = context.l10n;

    // Search empty: the list has members, the active query matched none →
    // one primary action that clears that query.
    if (_searchQuery.isNotEmpty) {
      return EmptyState(
        icon: Icons.search_off,
        title: l10n.emptySearchTitle,
        subtitle: l10n.emptySearchMessage,
        actionLabel: l10n.resetFilterAction,
        onAction: () {
          _searchController.clear();
          setState(() => _searchQuery = '');
        },
      );
    }

    // Collection empty: nobody to show for this relationship type.
    return EmptyState(
      icon: Icons.people_outline,
      title: widget.type == FollowListType.followers
          ? l10n.emptyFollowersTitle
          : l10n.emptyFollowingTitle,
    );
  }
}

/// Follow List Type
enum FollowListType { followers, following }
