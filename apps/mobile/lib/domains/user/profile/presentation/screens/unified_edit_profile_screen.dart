import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/user/profile/profile.dart'
    show profileStreamProvider;
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
// R4.3: Import providers instead of services directly
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show avatarUploadServiceProvider, coverPhotoUploadServiceProvider;
// R4.3: Type imports still needed for getter signatures
import 'package:labuda/domains/user/profile/data/services/avatar_upload_service.dart';
import 'package:labuda/domains/user/profile/data/services/cover_photo_upload_service.dart';
import 'package:labuda/domains/user/preference/seller/data/seller_providers.dart'
    show storePhotoUploadServiceProvider;
import 'package:labuda/domains/user/preference/seller/data/data.dart'
    show StorePhotoUploadService;
import 'edit_profile/edit_profile_cover_section.dart';
import 'edit_profile/edit_profile_avatar_section.dart';
import 'edit_profile/edit_profile_personal_section.dart';
import 'edit_profile/edit_profile_farm_section.dart';
import 'edit_profile/edit_profile_contact_section.dart';
import 'edit_profile/edit_profile_save_handler.dart';

/// Which section of the single-scroll edit page to bring into view on open.
enum UnifiedEditProfileSection { personal, business }

/// Unified Edit Profile Screen - Single page scroll design
/// No tabs - all fields visible in one scrollable page
/// - Avatars at top (side by side for sellers)
/// - Personal Information section
/// - Farm Information section (sellers only)
/// - Single Save button validates and saves all data
class UnifiedEditProfileScreen extends ConsumerStatefulWidget {
  final String userId;
  final UnifiedEditProfileSection initialSection;

  const UnifiedEditProfileScreen({
    super.key,
    required this.userId,
    this.initialSection = UnifiedEditProfileSection.personal,
  });

  @override
  ConsumerState<UnifiedEditProfileScreen> createState() =>
      _UnifiedEditProfileScreenState();
}

class _UnifiedEditProfileScreenState
    extends ConsumerState<UnifiedEditProfileScreen>
    with EditProfileSaveHandler {
  final _formKey = GlobalKey<FormState>();
  final _personalSectionKey = GlobalKey();
  final _businessSectionKey = GlobalKey();

  // Resolved userId (handles 'current_user' placeholder)
  late final String _actualUserId;

  // Controllers
  late final TextEditingController _usernameController;
  late final TextEditingController _bioController;
  late final TextEditingController _farmNameController;
  late final TextEditingController _websiteController;
  late final TextEditingController _instagramController;
  late final TextEditingController _facebookController;
  late final TextEditingController _tiktokController;
  late final TextEditingController _twitterController;

  // State
  bool _isLoading = false;
  bool _isSeller = false;
  ProfileEntity? _cachedProfile; // Cached profile for save operations
  String? _avatarUrl;
  String? _selectedAvatarPath;
  bool _isAvatarMarkedForRemoval = false;
  String? _farmPhotoUrl;
  String? _selectedStorePhotoPath;
  bool _isStorePhotoMarkedForRemoval = false;
  String? _coverPhotoUrl;
  String? _selectedCoverPath;
  bool _isCoverMarkedForRemoval = false;
  DateTime? _establishedDate;
  bool _isEmailPublic = false;
  bool _isPhonePublic = false;
  bool _isSocialMediaPublic = true;

  ProviderSubscription<AsyncValue<ProfileEntity?>>? _profileSubscription;

  // Implement getters required by EditProfileSaveHandler mixin
  @override
  GlobalKey<FormState> get formKey => _formKey;
  @override
  String get actualUserId => _actualUserId;
  @override
  bool get isSeller => _isSeller;
  @override
  ProfileEntity? get cachedProfile => _cachedProfile;
  @override
  TextEditingController get usernameController => _usernameController;
  @override
  TextEditingController get bioController => _bioController;
  @override
  TextEditingController get farmNameController => _farmNameController;
  @override
  TextEditingController get websiteController => _websiteController;
  @override
  TextEditingController get instagramController => _instagramController;
  @override
  TextEditingController get facebookController => _facebookController;
  @override
  TextEditingController get tiktokController => _tiktokController;
  @override
  TextEditingController get twitterController => _twitterController;
  @override
  String? get avatarUrl => _avatarUrl;
  @override
  String? get selectedAvatarPath => _selectedAvatarPath;
  @override
  bool get isAvatarMarkedForRemoval => _isAvatarMarkedForRemoval;
  @override
  String? get coverPhotoUrl => _coverPhotoUrl;
  @override
  String? get selectedCoverPath => _selectedCoverPath;
  @override
  bool get isCoverMarkedForRemoval => _isCoverMarkedForRemoval;
  @override
  String? get farmPhotoUrl => _farmPhotoUrl;
  @override
  String? get selectedStorePhotoPath => _selectedStorePhotoPath;
  @override
  bool get isStorePhotoMarkedForRemoval => _isStorePhotoMarkedForRemoval;
  @override
  DateTime? get establishedDate => _establishedDate;
  @override
  bool get isEmailPublic => _isEmailPublic;
  @override
  bool get isPhonePublic => _isPhonePublic;
  @override
  bool get isSocialMediaPublic => _isSocialMediaPublic;
  @override
  // R4.3: Use providers instead of inline service creation
  CoverPhotoUploadService get coverPhotoUploadService =>
      ref.read(coverPhotoUploadServiceProvider);
  @override
  AvatarUploadService get avatarUploadService =>
      ref.read(avatarUploadServiceProvider);
  @override
  StorePhotoUploadService get storePhotoUploadService =>
      ref.read(storePhotoUploadServiceProvider);
  @override
  void setLoading(bool loading) => setState(() => _isLoading = loading);

  @override
  void initState() {
    super.initState();
    _actualUserId = _resolveUserId();
    _initializeControllers();
    // R4.3: Removed _initializeServices() - services now provided via Riverpod
    _setupProfileListener(); // Setup listener first to catch initial data
    _loadData();
    if (widget.initialSection == UnifiedEditProfileSection.business) {
      WidgetsBinding.instance.addPostFrameCallback(
        (_) => _scrollToInitialSection(),
      );
    }
  }

  /// Scrolls to the section requested via [UnifiedEditProfileScreen.initialSection].
  /// The business/farm section only exists for sellers, so this is a no-op
  /// for non-sellers (the personal section, already visible at the top, is
  /// the correct destination for them).
  void _scrollToInitialSection() {
    if (!mounted || !_isSeller) return;
    final targetContext = _businessSectionKey.currentContext;
    if (targetContext == null) return;
    Scrollable.ensureVisible(
      targetContext,
      duration: const Duration(milliseconds: 300),
      curve: Curves.easeInOut,
    );
  }

  /// Resolve 'current_user' placeholder to actual user ID
  String _resolveUserId() {
    if (widget.userId == 'current_user') {
      final authState = ref.read(authControllerProvider);
      if (authState is AuthStateAuthenticated) {
        return authState.user.id;
      }
    }
    return widget.userId;
  }

  /// Listen to profile stream and update state when data arrives
  void _setupProfileListener() {
    _profileSubscription = ref.listenManual(
      profileStreamProvider(_actualUserId),
      (previous, next) {
        // Always update when data is available (empty check is inside _updateFromProfile)
        if (next.hasValue && next.value != null) {
          setState(() {
            _cachedProfile = next.value; // Cache for save operations
          });
          _updateFromProfile(next.value!);
        }
      },
      fireImmediately: true,
    );
  }

  /// Update state from profile entity
  void _updateFromProfile(ProfileEntity profile) {
    if (!mounted) return;

    setState(() {
      // Cover photo - only update if not manually changed
      if (_selectedCoverPath == null && !_isCoverMarkedForRemoval) {
        _coverPhotoUrl = profile.coverPhotoUrl;
      }

      // Farm info for sellers - always update to catch new seller upgrades
      if (profile.farmInfo != null) {
        final farm = profile.farmInfo!;

        // Update farm photo if not manually changed
        if (_selectedStorePhotoPath == null && !_isStorePhotoMarkedForRemoval) {
          _farmPhotoUrl = farm.farmPhotoUrl;
        }

        // Update text fields only if empty to preserve user edits
        if (_farmNameController.text.isEmpty) {
          _farmNameController.text = farm.farmName;
        }
        if (_websiteController.text.isEmpty) {
          _websiteController.text = farm.farmWebsite ?? '';
        }
        _establishedDate ??= farm.establishedDate;
      }

      // Contact info - always update to catch changes
      if (profile.contactInfo != null) {
        final contactInfo = profile.contactInfo!;
        _isEmailPublic = contactInfo.isEmailPublic;
        _isPhonePublic = contactInfo.isPhonePublic;
        _isSocialMediaPublic = contactInfo.isSocialMediaPublic;

        // Update social media handles only if empty
        if (_instagramController.text.isEmpty) {
          _instagramController.text = contactInfo.instagramHandle ?? '';
        }
        if (_facebookController.text.isEmpty) {
          _facebookController.text = contactInfo.facebookHandle ?? '';
        }
        if (_tiktokController.text.isEmpty) {
          _tiktokController.text = contactInfo.tiktokHandle ?? '';
        }
        if (_twitterController.text.isEmpty) {
          _twitterController.text = contactInfo.twitterHandle ?? '';
        }
      }
    });
  }

  void _initializeControllers() {
    _usernameController = TextEditingController();
    _bioController = TextEditingController();
    _farmNameController = TextEditingController();
    _websiteController = TextEditingController();
    _instagramController = TextEditingController();
    _facebookController = TextEditingController();
    _tiktokController = TextEditingController();
    _twitterController = TextEditingController();
  }

  // R4.3: Removed _initializeServices() - services now provided via Riverpod providers

  void _loadData() {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) return;

    final user = authState.user;
    setState(() {
      _isSeller = user.hasCreatedSellerProfile;
      _avatarUrl = user.avatarUrl;
    });

    _usernameController.text = user.username;
    _bioController.text = user.bio ?? '';

    // Profile data will be loaded by _setupProfileListener
    // No need to load it here to avoid race conditions
  }

  @override
  void dispose() {
    _profileSubscription?.close();
    _usernameController.dispose();
    _bioController.dispose();
    _farmNameController.dispose();
    _websiteController.dispose();
    _instagramController.dispose();
    _facebookController.dispose();
    _tiktokController.dispose();
    _twitterController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final profileAsync = ref.watch(profileStreamProvider(_actualUserId));

    return Scaffold(
      backgroundColor: isDark ? AppColors.darkGray900 : AppColors.neutralGray50,
      appBar: AppBar(
        title: const Text('Edit Profile'),
        backgroundColor: isDark ? AppColors.darkGray800 : AppColors.light,
        surfaceTintColor: Colors.transparent,
        scrolledUnderElevation: 0,
      ),
      body: Stack(
        children: [
          Form(
            key: _formKey,
            child: SingleChildScrollView(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Cover Photo Section
                  EditProfileCoverSection(
                    userId: _actualUserId,
                    coverPhotoUrl: _coverPhotoUrl,
                    selectedCoverPath: _selectedCoverPath,
                    isCoverMarkedForRemoval: _isCoverMarkedForRemoval,
                    onChangeCover: _changeCover,
                    onRemoveCover: _removeCover,
                  ),

                  const SizedBox(height: 24),

                  // Avatar Section
                  EditProfileAvatarSection(
                    isSeller: _isSeller,
                    avatarUrl: _avatarUrl,
                    selectedAvatarPath: _selectedAvatarPath,
                    isAvatarMarkedForRemoval: _isAvatarMarkedForRemoval,
                    farmPhotoUrl: _farmPhotoUrl,
                    selectedStorePhotoPath: _selectedStorePhotoPath,
                    isStorePhotoMarkedForRemoval: _isStorePhotoMarkedForRemoval,
                    onChangeAvatar: _changeAvatar,
                    onRemoveAvatar: _removeAvatar,
                    onChangeStorePhoto: _changeStorePhoto,
                    onRemoveStorePhoto: _removeStorePhoto,
                  ),

                  const SizedBox(height: 24),

                  // Personal Information Section
                  KeyedSubtree(
                    key: _personalSectionKey,
                    child: _buildSectionHeader(
                      'Informasi Profile',
                      Icons.person_outline,
                      isDark,
                    ),
                  ),
                  const SizedBox(height: 16),
                  EditProfilePersonalSection(
                    usernameController: _usernameController,
                    bioController: _bioController,
                  ),

                  // Farm Information Section (Seller only)
                  if (_isSeller) ...[
                    const SizedBox(height: 32),
                    KeyedSubtree(
                      key: _businessSectionKey,
                      child: _buildSectionHeader(
                        'Farm Information',
                        Icons.store_outlined,
                        isDark,
                      ),
                    ),
                    const SizedBox(height: 16),
                    EditProfileFarmSection(
                      farmNameController: _farmNameController,
                      websiteController: _websiteController,
                      establishedDate: _establishedDate,
                      onEstablishedDateChanged: (date) =>
                          setState(() => _establishedDate = date),
                    ),
                  ],

                  // Contact & Social Media Section
                  const SizedBox(height: 32),
                  _buildSectionHeader(
                    'Contact & Social Media',
                    Icons.contact_phone_outlined,
                    isDark,
                  ),
                  const SizedBox(height: 16),
                  EditProfileContactSection(
                    isEmailPublic: _isEmailPublic,
                    isPhonePublic: _isPhonePublic,
                    isSocialMediaPublic: _isSocialMediaPublic,
                    instagramController: _instagramController,
                    facebookController: _facebookController,
                    tiktokController: _tiktokController,
                    twitterController: _twitterController,
                    onEmailPublicChanged: (value) =>
                        setState(() => _isEmailPublic = value),
                    onPhonePublicChanged: (value) =>
                        setState(() => _isPhonePublic = value),
                    onSocialMediaPublicChanged: (value) =>
                        setState(() => _isSocialMediaPublic = value),
                  ),

                  const SizedBox(height: 24),
                ],
              ),
            ),
          ),
          // Loading overlay
          if (profileAsync.isLoading && _cachedProfile == null)
            Container(
              color: isDark
                  ? AppColors.darkGray900.withValues(alpha: 0.7)
                  : AppColors.neutralGray50.withValues(alpha: 0.7),
              child: const Center(child: CircularProgressIndicator()),
            ),
        ],
      ),
      bottomNavigationBar: _buildActionBar(),
    );
  }

  /// Change cover photo
  void _changeCover() {
    AvatarEditorWidget.showEditModal(
      context: context,
      userId: _actualUserId,
      aspectRatio: 16 / 9,
      circularCrop: false,
      cropTitle: 'Crop Cover',
      modalTitle: 'Change Cover Photo',
      onAvatarUpdated: (path) {
        setState(() {
          _selectedCoverPath = path;
          _isCoverMarkedForRemoval = path == null;
        });
      },
    );
  }

  /// Remove cover photo
  void _removeCover() {
    setState(() {
      _selectedCoverPath = null;
      _isCoverMarkedForRemoval = true;
    });
  }

  Widget _buildSectionHeader(String title, IconData icon, bool isDark) {
    return Row(
      children: [
        Icon(icon, size: 20, color: AppColors.primaryRed),
        const SizedBox(width: 8),
        Text(
          title,
          style: TextStyle(
            fontSize: 16,
            fontWeight: FontWeight.bold,
            color: isDark ? AppColors.light : AppColors.dark,
          ),
        ),
      ],
    );
  }

  Widget _buildActionBar() {
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(
          children: [
            Expanded(
              child: AppButton.secondary(
                text: 'Cancel',
                onPressed: () => Navigator.of(context).pop(),
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: AppButton.primary(
                text: 'Save',
                // Stage 4D: guard against double-submit. The save flow runs
                // two async phases (personal + profile fields) with a
                // Navigator.pop at the end; an unguarded onPressed would let a
                // rapid second tap re-enter save() and duplicate uploads.
                onPressed: _isLoading ? null : save,
                isLoading: _isLoading,
              ),
            ),
          ],
        ),
      ),
    );
  }

  void _changeAvatar() {
    AvatarEditorWidget.showEditModal(
      context: context,
      userId: _actualUserId,
      showAdvancedCropper: false,
      onAvatarUpdated: (path) => setState(() {
        _selectedAvatarPath = path;
        _isAvatarMarkedForRemoval = path == null;
      }),
    );
  }

  void _removeAvatar() => setState(() {
    _selectedAvatarPath = null;
    _isAvatarMarkedForRemoval = true;
  });

  void _changeStorePhoto() {
    AvatarEditorWidget.showEditModal(
      context: context,
      userId: _actualUserId,
      showAdvancedCropper: true,
      onAvatarUpdated: (path) => setState(() {
        _selectedStorePhotoPath = path;
        _isStorePhotoMarkedForRemoval = path == null;
      }),
    );
  }

  void _removeStorePhoto() => setState(() {
    _selectedStorePhotoPath = null;
    _isStorePhotoMarkedForRemoval = true;
  });
}
