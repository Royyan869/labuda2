import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/shared.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/profile_view_provider.dart';
import 'package:hishumi/domains/user/profile/domain/entities/profile_entity.dart';
// R4.3: Import providers instead of services directly
import 'package:hishumi/domains/user/profile/data/profile_providers.dart'
    show avatarUploadServiceProvider, coverPhotoUploadServiceProvider;
// R4.3: Type imports still needed for getter signatures
import 'package:hishumi/domains/user/profile/data/services/avatar_upload_service.dart';
import 'package:hishumi/domains/user/profile/data/services/cover_photo_upload_service.dart';
import 'package:hishumi/domains/user/preference/seller/data/seller_providers.dart'
    show storePhotoUploadServiceProvider;
import 'package:hishumi/domains/user/preference/seller/data/data.dart'
    show StorePhotoUploadService;
import 'edit_profile/edit_profile_cover_section.dart';
import 'edit_profile/edit_profile_avatar_section.dart';
import 'edit_profile/edit_profile_personal_section.dart';
import 'edit_profile/edit_profile_farm_section.dart';
import 'edit_profile/edit_profile_contact_section.dart';
import 'edit_profile/edit_profile_save_handler.dart';

/// Which section of the single-scroll edit page to bring into view on open.
enum UnifiedEditProfileSection { personal, business }

/// Unified Edit Profile Screen - Single page scroll design.
///
/// - Avatars at top (side by side for sellers)
/// - Personal Information section
/// - Farm Information section (sellers only)
/// - Contact & Social Media section
/// - Single Save button; enabled only when dirty and valid.
///
/// Loading is non-blocking: the form stays usable and shows an inline
/// indicator while the canonical profile fetch is pending.
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
  final _businessSectionKey = GlobalKey();

  // Resolved userId (handles 'current_user' placeholder)
  late final String _actualUserId;

  // Controllers
  late final TextEditingController _usernameController;
  late final TextEditingController _bioController;
  late final TextEditingController _farmNameController;
  late final TextEditingController _instagramController;
  late final TextEditingController _facebookController;
  late final TextEditingController _tiktokController;
  late final TextEditingController _twitterController;

  // Dirty tracking — snapshot taken once the initial data is hydrated.
  String? _initialSnapshot;

  // State
  bool _isLoading = false;
  bool _isSeller = false;
  bool _didHydrate = false;
  ProfileEntity? _cachedProfile;
  String? _avatarUrl;
  String? _selectedAvatarPath;
  bool _isAvatarMarkedForRemoval = false;
  String? _farmPhotoUrl;
  String? _selectedStorePhotoPath;
  bool _isStorePhotoMarkedForRemoval = false;
  String? _coverPhotoUrl;
  String? _selectedCoverPath;
  bool _isCoverMarkedForRemoval = false;

  ProviderSubscription<AsyncValue<ProfileViewData?>>? _profileSubscription;

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
    _loadData();
    _listenToProfile();
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
      duration: AppMotion.settled,
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

  /// ONE fetch for this page: the canonical `GET /users/{id}` via
  /// [profileViewDataProvider]. Used to hydrate cover + profile extension
  /// fields, then cached for the save operation. This is the same provider the
  /// Profile screen reads, so opening Edit Profile does not add a second
  /// source of truth.
  void _listenToProfile() {
    _profileSubscription = ref.listenManual(
      profileViewDataProvider(_actualUserId),
      (previous, next) {
        if (!next.hasValue || next.value == null) return;
        _updateFromProfile(next.value!.profile);
      },
      fireImmediately: true,
    );
  }

  /// Update state from the canonical profile entity. Only hydrates fields the
  /// user has not already changed.
  void _updateFromProfile(ProfileEntity profile) {
    if (!mounted) return;

    setState(() {
      _cachedProfile = profile;

      if (_selectedCoverPath == null && !_isCoverMarkedForRemoval) {
        _coverPhotoUrl = profile.coverPhotoUrl;
      }

      // Farm info for sellers — store name + photo only.
      final farm = profile.farmInfo;
      if (farm != null) {
        if (_selectedStorePhotoPath == null && !_isStorePhotoMarkedForRemoval) {
          _farmPhotoUrl = farm.farmPhotoUrl;
        }
        if (_farmNameController.text.isEmpty) {
          _farmNameController.text = farm.farmName;
        }
      }

      // Social media handles — only if not already edited.
      final contactInfo = profile.contactInfo;
      if (contactInfo != null) {
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

      if (!_didHydrate) {
        _didHydrate = true;
        _initialSnapshot = _snapshot();
      }
    });
  }

  void _initializeControllers() {
    _usernameController = TextEditingController();
    _bioController = TextEditingController();
    _farmNameController = TextEditingController();
    _instagramController = TextEditingController();
    _facebookController = TextEditingController();
    _tiktokController = TextEditingController();
    _twitterController = TextEditingController();

    // Text edits must refresh the dirty state / Save enabledment.
    for (final controller in [
      _bioController,
      _farmNameController,
      _instagramController,
      _facebookController,
      _tiktokController,
      _twitterController,
    ]) {
      controller.addListener(_onChanged);
    }
  }

  void _loadData() {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) return;

    final user = authState.user;
    setState(() {
      _isSeller = user.hasCreatedSellerProfile;
      _avatarUrl = user.avatarUrl;
    });

    // Username is canonical and read-only; bio is the only identity text.
    _usernameController.text = user.username;
    _bioController.text = user.bio ?? '';
  }

  /// Snapshot of every editable value, used for dirty detection.
  String _snapshot() => [
    _bioController.text,
    _farmNameController.text,
    _instagramController.text,
    _facebookController.text,
    _tiktokController.text,
    _twitterController.text,
    _coverPhotoUrl ?? '',
    _selectedCoverPath ?? '',
    _isCoverMarkedForRemoval ? '1' : '0',
    _selectedAvatarPath ?? '',
    _isAvatarMarkedForRemoval ? '1' : '0',
    _selectedStorePhotoPath ?? '',
    _isStorePhotoMarkedForRemoval ? '1' : '0',
  ].join('\u0000');

  bool get _isDirty =>
      _didHydrate && _initialSnapshot != null && _snapshot() != _initialSnapshot;

  @override
  void dispose() {
    _profileSubscription?.close();
    _usernameController.dispose();
    _bioController.dispose();
    _farmNameController.dispose();
    _instagramController.dispose();
    _facebookController.dispose();
    _tiktokController.dispose();
    _twitterController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final profileAsync = ref.watch(profileViewDataProvider(_actualUserId));

    return Scaffold(
      backgroundColor: scheme.surfaceContainerLowest,
      appBar: AppBar(
        title: const Text('Edit Profile'),
        bottom: profileAsync.isLoading && !_didHydrate
            ? const PreferredSize(
                preferredSize: Size.fromHeight(2),
                child: LinearProgressIndicator(minHeight: 2),
              )
            : null,
      ),
      body: Form(
        key: _formKey,
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(AppMetrics.p16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Cover Photo Section
              EditProfileCoverSection(
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
              _buildSectionHeader(
                'Informasi Profile',
                Icons.person_outline,
                scheme,
              ),
              const SizedBox(height: 16),
              EditProfilePersonalSection(
                usernameController: _usernameController,
                bioController: _bioController,
                onChanged: _onChanged,
              ),

              // Farm Information Section (Seller only)
              if (_isSeller) ...[
                const SizedBox(height: 32),
                KeyedSubtree(
                  key: _businessSectionKey,
                  child: _buildSectionHeader(
                    'Farm Information',
                    Icons.store_outlined,
                    scheme,
                  ),
                ),
                const SizedBox(height: 16),
                EditProfileFarmSection(
                  farmNameController: _farmNameController,
                ),
              ],

              // Contact & Social Media Section
              const SizedBox(height: 32),
              _buildSectionHeader(
                'Contact & Social Media',
                Icons.contact_phone_outlined,
                scheme,
              ),
              const SizedBox(height: 16),
              EditProfileContactSection(
                instagramController: _instagramController,
                facebookController: _facebookController,
                tiktokController: _tiktokController,
                twitterController: _twitterController,
              ),

              const SizedBox(height: 24),
            ],
          ),
        ),
      ),
      bottomNavigationBar: _buildActionBar(),
    );
  }

  void _onChanged([String? _]) {
    if (mounted) setState(() {});
  }

  Widget _buildSectionHeader(String title, IconData icon, ColorScheme scheme) {
    return Row(
      children: [
        Icon(icon, size: AppIconSize.action, color: scheme.primary),
        const SizedBox(width: 8),
        Text(
          title,
          style: context.typeRoles.titleCompact.copyWith(
            fontWeight: FontWeight.bold,
            color: scheme.onSurface,
          ),
        ),
      ],
    );
  }

  Widget _buildActionBar() {
    final canSave = _isDirty && !_isLoading;
    // Chrome owned by [BottomActionBar]. Dirty-state guard + double-submit
    // guard stay here: Save is disabled until something changed and never
    // re-enters while saving.
    return BottomActionBar(
      secondary: BottomBarAction(
        label: 'Cancel',
        onPressed: _isLoading ? null : () => Navigator.of(context).pop(),
      ),
      primary: BottomBarAction(
        label: 'Save',
        onPressed: canSave ? save : null,
        isLoading: _isLoading,
      ),
    );
  }

  /// Change cover photo
  void _changeCover() {
    AvatarEditorWidget.showEditModal(
      context: context,
      aspectRatio: 16 / 9,
      circularCrop: false,
      cropTitle: 'Potong Foto Sampul',
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

  void _changeAvatar() {
    AvatarEditorWidget.showEditModal(
      context: context,
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
      cropTitle: 'Potong Foto Toko',
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
