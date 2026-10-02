// Dart
// Flutter
import 'package:flutter/material.dart';

// External
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

// Internal
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/chat/chat/chat.dart';
import 'package:labuda/domains/social/follow/follow.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart'
    show ProfileEntity;
import 'package:labuda/domains/user/profile/presentation/providers/profile_about_provider.dart'
    show normalizeProfileLocation;
import 'package:labuda/domains/user/profile/presentation/providers/profile_view_provider.dart';
import 'package:labuda/domains/user/profile/presentation/providers/user_data_provider.dart';
import 'package:labuda/domains/user/profile/presentation/screens/settings_screen.dart';
import 'package:labuda/domains/user/profile/presentation/screens/unified_edit_profile_screen.dart';
import 'package:labuda/domains/user/profile/presentation/utils/profile_lifecycle_redaction.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/profile_actions.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/models/seller_identity_data.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/profile_cover.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/profile_feed_tab.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/profile_reviews_tab.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/profile_stats.dart';
import 'package:labuda/domains/user/profile/presentation/screens/profile_screen/profile_about_tab.dart';
import 'package:labuda/domains/system/report/domain/entities/entities.dart';
import 'package:labuda/domains/system/report/presentation/screens/report_screen.dart';
import 'package:labuda/domains/user/preference/seller/seller.dart';
import 'package:labuda/domains/social/share/share.dart';
import 'package:labuda/domains/user/profile/presentation/screens/profile_screen/profile_share_builder.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/governance/seller_tier_badge.dart';
import 'package:labuda/shared/providers/block_state_provider.dart';
import 'package:labuda/shared/widgets/block_confirmation_dialog.dart';

/// Profile Screen dengan Facebook-style collapsing header
///
/// Features:
/// - Cover photo sebagai background (FB style)
/// - Avatar overlap di bawah cover
/// - Collapsing animation: avatar + nama pindah ke AppBar saat scroll
/// - Action button icons di AppBar saat collapsed
class ProfileScreen extends ConsumerStatefulWidget {
  final String userId;
  final String? username;

  const ProfileScreen({super.key, required this.userId, this.username});

  @override
  ConsumerState<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends ConsumerState<ProfileScreen>
    with TickerProviderStateMixin {
  late TabController _tabController;
  late TabController _subTabController;
  late ScrollController _scrollController;

  int _currentTabLength = 3;
  int _selectedMainTab = 0;

  // Scroll tracking for collapsing header
  double _scrollOffset = 0;
  static const double _headerCollapsedHeight = 60.0;
  static const double _coverPhotoHeight = 160.0;
  static const double _avatarSize = 96.0;

  // Avatar sizes for flying animation
  static const double _avatarCollapsedSize = 40.0;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 3, vsync: this);
    _tabController.addListener(_onMainTabChanged);
    _subTabController = TabController(length: 2, vsync: this);
    _scrollController = ScrollController();
    _scrollController.addListener(_onScroll);
    // LOOP CLOSURE PASS V1: Pre-load follow status when entering profile
    _preloadFollowStatus();
  }

  /// Pre-load follow status for smoother UX
  /// Prevents flickering of follow button when navigating between profiles
  void _preloadFollowStatus() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final authState = ref.read(authControllerProvider);
      if (authState is! AuthStateAuthenticated) return;

      final currentUserId = authState.user.id;
      final actualUserId = _getActualUserId(authState);

      // Don't check follow status for own profile
      if (actualUserId == currentUserId) return;

      // Check if status is already loaded
      final followState = ref.read(followStatusProvider);
      if (!followState.followStatusMap.containsKey(actualUserId)) {
        // Pre-load follow status to prevent UI flicker
        ref
            .read(followStatusProvider.notifier)
            .checkFollowStatus(
              followerId: currentUserId,
              followingId: actualUserId,
            );
      }
    });
  }

  void _onScroll() {
    setState(() {
      _scrollOffset = _scrollController.offset;
    });
  }

  void _onMainTabChanged() {
    if (_tabController.indexIsChanging) return;
    setState(() {
      _selectedMainTab = _tabController.index;
    });
  }

  @override
  void dispose() {
    _tabController.removeListener(_onMainTabChanged);
    _tabController.dispose();
    _subTabController.dispose();
    _scrollController.removeListener(_onScroll);
    _scrollController.dispose();
    super.dispose();
  }

  void _updateTabController(int newLength) {
    if (_currentTabLength != newLength && mounted) {
      final oldIndex = _tabController.index;
      final newIndex = oldIndex < newLength ? oldIndex : 0;

      _tabController.removeListener(_onMainTabChanged);
      _tabController.dispose();

      _currentTabLength = newLength;
      _tabController = TabController(
        length: newLength,
        vsync: this,
        initialIndex: newIndex,
      );
      _tabController.addListener(_onMainTabChanged);
      _selectedMainTab = newIndex;

      if (mounted) setState(() {});
    }
  }

  // Calculate collapse progress (0.0 = expanded, 1.0 = collapsed)
  double get _collapseProgress {
    // Extend range to slow down avatar animation (+60px more scroll)
    const collapseStart = (_coverPhotoHeight - _headerCollapsedHeight) + 60;
    if (_scrollOffset <= 0) return 0.0;
    if (_scrollOffset >= collapseStart) return 1.0;
    return _scrollOffset / collapseStart;
  }

  bool _isShowcaseTabSelected(bool isSeller) =>
      isSeller && _selectedMainTab == 0;

  int _getTabCount(bool isSeller) => isSeller ? 4 : 3;

  List<Widget> _getTabs(bool isSeller) {
    if (isSeller) {
      return const [
        Tab(text: 'Showcase'),
        Tab(text: 'Feed'),
        Tab(text: 'Reviews'),
        Tab(text: 'About'),
      ];
    }
    return const [Tab(text: 'Feed'), Tab(text: 'Reviews'), Tab(text: 'About')];
  }

  List<Widget> _getTabViews(bool isSeller, String userId, AuthState authState) {
    final isOwnProfile = _isOwnProfile(authState);
    if (isSeller) {
      return [
        ProfileStoreTab(userId: userId, subTabController: _subTabController),
        ProfileFeedTab(userId: userId),
        ProfileReviewsTab(userId: userId, isSeller: true),
        ProfileAboutTab(userId: userId, isOwnProfile: isOwnProfile),
      ];
    }
    return [
      ProfileFeedTab(userId: userId),
      ProfileReviewsTab(userId: userId, isSeller: false),
      ProfileAboutTab(userId: userId, isOwnProfile: isOwnProfile),
    ];
  }

  String _getUserDisplayName(AuthState authState) {
    if (_isOwnProfile(authState) && authState is AuthStateAuthenticated) {
      return '@${authState.user.username}';
    }
    final userDataAsync = ref.watch(userDataProvider(widget.userId));
    if (userDataAsync.hasValue && userDataAsync.value != null) {
      return '@${userDataAsync.value!.username}';
    }
    return userDataAsync.isLoading ? 'Loading...' : 'User Profile';
  }

  String _getActualUserId(AuthState authState) {
    if (widget.userId == 'current_user' &&
        authState is AuthStateAuthenticated) {
      return authState.user.id;
    }
    return widget.userId;
  }

  bool _isOwnProfile(AuthState authState) {
    if (authState is AuthStateAuthenticated) {
      return _getActualUserId(authState) == authState.user.id;
    }
    return false;
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final authState = ref.watch(authControllerProvider);
    final actualUserId = _getActualUserId(authState);
    final isOwnProfile = _isOwnProfile(authState);

    if (!isOwnProfile) {
      final userDataAsync = ref.watch(userDataProvider(actualUserId));
      return userDataAsync.when(
        loading: () => _buildViewedProfileLoadingState(),
        error: (error, _) =>
            _buildViewedProfileErrorState(userId: actualUserId, error: error),
        data: (user) {
          if (user == null) {
            return _buildViewedProfileUnavailableState();
          }

          final isSeller = user.hasCreatedSellerProfile;
          final profileData = _getProfileData(
            authState,
            actualUserId,
            viewedUser: user,
          );
          final lifecycle =
              (profileData['lifecycle'] as ContentLifecycle?) ??
              ContentLifecycle.active;

          return _buildProfileScaffold(
            context: context,
            scheme: scheme,
            authState: authState,
            actualUserId: actualUserId,
            isOwnProfile: false,
            isSeller: isSeller,
            profileData: profileData,
            lifecycle: lifecycle,
          );
        },
      );
    }

    final sellerIdentityStatus = ref.watch(sellerIdentityStatusProvider);
    final sellerCapabilityStatus = ref.watch(sellerCapabilityStatusProvider);
    final sellerAuthorityUnknown =
        sellerIdentityStatus == SellerIdentityStatus.unknown ||
        sellerCapabilityStatus == SellerCapabilityStatus.unknown;

    if (sellerAuthorityUnknown) {
      return _buildViewedProfileLoadingState();
    }

    final isSeller = sellerIdentityStatus == SellerIdentityStatus.seller;
    final profileData = _getProfileData(authState, actualUserId);
    // E5.2 — Lifecycle drives action-button gating + sensitive-section
    // suppression. Sourced from response.identity.lifecycle (never from raw
    // accountStatus). Defaults to active for own-profile and for any
    // surface that does not (yet) emit the canonical identity card.
    final lifecycle =
        (profileData['lifecycle'] as ContentLifecycle?) ??
        ContentLifecycle.active;

    return _buildProfileScaffold(
      context: context,
      scheme: scheme,
      authState: authState,
      actualUserId: actualUserId,
      isOwnProfile: true,
      isSeller: isSeller,
      profileData: profileData,
      lifecycle: lifecycle,
    );
  }

  Widget _buildProfileScaffold({
    required BuildContext context,
    required ColorScheme scheme,
    required AuthState authState,
    required String actualUserId,
    required bool isOwnProfile,
    required bool isSeller,
    required Map<String, dynamic> profileData,
    required ContentLifecycle lifecycle,
  }) {
    final newTabLength = _getTabCount(isSeller);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _updateTabController(newTabLength);
    });

    // Header height is now fixed - cover photo + avatar overlap space
    // Avatar overlaps into content area, give it some breathing room
    final statusBarHeight = MediaQuery.of(context).padding.top;
    const avatarOverlapSpace = 32.0; // Reduced for tighter spacing
    final headerExpandedHeight =
        _coverPhotoHeight + statusBarHeight + avatarOverlapSpace;

    return Scaffold(
      body: NestedScrollView(
        controller: _scrollController,
        headerSliverBuilder: (context, innerBoxIsScrolled) {
          final isBlocked =
              !isOwnProfile && ref.watch(isUserBlockedProvider(actualUserId));

          return [
            // Custom collapsing AppBar with cover photo
            SliverAppBar(
              expandedHeight: headerExpandedHeight,
              pinned: true,
              leading: _buildBackButton(scheme),
              // Title removed - using FlexibleSpaceBar.title for smooth walk animation
              actions: _buildAppBarActions(
                scheme,
                isOwnProfile,
                actualUserId,
                authState,
                lifecycle,
              ),
              flexibleSpace: _buildExpandedHeader(
                context,
                scheme,
                actualUserId,
                isSeller,
                isOwnProfile,
                profileData,
              ),
            ),

            // Blocked banner
            if (isBlocked)
              SliverToBoxAdapter(
                child: BlockedUserBanner(
                  displayName: _getUserDisplayName(authState),
                  onUnblock: () => _handleUnblockUser(actualUserId),
                ),
              ),

            // Profile info (farm/location/bio) + Stats section
            SliverToBoxAdapter(
              child: _buildProfileInfoAndStats(
                scheme: scheme,
                userId: actualUserId,
                isSeller: isSeller,
                profileData: profileData,
                lifecycle: lifecycle,
              ),
            ),

            // Main TabBar (sticky)
            SliverPersistentHeader(
              pinned: true,
              delegate: _SliverTabBarDelegate(
                TabBar(
                  controller: _tabController,
                  tabs: _getTabs(isSeller),
                ),
                scheme: scheme,
              ),
            ),

            // Sub-tabs for Showcase
            if (_isShowcaseTabSelected(isSeller))
              SliverPersistentHeader(
                pinned: true,
                delegate: _SliverSubTabBarDelegate(
                  TabBar(
                    controller: _subTabController,
                    labelPadding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16),
                    tabs: const [
                      Tab(text: 'Dijual', height: 40),
                      Tab(text: 'Lelang', height: 40),
                    ],
                  ),
                  scheme: scheme,
                ),
              ),
          ];
        },
        body: SafeArea(
          top: false,
          child: TabBarView(
            controller: _tabController,
            children: _getTabViews(isSeller, actualUserId, authState),
          ),
        ),
      ),
    );
  }

  Widget _buildViewedProfileLoadingState() {
    return Scaffold(
      body: SafeArea(
        child: EmptyState.loading(
          title: 'Memuat profil',
          subtitle: 'Mengambil identitas pengguna tujuan',
        ),
      ),
    );
  }

  Widget _buildViewedProfileErrorState({
    required String userId,
    required Object error,
  }) {
    return Scaffold(
      body: SafeArea(
        child: EmptyState.error(
          title: 'Profil belum bisa dimuat',
          subtitle: _sanitizeProfileLoadError(error, userId: userId),
          onRetry: () => ref.invalidate(userDataProvider(userId)),
        ),
      ),
    );
  }

  Widget _buildViewedProfileUnavailableState() {
    return Scaffold(
      body: SafeArea(
        child: EmptyState(
          title: 'Pengguna tidak tersedia',
          subtitle: 'Akun ini tidak ditemukan atau sudah tidak tersedia.',
          icon: Icons.person_off_outlined,
          type: EmptyStateType.noData,
        ),
      ),
    );
  }

  /// Full error detail goes to the logger; the UI only ever receives the
  /// first line of the message, hard-capped. Raw `error.toString()` (which
  /// can embed provider/stack dumps hundreds of thousands of characters
  /// long) must never reach a `Text` inside a non-scrollable Column —
  /// that caused a 146k-pixel RenderFlex overflow.
  String _sanitizeProfileLoadError(Object error, {String? userId}) {
    final raw = error.toString().trim();
    LoggerService.instance.error(
      'ProfileScreen: load error${userId != null ? ' for userId=$userId' : ''}',
      extra: {'error': raw},
    );

    if (raw.isEmpty) {
      return 'Profil belum bisa dimuat. Coba lagi.';
    }

    var message = raw;
    if (message.startsWith('Exception: ')) {
      final stripped = message.substring('Exception: '.length).trim();
      if (stripped.isNotEmpty) {
        message = stripped;
      }
    }

    // First line only — the remainder is stack-trace noise.
    final firstLine = message.split('\n').first.trim();
    if (firstLine.isEmpty) {
      return 'Profil belum bisa dimuat. Coba lagi.';
    }

    const maxUserFacingLength = 200;
    if (firstLine.length <= maxUserFacingLength) {
      return firstLine;
    }
    return '${firstLine.substring(0, maxUserFacingLength)}…';
  }

  Widget _buildBackButton(ColorScheme scheme) {
    return IconButton(
      icon: Container(
        padding: const EdgeInsets.all(AppMetrics.p8),
        decoration: BoxDecoration(
          color: _collapseProgress < 0.5
              ? scheme.shadow.withValues(alpha: 0.3)
              : Colors.transparent,
          shape: BoxShape.circle,
        ),
        child: Icon(
          Icons.arrow_back,
          color: _collapseProgress < 0.5
              ? scheme.onPrimary
              : (scheme.onSurface),
          size: AppIconSize.action,
        ),
      ),
      onPressed: () {
        if (Navigator.of(context).canPop()) Navigator.of(context).pop();
      },
    );
  }

  List<Widget> _buildAppBarActions(
    ColorScheme scheme,
    bool isOwnProfile,
    String userId,
    AuthState authState,
    ContentLifecycle lifecycle,
  ) {
    final iconColor = _collapseProgress < 0.5
        ? scheme.onPrimary
        : (scheme.onSurface);

    final bgColor = _collapseProgress < 0.5
        ? scheme.shadow.withValues(alpha: 0.3)
        : Colors.transparent;

    // E5.2 — Share is target-user action; suppress on degraded. Block/report
    // remain active — degraded identities must still be reportable/blockable.
    final actionsDisabled = profileLifecycleDisablesTargetActions(lifecycle);

    // CANONICAL: AppBar is navigation/overflow only. Primary CTAs
    // (Edit/Share/Follow/Message) live exclusively in ProfileActions
    // below the header (single authority, no duplication).
    if (isOwnProfile) {
      return [
        IconButton(
          icon: Container(
            padding: const EdgeInsets.all(AppMetrics.p8),
            decoration: BoxDecoration(color: bgColor, shape: BoxShape.circle),
            child: Icon(Icons.settings_outlined, color: iconColor, size: AppIconSize.action),
          ),
          onPressed: () => _navigateToSettings(context),
          tooltip: 'Settings',
        ),
      ];
    } else {
      return [
        PopupMoreOptionsButton(
          contentType: PopupMoreOptionsContentType.profile,
          isCreator: false,
          isDeleting: false,
          onShare: actionsDisabled ? null : _handleShareProfile,
          onBlock: _handleBlockUser,
          onReport: _handleReportUser,
          iconColor: iconColor,
        ),
      ];
    }
  }

  Widget _buildExpandedHeader(
    BuildContext context,
    ColorScheme scheme,
    String userId,
    bool isSeller,
    bool isOwnProfile,
    Map<String, dynamic> profileData,
  ) {
    final statusBarHeight = MediaQuery.of(context).padding.top;
    final coverHeight = _coverPhotoHeight + statusBarHeight;

    return Stack(
      fit: StackFit.expand,
      clipBehavior: Clip.hardEdge, // Clip overflow during collapse animation
      children: [
        // Cover photo background
        Positioned(
          top: 0,
          left: 0,
          right: 0,
          height: coverHeight,
          child: ProfileCover(
            coverPhotoUrl: profileData['coverPhotoUrl'],
            height: coverHeight,
            isOwnProfile: isOwnProfile,
            collapseProgress:
                _collapseProgress, // Pass collapse state for smooth fade
          ),
        ),

        // Flying avatar with name & username - animates from expanded to AppBar
        _buildFlyingAvatarWithInfo(
          context,
          scheme,
          profileData,
          isSeller,
          userId,
          coverHeight,
        ),
      ],
    );
  }

  /// Avatar, nama, dan username yang terbang bersama dari expanded ke AppBar
  Widget _buildFlyingAvatarWithInfo(
    BuildContext context,
    ColorScheme scheme,
    Map<String, dynamic> profileData,
    bool isSeller,
    String userId,
    double coverHeight,
  ) {
    final statusBarHeight = MediaQuery.of(context).padding.top;

    // === Avatar Animation ===
    // Start position (expanded): overlapping cover at bottom
    final avatarStartTop = coverHeight - (_avatarSize / 2);
    final avatarStartLeft = 16.0;
    final avatarStartSize = _avatarSize;

    // End position (collapsed): in AppBar after back button
    final avatarEndTop =
        statusBarHeight + (kToolbarHeight - _avatarCollapsedSize) / 2;
    final avatarEndLeft = 56.0;
    final avatarEndSize = _avatarCollapsedSize;

    // Interpolate avatar
    final currentAvatarTop = _lerp(
      avatarStartTop,
      avatarEndTop,
      _collapseProgress,
    );
    final currentAvatarLeft = _lerp(
      avatarStartLeft,
      avatarEndLeft,
      _collapseProgress,
    );
    final currentAvatarSize = _lerp(
      avatarStartSize,
      avatarEndSize,
      _collapseProgress,
    );

    // === Identity Animation ===
    // Start position: beside avatar, vertically centered with avatar
    // Text height (handle + optional farm line) ~36px
    final textHeight = 36.0;
    final textStartTop = avatarStartTop + ((_avatarSize - textHeight) / 2);
    final textStartLeft = 16 + _avatarSize + 12.0; // beside avatar
    final nameStartSize = 18.0;
    final usernameStartSize = 13.0;

    // End position: beside avatar in AppBar
    final textEndTop =
        statusBarHeight +
        (kToolbarHeight - 36) /
            2; // center vertically (name + username height ~36)
    final textEndLeft =
        56.0 + _avatarCollapsedSize + 8.0; // after avatar + spacing
    final nameEndSize = 14.0;
    final usernameEndSize = 11.0;

    // Right constraint: minimal when expanded, space for 3 action buttons when collapsed
    const textStartRight = 16.0; // just padding when expanded
    const textEndRight =
        150.0; // space for 3 action buttons (~48px each + padding)

    // Interpolate text positions
    final currentTextTop = _lerp(textStartTop, textEndTop, _collapseProgress);
    final currentTextLeft = _lerp(
      textStartLeft,
      textEndLeft,
      _collapseProgress,
    );
    final currentTextRight = _lerp(
      textStartRight,
      textEndRight,
      _collapseProgress,
    );
    final currentNameSize = _lerp(
      nameStartSize,
      nameEndSize,
      _collapseProgress,
    );
    final currentUsernameSize = _lerp(
      usernameStartSize,
      usernameEndSize,
      _collapseProgress,
    );

    // The pairing rule lives in ONE place — the identity model. This screen
    // only renders the labels it was handed; it never composes the order.
    final primaryIdentityLine =
        (profileData['identityPrimary'] ?? profileData['name']) as String;
    final secondaryIdentityLine = profileData['identitySecondary'] as String?;

    // Color interpolation for better visibility when collapsed
    // Name: stays high contrast
    // Username: gets darker/more visible when collapsed
    final nameColor = scheme.onSurface;

    // Username color transitions to same as name when collapsed for better visibility
    final secondaryExpandedColor = scheme.onSurfaceVariant;
    final secondaryCollapsedColor = secondaryExpandedColor;
    final secondaryColor = Color.lerp(
      secondaryExpandedColor,
      secondaryCollapsedColor,
      _collapseProgress,
    )!;

    return Stack(
      children: [
        // Flying Avatar
        Positioned(
          top: currentAvatarTop,
          left: currentAvatarLeft,
          child: OnlineBadge(
            userId: userId,
            // hide online indicator when collapsed
            enabled: _collapseProgress < 0.5,
            child: SellerAvatar(
              userId: userId,
              avatarUrl: profileData['avatar'] as String?,
              storeImageUrl: profileData['farmPhotoUrl'] as String?,
              isSeller: isSeller,
              size: currentAvatarSize,
              onTap: () {},
            ),
          ),
        ),

        // Flying identity lines
        Positioned(
          top: currentTextTop,
          left: currentTextLeft,
          right: currentTextRight,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              // Primary identity line — store name when the user has a store,
              // otherwise the handle.
              Text(
                primaryIdentityLine,
                style: TextStyle(
                  fontSize: currentNameSize,
                  fontWeight: FontWeight.bold,
                  color: nameColor,
                ),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
              if (secondaryIdentityLine != null) ...[
                Text(
                  secondaryIdentityLine,
                  style: TextStyle(
                    fontSize: currentUsernameSize,
                    color: secondaryColor,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ],
          ),
        ),
      ],
    );
  }

  double _lerp(double start, double end, double progress) {
    return start + (end - start) * progress;
  }

  /// Profile info (farm/location/bio) + Stats section
  /// Semua konten ini ikut scroll, tidak collapse
  Widget _buildProfileInfoAndStats({
    required ColorScheme scheme,
    required String userId,
    required bool isSeller,
    required Map<String, dynamic> profileData,
    required ContentLifecycle lifecycle,
  }) {
    final authState = ref.watch(authControllerProvider);
    final isOwnProfile = _isOwnProfile(authState);

    return Container(
      color: scheme.surface,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          // Action buttons (Edit/Share for own profile, Follow/Message for others)
          // E5.2 — lifecycle gates target-user actions for non-own profiles.
          Padding(
            padding: const EdgeInsets.fromLTRB(AppMetrics.p16, AppMetrics.p12, AppMetrics.p16, AppMetrics.p12),
            child: ProfileActions(
              userId: userId,
              isOwnProfile: isOwnProfile,
              onEditProfile: () => _navigateToEditProfile(),
              onShare: () => _handleShareProfile(),
              onMessage: () => _handleMessageAction(userId),
              lifecycle: lifecycle,
            ),
          ),

          // Seller reputation tier badge (pro/elite only; null/basic hides
          // inside the widget itself). Ported from the purged duplicate
          // header builder — profile header is a documented SellerTierBadge
          // surface and the data was already prepared but never rendered.
          if (profileData['sellerTier'] != null) ...[
            Padding(
              padding: const EdgeInsets.fromLTRB(AppMetrics.p16, AppMetrics.p0, AppMetrics.p16, AppMetrics.p12),
              child: Align(
                alignment: Alignment.centerLeft,
                child: SellerTierBadge(
                  tier: profileData['sellerTier'] as String?,
                ),
              ),
            ),
          ],

          // Profile info section (location, bio)
          if (_hasProfileInfo(profileData, isSeller))
            Padding(
              padding: const EdgeInsets.fromLTRB(
                AppMetrics.p16,
                AppMetrics.p0,
                AppMetrics.p16,
                AppMetrics.p12,
              ), // Reduce top padding to minimize gap
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  // Location
                  if (profileData['location'] != null &&
                      profileData['location'].toString().isNotEmpty) ...[
                    Row(
                      children: [
                        Icon(
                          Icons.location_on_outlined,
                          size: AppIconSize.inlineGlyph,
color: scheme.onSurfaceVariant,
                        ),
                        const SizedBox(width: 4),
                        Expanded(
                          child: Text(
                            profileData['location'],
                            style: TextStyle(
                              fontSize: AppType.s12,
color: scheme.onSurfaceVariant,
                            ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 8),
                  ],

                  // Bio
                  if (profileData['bio'] != null &&
                      profileData['bio'].toString().trim().isNotEmpty)
                    Text(
                      profileData['bio'],
                      style: TextStyle(
                        fontSize: AppType.s14,
color: scheme.onSurfaceVariant,
                        height: 1.3,
                      ),
                    ),
                ],
              ),
            ),

          // Stats section
          ProfileStats(userId: userId, isSeller: isSeller),
        ],
      ),
    );
  }

  /// Check if profile has any info to display (farm/location/bio)
  bool _hasProfileInfo(Map<String, dynamic> profileData, bool isSeller) {
    final hasFarm =
        isSeller &&
        profileData['farmName'] != null &&
        profileData['farmName'].toString().isNotEmpty;
    final hasLocation =
        profileData['location'] != null &&
        profileData['location'].toString().isNotEmpty;
    final hasBio =
        profileData['bio'] != null &&
        profileData['bio'].toString().trim().isNotEmpty;

    return hasFarm || hasLocation || hasBio;
  }

  void _navigateToEditProfile() {
    // Use centralized provider (TANGGUNG_JAWAB_MODUL compliance)
    final currentUser = ref.read(authenticatedUserProvider);
    if (currentUser == null) return;

    context.push(
      RoutePaths.editProfile,
      extra: UnifiedEditProfileSection.personal,
    );
  }

  // === Data & Navigation Methods ===

  Map<String, dynamic> _getProfileData(
    AuthState authState,
    String userId, {
    AuthUser? viewedUser,
  }) {
    final isOwnProfile = _isOwnProfile(authState);

    // ONE AUTHORITY FOR THIS PAGE: a single GET supplies cover + location for
    // both own and viewed profiles. The profile stream (30s polling) and the
    // client-side address composition are gone — the backend owns the public
    // origin line, so cover and location render on the first frame.
    final profileView = ref.watch(profileViewDataProvider(userId)).value;
    final coverPhotoUrl = profileView?.profile.coverPhotoUrl;
    final location = _publicLocation(profileView?.profile);

    if (authState is AuthStateAuthenticated) {
      if (isOwnProfile) {
        final user = authState.user;
        // ONE identity rule: the pairing order (store name primary, handle
        // secondary) is implemented once, in the identity model.
        final identity = SellerIdentityData(
          userId: user.id,
          username: user.username,
          storeName: user.storeName,
        );
        // E5.2 — Own profile is never degraded from the viewer's own POV;
        // the canonical lifecycle source is the public profile fetch
        // (/users/:id), not /users/me. Force active here so the header
        // renders normally even if /users/me happens to surface a lifecycle
        // hint in the future.
        return {
          'name': '@${user.username}',
          // Store identity travels with the identity payload itself: the
          // hydrated session snapshot for own profile, the public projection
          // for a viewed user. The profile stream is not an identity source.
          'farmName': user.storeName,
          'identityPrimary': identity.primaryLabel ?? '@${user.username}',
          'identitySecondary': identity.secondaryLabel,
          'username': '@${user.username}',
          'avatar': user.avatarUrl,
          'bio': user.bio,
          'location': location,
          'farmPhotoUrl': user.storeImageUrl,
          'coverPhotoUrl': coverPhotoUrl,
          // CONTRACT ALIGNMENT V1: No fake trust score - removed hardcoded value
          'isVerified': user.isEmailVerified || user.role != UserRole.guest,
          'lifecycle': ContentLifecycle.active,
          'sellerTier': null, // Own profile: tier not exposed via /users/me
        };
      } else if (viewedUser != null) {
        final user = viewedUser;
        // E5.2 — Canonical lifecycle from response.identity.lifecycle.
        // Degraded identities receive a redacted placeholder name, neutral
        // avatar (null), and have bio/location/farm suppressed so the
        // profile-detail surface renders an honest tombstone instead of
        // a stale active-looking card.
        final lifecycle = user.lifecycle;
        final degraded = lifecycle.isDegraded;
        final renderedName = degraded
            ? lifecycle.publicRedactionLabel
            : '@${user.username}';
        final renderedUsername = degraded ? '' : '@${user.username}';
        final renderedAvatar = degraded ? null : user.avatarUrl;
        final renderedBio = degraded ? null : (user.bio ?? '');
        // ONE identity rule — the same model getters every other surface uses.
        final identity = SellerIdentityData(
          userId: user.id,
          username: user.username,
          storeName: degraded ? null : user.storeName,
        );
        final renderedIdentityPrimary = degraded
            ? renderedName
            : (identity.primaryLabel ?? renderedUsername);
        final renderedIdentitySecondary = degraded
            ? null
            : identity.secondaryLabel;
        final renderedFarmName = degraded ? null : user.storeName;
        final renderedFarmPhoto = degraded ? null : user.storeImageUrl;
        final renderedLocation = degraded ? null : location;

        return {
          'name': renderedName,
          'farmName': renderedFarmName,
          'identityPrimary': renderedIdentityPrimary,
          'identitySecondary': renderedIdentitySecondary,
          'username': renderedUsername,
          'avatar': renderedAvatar,
          'bio': renderedBio,
          'location': renderedLocation,
          'farmPhotoUrl': renderedFarmPhoto,
          'coverPhotoUrl': degraded ? null : coverPhotoUrl,
          // CONTRACT ALIGNMENT V1: No fake trust score - removed hardcoded value
          // E5.2 — verification badges suppressed when degraded.
          'isVerified': degraded
              ? false
              : (user.isEmailVerified || user.role != UserRole.guest),
          'lifecycle': lifecycle,
          'sellerTier': degraded ? null : user.sellerTier?.apiValue,
        };
      }
    }

    throw StateError('Profile identity unresolved');
  }

  /// The ONE location value for this page: the backend's public origin line
  /// (city, province of the user's sender address), falling back to the
  /// user's own location field when no sender address exists. The client NEVER
  /// composes a location from raw address records — that rule lives server-side.
  String? _publicLocation(ProfileEntity? profile) {
    if (profile == null) return null;
    return normalizeProfileLocation(profile.publicOriginLine) ??
        normalizeProfileLocation(profile.location);
  }

  void _navigateToSettings(BuildContext context) {
    Navigator.of(
      context,
    ).push(MaterialPageRoute(builder: (context) => const SettingsScreen()));
  }

  Future<void> _handleMessageAction(String userId) async {
    final authState = ref.read(authControllerProvider);

    if (authState is! AuthStateAuthenticated) {
      if (mounted) {
        AppSnackBar.showError(context, 'Please login to send messages');
      }
      return;
    }

    final currentUserId = authState.user.id;
    final targetUserId = userId;

    // Don't allow messaging yourself
    if (currentUserId == targetUserId) {
      if (mounted) {
        AppSnackBar.showError(context, 'Cannot send message to yourself');
      }
      return;
    }

    try {
      // Show loading
      showDialog(
        context: context,
        barrierDismissible: false,
        builder: (context) => const Center(child: CircularProgressIndicator()),
      );

      // RECOVERY: Use new ChatList notifier instead of GetOrCreateChatUseCase
      final chat = await ref
          .read(chatListProvider.notifier)
          .getOrCreateChat(userId: currentUserId, otherUserId: targetUserId);

      // Hide loading
      if (mounted && context.mounted && Navigator.canPop(context)) {
        Navigator.pop(context);
      }

      if (chat != null && mounted) {
        // Navigate to chat detail using navigation handler
        final navigation = ref.read(navigationHandlerProvider);
        navigation.navigateToChatConversation(chat.id);
      } else if (mounted && context.mounted) {
        AppSnackBar.showError(context, 'Failed to create chat');
      }
    } catch (e) {
      // Hide loading if still showing
      if (mounted && context.mounted && Navigator.canPop(context)) {
        Navigator.pop(context);
      }

      if (mounted && context.mounted) {
        AppSnackBar.showError(context, 'Gagal membuka chat. Coba lagi.');
      }
    }
  }

  void _handleShareProfile() {
    // Get user data from ref
    final userData = ref.read(userDataProvider(widget.userId));

    userData.whenOrNull(
      data: (user) {
        if (user == null) return;

        // CANONICAL: payload built through the tested share-target builder
        // (@handle title, default description for empty bio, lifecycle
        // guard). Previously this screen duplicated that logic with raw
        // username and no degraded-identity guard.
        final shareTarget = buildProfileShareTarget(
          userId: user.id,
          username: user.username,
          bio: user.bio,
          avatarUrl: user.avatarUrl,
          lifecycle: user.lifecycle,
        );
        if (shareTarget == null) return;

        // Show share bottom sheet
        ShareBottomSheet.show(
          context: context,
          target: shareTarget,
          canSharePost: false,
        );
      },
      error: (error, _) {
        AppSnackBar.showError(context, 'Failed to load profile data');
      },
      loading: () {
        AppSnackBar.showInfo(context, 'Loading profile...');
      },
    );
  }

  Future<void> _handleUnblockUser(String userId) async {
    final authState = ref.read(authControllerProvider);
    final displayName = _getUserDisplayName(authState);

    final success = await ref
        .read(blockActionsProvider.notifier)
        .unblockUser(targetUserId: userId, targetDisplayName: displayName);

    if (!mounted) return;

    if (success) {
      AppSnackBar.showSuccess(context, '$displayName has been unblocked');
    } else {
      final error = ref.read(blockActionsProvider).error;
      AppSnackBar.showError(context, error ?? 'Failed to unblock user');
    }
  }

  Future<void> _handleBlockUser() async {
    final authState = ref.read(authControllerProvider);
    final actualUserId = _getActualUserId(authState);
    final displayName = _getUserDisplayName(authState);

    final userDataAsync = ref.read(userDataProvider(actualUserId));
    final avatarUrl = userDataAsync.hasValue
        ? userDataAsync.value?.avatarUrl
        : null;

    final confirmed = await BlockConfirmationDialog.show(
      context,
      targetUserId: actualUserId,
      targetDisplayName: displayName,
      targetAvatarUrl: avatarUrl,
    );

    if (confirmed != true || !mounted) return;

    final success = await ref
        .read(blockActionsProvider.notifier)
        .blockUser(
          targetUserId: actualUserId,
          targetDisplayName: displayName,
          targetAvatarUrl: avatarUrl,
        );

    if (!mounted) return;

    if (success) {
      AppSnackBar.showSuccess(context, '$displayName has been blocked');
      if (Navigator.of(context).canPop()) Navigator.of(context).pop();
    } else {
      final error = ref.read(blockActionsProvider).error;
      AppSnackBar.showError(context, error ?? 'Failed to block user');
    }
  }

  /// Handle report user from profile
  ///
  /// PHASE 2: User reporting is now enabled with proper context capture.
  /// Reports are stored with user context for moderation review.
  Future<void> _handleReportUser() async {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) {
      if (mounted) {
        AppSnackBar.showError(context, 'Please login to report users');
      }
      return;
    }

    final actualUserId = _getActualUserId(authState);

    // Don't allow reporting yourself
    if (actualUserId == authState.user.id) {
      if (mounted) {
        AppSnackBar.showError(context, 'Cannot report yourself');
      }
      return;
    }

    final displayName = _getUserDisplayName(authState);

    // Navigate to report screen with user context
    final result = await Navigator.of(context).push<bool>(
      MaterialPageRoute(
        builder: (context) => ReportScreen(
          targetType: ReportTargetType.user.name,
          targetId: actualUserId,
        ),
      ),
    );

    // After reporting, offer to block the user for immediate protection
    if (result == true && mounted) {
      _showReportFollowUpDialog(actualUserId, displayName);
    }
  }

  /// Show follow-up dialog after reporting offering additional protection
  void _showReportFollowUpDialog(String targetUserId, String displayName) {
    final scheme = Theme.of(context).colorScheme;
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
icon: Icon(
           Icons.shield_outlined,
           color: scheme.primary,
          size: AppIconSize.display,
        ),
        title: const Text('Report Submitted'),
        content: Text(
          'Thank you for your report. Would you also like to block $displayName to prevent further contact?',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('No, Thanks'),
          ),
          FilledButton(
            onPressed: () {
              Navigator.pop(context);
              _handleBlockUser();
            },
            child: const Text('Block User'),
          ),
        ],
      ),
    );
  }
}

/// Delegate for sticky TabBar
class _SliverTabBarDelegate extends SliverPersistentHeaderDelegate {
  final TabBar tabBar;
  final ColorScheme scheme;

  _SliverTabBarDelegate(this.tabBar, {required this.scheme});

  @override
  double get minExtent => tabBar.preferredSize.height;

  @override
  double get maxExtent => tabBar.preferredSize.height;

  @override
  Widget build(
    BuildContext context,
    double shrinkOffset,
    bool overlapsContent,
  ) {
    return Container(
      decoration: BoxDecoration(
        color: scheme.surface,
        border: Border(
          bottom: BorderSide(
            color: scheme.outlineVariant,
            width: 1,
          ),
        ),
      ),
      child: tabBar,
    );
  }

  @override
  bool shouldRebuild(_SliverTabBarDelegate oldDelegate) {
    return tabBar != oldDelegate.tabBar || scheme != oldDelegate.scheme;
  }
}

/// Delegate for sticky Sub-TabBar
class _SliverSubTabBarDelegate extends SliverPersistentHeaderDelegate {
  final TabBar tabBar;
  final ColorScheme scheme;

  _SliverSubTabBarDelegate(this.tabBar, {required this.scheme});

  @override
  double get minExtent => 40;

  @override
  double get maxExtent => 40;

  @override
  Widget build(
    BuildContext context,
    double shrinkOffset,
    bool overlapsContent,
  ) {
    return Container(
      color: scheme.surface,
      child: tabBar,
    );
  }

  @override
  bool shouldRebuild(_SliverSubTabBarDelegate oldDelegate) {
    return tabBar != oldDelegate.tabBar || scheme != oldDelegate.scheme;
  }
}
