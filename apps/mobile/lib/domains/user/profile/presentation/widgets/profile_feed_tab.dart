import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/widgets/empty_state.dart';
import 'package:labuda/domains/social/content/content.dart';
import 'package:labuda/features/home/domain/domain.dart'; // R3.1: Import FeedItem from home domain
import 'package:labuda/features/home/presentation/providers/feed_renderers.dart';

/// Feed Tab untuk User Profile dengan real content dan reposts
///
/// CONTRACT ALIGNMENT V1:
/// - Display user's content and reposts
/// - Repost attribution shown for reposted content
/// - Real data dari database
/// - Loading states dan empty states
/// - Proper error handling
class ProfileFeedTab extends ConsumerStatefulWidget {
  final String userId;

  const ProfileFeedTab({super.key, required this.userId});

  @override
  ConsumerState<ProfileFeedTab> createState() => _ProfileFeedTabState();
}

class _ProfileFeedTabState extends ConsumerState<ProfileFeedTab> {
  String _selectedFilter = 'All';
  final List<String> _filterOptions = ['All'];
  bool _isLoading = false;
  bool _isLoadingMore = false;
  String? _loadErrorMessage;
  List<FeedItem> _allFeedItems = [];

  // C3B — cursor pagination state
  String? _nextCursor;
  bool _hasMore = false;

  final ScrollController _scrollController = ScrollController();

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_onScroll);
    _loadUserContent();
  }

  @override
  void didUpdateWidget(ProfileFeedTab oldWidget) {
    super.didUpdateWidget(oldWidget);
    // LOOP CLOSURE PASS V1: Reset filter when navigating to different profile
    if (oldWidget.userId != widget.userId) {
      setState(() {
        _selectedFilter = 'All';
        _allFeedItems = [];
        _nextCursor = null;
        _hasMore = false;
        _loadErrorMessage = null;
      });
      _loadUserContent();
    }
  }

  @override
  void dispose() {
    _scrollController
      ..removeListener(_onScroll)
      ..dispose();
    super.dispose();
  }

  void _onScroll() {
    if (!_scrollController.hasClients) return;
    final threshold = _scrollController.position.maxScrollExtent - 200;
    if (_scrollController.offset >= threshold &&
        _hasMore &&
        !_isLoadingMore &&
        !_isLoading) {
      _loadMore();
    }
  }

  FeedItem _contentToFeedItem(Content content) {
    const feedItemType = FeedItemType.content;

    final additionalData = <String, dynamic>{'status': content.status.name};
    if (content.isRepost) {
      additionalData['isRepost'] = true;
      additionalData['originalAuthorId'] = content.originalAuthorId;
    }
    if (content.resourceProjection != null) {
      additionalData['resourceProjection'] = content.resourceProjection;
    }

    return FeedItem(
      id: content.id,
      content: content.content,
      authorId: content.authorId,
      authorUsername: content.authorUsername,
      authorAvatarUrl: content.authorAvatarUrl,
      type: feedItemType,
      createdAt: content.createdAt,
      media: content.media,
      likes: content.engagement.likeCount,
      comments: content.engagement.commentCount,
      additionalData: additionalData,
      // E6/E9 — propagate author lifecycle so FeedCard can redact degraded
      // author identities on the profile surface.
      authorLifecycle: content.authorLifecycle,
    );
  }

  Future<void> _loadUserContent() async {
    setState(() {
      _isLoading = true;
      _loadErrorMessage = null;
    });

    try {
      final contentRepository = ref.read(contentRepositoryProvider);
      final logger = ref.read(loggerServiceProvider);

      logger.info(
        '📱 ProfileFeedTab: Loading content for userId: ${widget.userId}',
      );

      final result = await contentRepository.getContentsByAuthorPaged(
        widget.userId,
        limit: 20,
      );

      result.fold(
        (error) {
          logger.error('❌ ProfileFeedTab: Failed to load content - $error');
          if (mounted) {
            setState(() {
              _loadErrorMessage = _toUserFacingError(error);
            });
          }
        },
        (page) {
          logger.info(
            '✅ ProfileFeedTab: Loaded ${page.items.length} content items'
            ' (hasMore=${page.hasMore})',
          );
        },
      );

      if (result.isSuccess) {
        final page = result.data!;
        final items = page.items.map(_contentToFeedItem).toList();
        setState(() {
          _allFeedItems = items;
          _nextCursor = page.nextCursor;
          _hasMore = page.hasMore;
        });
      }
    } catch (e) {
      await ref
          .read(loggerServiceProvider)
          .error(
            'ProfileFeedTab unexpected load failure',
            extra: {'error': e.toString()},
          );
      if (mounted) {
        setState(() {
          _loadErrorMessage = _toUserFacingError(e.toString());
        });
      }
    } finally {
      if (mounted) setState(() => _isLoading = false);
    }
  }

  Future<void> _loadMore() async {
    if (_isLoadingMore || !_hasMore || _nextCursor == null) return;
    setState(() => _isLoadingMore = true);

    try {
      final contentRepository = ref.read(contentRepositoryProvider);
      final result = await contentRepository.getContentsByAuthorPaged(
        widget.userId,
        limit: 20,
        cursor: _nextCursor,
      );

      if (!mounted) return;

      result.fold(
        (error) =>
            AppSnackBar.showError(context, 'Failed to load more: $error'),
        (page) {
          final newItems = page.items.map(_contentToFeedItem).toList();
          setState(() {
            _allFeedItems = [..._allFeedItems, ...newItems];
            _nextCursor = page.nextCursor;
            _hasMore = page.hasMore;
          });
        },
      );
    } catch (e) {
      if (mounted) {
        AppSnackBar.showError(context, 'Failed to load more: ${e.toString()}');
      }
    } finally {
      if (mounted) setState(() => _isLoadingMore = false);
    }
  }

  List<FeedItem> get _filteredContent {
    return _allFeedItems;
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    // Use CustomScrollView for NestedScrollView compatibility
    return CustomScrollView(
      controller: _scrollController,
      slivers: [
        // Filter section (content-only)
        SliverToBoxAdapter(
          child: Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
              border: Border(
                bottom: BorderSide(
                  color: isDark
                      ? AppColors.darkGray600
                      : AppColors.neutralGray200,
                ),
              ),
            ),
            child: Row(
              children: [
                Text(
                  'Content (${_filteredContent.length})',
                  style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.w600,
                    color: isDark
                        ? AppColors.neutralWhite
                        : AppColors.neutralGray900,
                  ),
                ),
                const Spacer(),
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 12,
                    vertical: 6,
                  ),
                  decoration: BoxDecoration(
                    color: isDark
                        ? AppColors.darkGray700
                        : AppColors.neutralGray100,
                    borderRadius: BorderRadius.circular(20),
                  ),
                  child: DropdownButtonHideUnderline(
                    child: DropdownButton<String>(
                      value: _selectedFilter,
                      isDense: true,
                      style: TextStyle(
                        fontSize: 14,
                        color: isDark
                            ? AppColors.neutralWhite
                            : AppColors.neutralGray800,
                      ),
                      dropdownColor: isDark
                          ? AppColors.darkGray700
                          : AppColors.neutralWhite,
                      items: _filterOptions.map((String filter) {
                        return DropdownMenuItem<String>(
                          value: filter,
                          child: Text(filter),
                        );
                      }).toList(),
                      onChanged: (String? value) {
                        if (value != null) {
                          setState(() {
                            _selectedFilter = value;
                          });
                        }
                      },
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),

        // Content list
        if (_isLoading)
          const SliverFillRemaining(child: Center(child: LoadingIndicator()))
        else if (_loadErrorMessage != null)
          SliverFillRemaining(child: _buildErrorState(_loadErrorMessage!))
        else if (_filteredContent.isEmpty)
          SliverFillRemaining(child: _buildEmptyState(context, isDark))
        else
          SliverPadding(
            padding: const EdgeInsets.symmetric(vertical: 8),
            sliver: SliverList(
              delegate: SliverChildBuilderDelegate((context, index) {
                final feedItem = _filteredContent[index];
                return Consumer(
                  builder: (context, ref, _) {
                    return FeedCard(item: feedItem);
                  },
                );
              }, childCount: _filteredContent.length),
            ),
          ),
        // Load-more footer: spinner while fetching next page.
        if (_isLoadingMore)
          const SliverToBoxAdapter(
            child: Padding(
              padding: EdgeInsets.symmetric(vertical: 16),
              child: Center(child: LoadingIndicator()),
            ),
          ),
      ],
    );
  }

  // Content feed now stays on the universal card renderer only.

  Widget _buildErrorState(String message) {
    return EmptyState.error(
      title: 'Failed to load content',
      subtitle: message,
      onRetry: () => _loadUserContent(),
    );
  }

  String _toUserFacingError(String error) {
    final trimmed = error.trim();
    if (trimmed.isEmpty) {
      return 'Coba lagi beberapa saat.';
    }

    const safeFragments = [
      'Connection timed out',
      'Cannot reach Labuda server',
      'Network error',
      'Please try again',
      'not found',
      '404',
    ];

    final lower = trimmed.toLowerCase();
    for (final fragment in safeFragments) {
      if (lower.contains(fragment.toLowerCase())) {
        return trimmed;
      }
    }

    return 'Coba lagi beberapa saat.';
  }

  Widget _buildEmptyState(BuildContext context, bool isDark) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.photo_library_outlined,
            size: 64,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray500,
          ),
          const SizedBox(height: 16),
          Text(
            _getEmptyStateTitle(),
            style: TextStyle(
              fontSize: 18,
              fontWeight: FontWeight.w600,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray600,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            _getEmptyStateSubtitle(),
            style: TextStyle(
              fontSize: 14,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
            ),
          ),
        ],
      ),
    );
  }

  String _getEmptyStateTitle() {
    return 'No Content Yet';
  }

  String _getEmptyStateSubtitle() {
    return 'User hasn\'t shared any content';
  }

  // OLD _viewContent removed - navigation handled by FeedCardBuilders
  // OLD like helpers removed - unified content reactions now live elsewhere.
}
