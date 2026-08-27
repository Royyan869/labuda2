import 'dart:async';

import 'package:firebase_auth/firebase_auth.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/config/seller_upgrade_config_entity.dart';
import 'package:labuda/core/config/seller_upgrade_config_provider.dart'
    as config;
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/widgets/blocked_action_gate.dart';
import 'package:labuda/domains/user/preference/seller/data/seller_providers.dart'
    show sellerRemoteDatasourceProvider, sellerRepositoryProvider,
        storePhotoUploadServiceProvider;
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_subscription.dart';
import 'package:labuda/domains/user/preference/seller/domain/repositories/seller_repository.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_state.dart';
import 'package:labuda/domains/user/preference/seller/presentation/widgets/wizard/seller_wizard_helpers.dart';
import 'package:labuda/domains/user/preference/seller/presentation/widgets/wizard/seller_wizard_navigation_buttons.dart';
import 'package:labuda/domains/user/preference/seller/presentation/widgets/wizard/seller_wizard_preview_widget.dart';
import 'package:labuda/domains/user/preference/seller/presentation/widgets/wizard/seller_wizard_step2_widget.dart';
import 'package:labuda/domains/user/profile/profile.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/helpers/canonical_phone_validator.dart';
import 'package:url_launcher/url_launcher.dart';

/// Seller Upgrade Wizard
///
/// Flow:
/// 1. Package & seller terms
/// 2. Account prerequisites
/// 3. Store/farm information
/// 4. Preview & terms
/// 5. Payment
enum _SellerUpgradeWizardMode {
  unhydrated,
  unauthenticated,
  registration,
  renewal,
  restricted,
}

class _SellerPaymentOperationContext {
  final String initiatingUserId;
  final int requestEpoch;
  final SellerSubscription? baselineSubscription;

  const _SellerPaymentOperationContext({
    required this.initiatingUserId,
    required this.requestEpoch,
    required this.baselineSubscription,
  });
}

class SellerUpgradeWizardScreen extends ConsumerStatefulWidget {
  const SellerUpgradeWizardScreen({super.key});

  @override
  ConsumerState<SellerUpgradeWizardScreen> createState() =>
      _SellerUpgradeWizardScreenState();
}

class _SellerUpgradeWizardScreenState
    extends ConsumerState<SellerUpgradeWizardScreen> {
  final PageController _pageController = PageController();
  int _currentStep = 0;
  final int _totalSteps = 5;

  final _accountFormKey = GlobalKey<FormState>();
  final _storeFormKey = GlobalKey<FormState>();

  final _usernameController = TextEditingController();
  final _bioController = TextEditingController();
  final _phoneController = TextEditingController();
  final _farmNameController = TextEditingController();

  String _initialUsername = '';
  String _initialBio = '';
  String _initialPhone = '';
  String? _initialSenderAddressId;
  String _initialFarmName = '';
  String? _initialFarmPhotoUrl;
  String? _initialSelectedStorePhotoPath;
  final bool _initialAgreeToTerms = false;

  AddressEntity? _selectedSenderAddress;
  String? _senderAddressError;
  bool _isLoadingSenderAddress = false;

  String? _farmPhotoUrl;
  String? _selectedStorePhotoPath;
  bool _isStorePhotoUploading = false;
  bool _agreeToTerms = false;
  bool _isSubmitting = false;

  ProviderSubscription<AuthState>? _authSubscription;
  ProviderSubscription<AsyncValue<ProfileEntity?>>? _profileSubscription;

  @override
  void initState() {
    super.initState();
    _authSubscription = ref.listenManual<AuthState>(
      authControllerProvider,
      _handleAuthStateChanged,
      fireImmediately: true,
    );

    for (final controller in [
      _usernameController,
      _bioController,
      _phoneController,
      _farmNameController,
    ]) {
      controller.addListener(_markDirty); } WidgetsBinding.instance.addPostFrameCallback((_) { final auth = ref.read(authControllerProvider); final user = ref.read(authenticatedUserProvider); if (_wizardModeFrom(auth, user) == _SellerUpgradeWizardMode.renewal) { _goToStep(4); } }); }

  void _markDirty() {
    if (mounted) setState(() {});
  }

  String? _principalIdForState(AuthState state) {
    return switch (state) {
      AuthStateAuthenticated(:final user) => user.id,
      AuthStateLoading(:final principal) => principal?.uid,
      AuthStateFirebaseAuthenticated(:final userId) => userId,
      AuthStateSyncingWithBackend(:final userId) => userId,
      AuthStateRequiresProfileCompletion(:final userId) => userId,
      AuthStateAccountRestricted(:final user) => user.id,
      _ => null,
    };
  }

  _SellerUpgradeWizardMode _wizardModeFrom(
    AuthState authState,
    AuthUser? authenticatedUser,
  ) {
    if (authState is AuthStateAuthenticated) {
      return authenticatedUser?.hasSellerProfile == true
          ? _SellerUpgradeWizardMode.renewal
          : _SellerUpgradeWizardMode.registration;
    }

    if (authState is AuthStateAccountRestricted) {
      return _SellerUpgradeWizardMode.restricted;
    }

    if (authState is AuthStateUnauthenticated || authState is AuthStateError) {
      return _SellerUpgradeWizardMode.unauthenticated;
    }

    return _SellerUpgradeWizardMode.unhydrated;
  }

  String? _currentAuthenticatedUserId() {
    final authState = ref.read(authControllerProvider);
    if (authState is AuthStateAuthenticated) {
      return authState.user.id;
    }
    return null;
  }

  void _clearPrincipalBoundState() {
    if (!mounted) return;

    setState(() {
      _currentStep = 0;
      _usernameController.clear();
      _bioController.clear();
      _phoneController.clear();
      _farmNameController.clear();
      _initialUsername = '';
      _initialBio = '';
      _initialPhone = '';
      _initialSenderAddressId = null;
      _initialFarmName = '';
      _initialFarmPhotoUrl = null;
      _initialSelectedStorePhotoPath = null;
      _selectedSenderAddress = null;
      _senderAddressError = null;
      _farmPhotoUrl = null;
      _selectedStorePhotoPath = null;
      _agreeToTerms = false;
      _isLoadingSenderAddress = false;
      _isStorePhotoUploading = false;
      _isSubmitting = false;
    });

    if (_pageController.hasClients) { _pageController.jumpToPage(0); } WidgetsBinding.instance.addPostFrameCallback((_) { final auth = ref.read(authControllerProvider); final user = ref.read(authenticatedUserProvider); if (_wizardModeFrom(auth, user) == _SellerUpgradeWizardMode.renewal) { _goToStep(4); } });
  }

  void _bindProfileListener(String userId) {
    _profileSubscription?.close();
    _profileSubscription = ref.listenManual(profileStreamProvider(userId), (
      previous,
      next,
    ) {
      if (next.hasValue) {
        _updateFromProfile(next.value);
      }
    }, fireImmediately: true);
  }

  void _hydrateAuthenticatedPrincipal(AuthUser user) {
    if (!mounted) return;

    setState(() {
      _usernameController.text = user.username;
      _bioController.text = user.bio ?? '';
      _phoneController.text = user.phoneNumber ?? '';
      _initialUsername = _usernameController.text.trim();
      _initialBio = _bioController.text.trim();
      _initialPhone = _phoneController.text.trim();
    });

    _bindProfileListener(user.id);
    unawaited(_loadSenderAddress());
  }

  void _handleAuthStateChanged(AuthState? previous, AuthState next) {
    final previousPrincipalId = previous == null
        ? null
        : _principalIdForState(previous);
    final nextPrincipalId = _principalIdForState(next);

    final principalChanged = previousPrincipalId != nextPrincipalId;
    if (principalChanged) {
      _principalEpoch++;
      _profileSubscription?.close();
      _profileSubscription = null;
      _clearPrincipalBoundState();
    }

    if (next is AuthStateAuthenticated) {
      _hydrateAuthenticatedPrincipal(next.user);
      return;
    }

    if (next is AuthStateAccountRestricted ||
        next is AuthStateUnauthenticated ||
        next is AuthStateError) {
      _profileSubscription?.close();
      _profileSubscription = null;
      _clearPrincipalBoundState();
    }
  }

  int _principalEpoch = 0;

  bool _isCurrentPrincipalRequest(int requestEpoch, String? userId) {
    if (!mounted) return false;
    if (requestEpoch != _principalEpoch) return false;
    return _currentAuthenticatedUserId() == userId;
  }

  void _updateFromProfile(ProfileEntity? profile) {
    if (!mounted || profile == null) return;

    setState(() {
      final farm = profile.farmInfo;
      if (farm != null) {
        _farmNameController.text = _farmNameController.text.isEmpty
            ? farm.farmName
            : _farmNameController.text;
        _farmPhotoUrl ??= farm.farmPhotoUrl;
      }

      if (_initialFarmName.isEmpty &&
          _farmNameController.text.trim().isNotEmpty) {
        _initialFarmName = _farmNameController.text.trim();
      }
      _initialFarmPhotoUrl ??= _farmPhotoUrl;
    });
  }

  Future<void> _loadSenderAddress() async {
    final requestEpoch = _principalEpoch;
    final userId = _currentAuthenticatedUserId();
    if (userId == null) return;

    if (mounted) {
      setState(() {
        _isLoadingSenderAddress = true;
        _senderAddressError = null;
      });
    }

    try {
      final repository = ref.read(addressRepositoryProvider);
      final result = await repository.getAddressesByPurpose(
        userId,
        AddressPurpose.sender,
      );

      if (!mounted || !_isCurrentPrincipalRequest(requestEpoch, userId)) {
        return;
      }

      result.fold(
        (error) {
          if (!_isCurrentPrincipalRequest(requestEpoch, userId)) return;
          setState(() {
            _selectedSenderAddress = null;
            _senderAddressError = 'Failed to load sender address: $error';
          });
        },
        (addresses) {
          if (!_isCurrentPrincipalRequest(requestEpoch, userId)) return;
          if (addresses.isEmpty) {
            setState(() {
              _selectedSenderAddress = null;
              _senderAddressError = 'Sender address is required.';
            });
            return;
          }

          final primaryAddress = addresses.firstWhere(
            (address) => address.isPrimary,
            orElse: () => addresses.first,
          );

          setState(() {
            _selectedSenderAddress = primaryAddress;
            _senderAddressError = null;
            _initialSenderAddressId ??= primaryAddress.id;
          });
        },
      );
    } catch (e) {
      if (!mounted || !_isCurrentPrincipalRequest(requestEpoch, userId)) {
        return;
      }
      setState(() {
        _selectedSenderAddress = null;
        _senderAddressError = 'Failed to load sender address: $e';
      });
    } finally {
      if (mounted && _isCurrentPrincipalRequest(requestEpoch, userId)) {
        setState(() => _isLoadingSenderAddress = false);
      }
    }
  }

  Future<SellerSubscription?> _loadBaselineSellerSubscription(
    String sellerId,
  ) async {
    try {
      final SellerRepository repository = ref.read(sellerRepositoryProvider);
      final result = await repository.getSubscription(sellerId);
      if (result.isSuccess && result.data != null) {
        return result.data;
      }
    } catch (_) {
      // Intentionally fail closed: renewal success is still gated by auth
      // refresh and a fresh subscription snapshot during polling.
    }
    return null;
  }

  Widget _buildReadOnlyStatusCard(
    bool isDark, {
    required String title,
    required IconData icon,
    required String value,
    String? note,
  }) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
        ),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(
            icon,
            size: 20,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray600,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  value,
                  style: TextStyle(
                    fontSize: 14,
                    color: isDark
                        ? AppColors.neutralGray200
                        : AppColors.neutralGray800,
                  ),
                ),
                if (note != null) ...[
                  const SizedBox(height: 4),
                  Text(
                    note,
                    style: TextStyle(
                      fontSize: 12,
                      color: isDark
                          ? AppColors.neutralGray500
                          : AppColors.neutralGray600,
                    ),
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSenderAddressSection(bool isDark) {
    final address = _selectedSenderAddress;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                'Sender Address *',
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w600,
                  color: isDark
                      ? AppColors.neutralGray200
                      : AppColors.neutralGray900,
                ),
              ),
            ),
            TextButton.icon(
              onPressed: _isLoadingSenderAddress
                  ? null
                  : _showSenderAddressDialog,
              icon: Icon(
                address == null ? Icons.add_location_alt_outlined : Icons.edit,
                size: 18,
              ),
              label: Text(address == null ? 'Add sender address' : 'Edit'),
            ),
          ],
        ),
        const SizedBox(height: 12),
        if (_isLoadingSenderAddress)
          const LinearProgressIndicator(minHeight: 2),
        if (_isLoadingSenderAddress) const SizedBox(height: 12),
        if (address != null)
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
              borderRadius: BorderRadius.circular(12),
              border: Border.all(
                color: isDark
                    ? AppColors.darkGray600
                    : AppColors.neutralGray200,
              ),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Icon(
                      Icons.warehouse_outlined,
                      size: 20,
                      color: isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray600,
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Text(
                        address.displayLabel,
                        style: TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                          color: isDark
                              ? AppColors.neutralGray200
                              : AppColors.neutralGray900,
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 12),
                Text(
                  'Recipient: ${address.recipientName}',
                  style: TextStyle(
                    fontSize: 13,
                    color: isDark
                        ? AppColors.neutralGray300
                        : AppColors.neutralGray700,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  'Phone: ${address.phone}',
                  style: TextStyle(
                    fontSize: 13,
                    color: isDark
                        ? AppColors.neutralGray300
                        : AppColors.neutralGray700,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  address.fullAddress,
                  style: TextStyle(
                    fontSize: 13,
                    height: 1.5,
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray600,
                  ),
                ),
                if (!address.isPrimary) ...[
                  const SizedBox(height: 8),
                  Text(
                    'This sender address is not marked primary yet.',
                    style: TextStyle(
                      fontSize: 12,
                      color: AppColors.warningYellow,
                    ),
                  ),
                ],
              ],
            ),
          )
        else
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: isDark
                  ? AppColors.darkGray700.withValues(alpha: 0.5)
                  : AppColors.neutralGray50,
              borderRadius: BorderRadius.circular(12),
              border: Border.all(
                color: isDark
                    ? AppColors.darkGray600
                    : AppColors.neutralGray200,
              ),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'No sender address selected yet.',
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: isDark
                        ? AppColors.neutralGray200
                        : AppColors.neutralGray800,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  'Use the structured address form to add province, city/regency, district, village/subdistrict, street address, and postal code.',
                  style: TextStyle(
                    fontSize: 12,
                    height: 1.4,
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray600,
                  ),
                ),
              ],
            ),
          ),
        if (_senderAddressError != null) ...[
          const SizedBox(height: 8),
          Text(
            _senderAddressError!,
            style: const TextStyle(fontSize: 12, color: AppColors.error),
          ),
        ],
      ],
    );
  }

  Future<void> _showSenderAddressDialog() async {
    final userId = _currentAuthenticatedUserId();
    if (userId == null) {
      AppSnackBar.showError(context, 'User not authenticated');
      return;
    }

    FocusManager.instance.primaryFocus?.unfocus();

    final saved = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AddEditAddressDialog(
        userId: userId,
        address: _selectedSenderAddress,
        forcedPurpose: AddressPurpose.sender,
      ),
    );

    if (saved == true) {
      await _loadSenderAddress();
    }
  }

  @override
  void dispose() {
    _authSubscription?.close();
    _profileSubscription?.close();
    for (final controller in [
      _usernameController,
      _bioController,
      _phoneController,
      _farmNameController,
    ]) {
      controller.removeListener(_markDirty);
      controller.dispose();
    }
    _pageController.dispose();
    super.dispose();
  }

  bool get _isAccountStepValid {
    final authState = ref.read(authControllerProvider);
    final emailVerified = authState is AuthStateAuthenticated
        ? authState.user.isEmailVerified
        : false;
    return SellerWizardHelpers.isAccountStepValid(
      emailVerified: emailVerified,
      username: _usernameController.text.trim(),
      bio: _bioController.text.trim(),
      phoneNumber: _phoneController.text.trim(),
      senderAddress: _selectedSenderAddress?.fullAddress.trim() ?? '',
    );
  }

  bool get _isStoreStepValid =>
      SellerWizardHelpers.isStoreStepValid(
        storeName: _farmNameController.text.trim(),
      ) &&
      !_isStorePhotoUploading;

  bool get _canSubmit =>
      _isAccountStepValid && _isStoreStepValid && _agreeToTerms;

  bool get _hasAnyChanges {
    return _usernameController.text.trim() != _initialUsername ||
        _bioController.text.trim() != _initialBio ||
        _phoneController.text.trim() != _initialPhone ||
        _selectedSenderAddress?.id != _initialSenderAddressId ||
        _farmNameController.text.trim() != _initialFarmName ||
        _farmPhotoUrl != _initialFarmPhotoUrl ||
        _selectedStorePhotoPath != _initialSelectedStorePhotoPath ||
        _agreeToTerms != _initialAgreeToTerms;
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final configAsync = ref.watch(config.sellerUpgradeConfigProvider);
    final packageConfig = configAsync.asData?.value;
    final packageStepWidget = configAsync.when(
      data: (data) => _buildPackageDisclosureStep(data, isDark),
      loading: () => _buildPackageLoadingStep(isDark),
      error: (error, _) => _buildPackageErrorStep(isDark, error.toString()),
    );
    final authState = ref.watch(authControllerProvider);
    final authenticatedUser = ref.watch(authenticatedUserProvider);
    final sellerState = SellerState.fromAuthUser(authenticatedUser);
    final wizardMode = _wizardModeFrom(authState, authenticatedUser);
    final isEmailVerified = authState is AuthStateAuthenticated
        ? authState.user.isEmailVerified
        : false;
    final lifecycleLabel = authState is AuthStateAuthenticated
        ? authState.user.lifecycle.name
        : 'unknown';
    final previewStepWidget = packageConfig == null
        ? _buildPackagePendingStep(
            isDark,
            'Seller package must load before you can preview the onboarding summary.',
          )
        : SellerWizardPreviewWidget(
            username: _usernameController.text.trim(),
            bio: _bioController.text.trim(),
            phoneNumber: _phoneController.text.trim(),
            senderAddress: _selectedSenderAddress?.fullAddress.trim() ?? '',
            emailVerified: isEmailVerified,
            farmName: _farmNameController.text.trim(),
            farmPhotoUrl: _farmPhotoUrl,
            selectedStorePhotoPath: _selectedStorePhotoPath,
            packageFee: packageConfig.yearlyFee,
            packageDurationDays: packageConfig.durationDays,
            agreeToTerms: _agreeToTerms,
            onAgreeToTermsChanged: (value) =>
                setState(() => _agreeToTerms = value),
            isDark: isDark,
          );
    final paymentStepWidget = packageConfig == null
        ? _buildPackagePendingStep(
            isDark,
            'Seller package must load before payment can continue.',
          )
        : _buildPaymentStep(packageConfig, isDark, wizardMode, sellerState);
    final canAdvanceFromPackage =
        packageConfig != null && packageConfig.isEnabled;
    final isOperationalMode =
        wizardMode == _SellerUpgradeWizardMode.registration ||
        wizardMode == _SellerUpgradeWizardMode.renewal;

    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) async {
        if (didPop) return;

        if (_currentStep > 0) {
          _previousStep();
          return;
        }

        final shouldPop = await SellerWizardHelpers.showExitConfirmation(
          context,
          _hasAnyChanges,
        );
        if (shouldPop && context.mounted) {
          Navigator.of(context).pop();
        }
      },
      child: Scaffold(
        appBar: AppBarCustom(
          title: switch (wizardMode) {
            _SellerUpgradeWizardMode.registration => 'Daftar Seller',
            _SellerUpgradeWizardMode.renewal => 'Perpanjang Seller',
            _SellerUpgradeWizardMode.restricted => 'Akun Dibatasi',
            _SellerUpgradeWizardMode.unauthenticated => 'Login Diperlukan',
            _SellerUpgradeWizardMode.unhydrated => 'Memuat Seller',
          },
          leading: IconButton(
            icon: const Icon(Icons.close),
            onPressed: () async {
              final shouldPop = await SellerWizardHelpers.showExitConfirmation(
                context,
                _hasAnyChanges,
              );
              if (shouldPop && context.mounted) {
                Navigator.of(context).pop();
              }
            },
          ),
        ),
        body: isOperationalMode
            ? Column(
                children: [
                  _buildModeBanner(isDark, wizardMode, sellerState),
                  WizardProgressIndicator(
                    currentStep: _currentStep,
                    totalSteps: _totalSteps,
                    stepLabels: const [
                      'Paket',
                      'Akun',
                      'Toko/Farm',
                      'Preview',
                      'Pembayaran',
                    ],
                    isDark: isDark,
                  ),
                  Expanded(
                    child: PageView(
                      controller: _pageController,
                      physics: const NeverScrollableScrollPhysics(),
                      onPageChanged: (index) =>
                          setState(() => _currentStep = index),
                      children: [
                        packageStepWidget,
                        _buildAccountStep(
                          isDark,
                          wizardMode,
                          sellerState,
                          isEmailVerified,
                          lifecycleLabel,
                          authState is AuthStateAuthenticated
                              ? authState.user.email
                              : '',
                        ),
                        SellerWizardStep2Widget(
                          formKey: _storeFormKey,
                          farmNameController: _farmNameController,
                          onStorePhotoUpload: _handleStorePhotoUpload,
                          farmPhotoUrl: _farmPhotoUrl,
                          selectedStorePhotoPath: _selectedStorePhotoPath,
                          isDark: isDark,
                        ),
                        previewStepWidget,
                        paymentStepWidget,
                      ],
                    ),
                  ),
                  SellerWizardNavigationButtons(
                    currentStep: _currentStep,
                    totalSteps: _totalSteps,
                    isCurrentStepValid: switch (_currentStep) {
                      0 => canAdvanceFromPackage,
                      1 => _isAccountStepValid,
                      2 => _isStoreStepValid,
                      3 => _agreeToTerms,
                      4 => _canSubmit && canAdvanceFromPackage,
                      _ => false,
                    },
                    canSubmit: _canSubmit && canAdvanceFromPackage,
                    onPrevious: _previousStep,
                    onNext: _nextStep,
                    onSubmit: _submitUpgrade,
                    isDark: isDark,
                  ),
                ],
              )
            : _buildWizardGate(
                isDark,
                icon: wizardMode == _SellerUpgradeWizardMode.restricted
                    ? Icons.block
                    : wizardMode == _SellerUpgradeWizardMode.unauthenticated
                    ? Icons.login
                    : Icons.hourglass_bottom,
                title: switch (wizardMode) {
                  _SellerUpgradeWizardMode.restricted => 'Akun dibatasi',
                  _SellerUpgradeWizardMode.unauthenticated =>
                    'Login diperlukan',
                  _SellerUpgradeWizardMode.unhydrated =>
                    'Seller account is loading',
                  _SellerUpgradeWizardMode.registration ||
                  _SellerUpgradeWizardMode.renewal => 'Seller upgrade',
                },
                message: switch (wizardMode) {
                  _SellerUpgradeWizardMode.restricted =>
                    'This seller account is restricted and cannot continue here.',
                  _SellerUpgradeWizardMode.unauthenticated =>
                    'Sign in again to continue with seller registration or renewal.',
                  _SellerUpgradeWizardMode.unhydrated =>
                    'Waiting for the current authenticated principal to hydrate before seller actions are enabled.',
                  _SellerUpgradeWizardMode.registration ||
                  _SellerUpgradeWizardMode.renewal =>
                    'Seller content is available only after the current account is operational.',
                },
              ),
      ),
    );
  }

  Widget _buildWizardGate(
    bool isDark, {
    required IconData icon,
    required String title,
    required String message,
  }) {
    return Center(
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(24),
        child: Container(
          width: double.infinity,
          padding: const EdgeInsets.all(24),
          margin: const EdgeInsets.symmetric(horizontal: 8),
          decoration: BoxDecoration(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
            borderRadius: BorderRadius.circular(16),
            border: Border.all(
              color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
            ),
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                icon,
                size: 40,
                color: isDark
                    ? AppColors.neutralGray300
                    : AppColors.neutralGray600,
              ),
              const SizedBox(height: 16),
              Text(
                title,
                textAlign: TextAlign.center,
                style: TextStyle(
                  fontSize: 20,
                  fontWeight: FontWeight.bold,
                  color: isDark
                      ? AppColors.neutralWhite
                      : AppColors.neutralGray900,
                ),
              ),
              const SizedBox(height: 12),
              Text(
                message,
                textAlign: TextAlign.center,
                style: TextStyle(
                  fontSize: 14,
                  height: 1.5,
                  color: isDark
                      ? AppColors.neutralGray300
                      : AppColors.neutralGray700,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildModeBanner(
    bool isDark,
    _SellerUpgradeWizardMode wizardMode,
    SellerState sellerState,
  ) {
    final isRegistration = wizardMode == _SellerUpgradeWizardMode.registration;
    final headline = isRegistration
        ? 'Registration mode'
        : sellerState.isExpired
        ? 'Renewal mode'
        : 'Early renewal mode';
    final message = isRegistration
        ? 'Never-sellers enter the canonical onboarding flow and may create a seller profile only through registration.'
        : sellerState.isExpired
        ? 'Existing seller profile detected. Renew to restore market authority without recreating identity.'
        : 'Existing seller profile detected. Early renewal keeps your seller identity intact.';

    return Container(
      width: double.infinity,
      margin: const EdgeInsets.fromLTRB(16, 16, 16, 12),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: isRegistration
              ? [
                  AppColors.primaryBlue.withValues(alpha: 0.18),
                  AppColors.primaryBlue.withValues(alpha: 0.06),
                ]
              : [
                  AppColors.successGreen.withValues(alpha: 0.16),
                  AppColors.successGreen.withValues(alpha: 0.05),
                ],
        ),
        borderRadius: BorderRadius.circular(16),
        border: Border.all(
          color: isRegistration
              ? AppColors.primaryBlue.withValues(alpha: 0.35)
              : AppColors.successGreen.withValues(alpha: 0.35),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            headline,
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w700,
              color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 6),
          Text(
            message,
            style: TextStyle(
              fontSize: 13,
              height: 1.4,
              color: isDark
                  ? AppColors.neutralGray200
                  : AppColors.neutralGray700,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Seller identity: ${sellerState.displayLabel}',
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w600,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray700,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildAccountStep(
    bool isDark,
    _SellerUpgradeWizardMode wizardMode,
    SellerState sellerState,
    bool isEmailVerified,
    String lifecycleLabel,
    String email,
  ) {
    return Form(
      key: _accountFormKey,
      child: ListView(
        padding: const EdgeInsets.all(24),
        children: [
          Text(
            wizardMode == _SellerUpgradeWizardMode.registration
                ? 'Lengkapi Akun Seller Baru'
                : 'Perpanjang Akun Seller',
            style: TextStyle(
              fontSize: 20,
              fontWeight: FontWeight.bold,
              color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            wizardMode == _SellerUpgradeWizardMode.registration
                ? 'Email status tetap read-only. Isi data akun sebelum lanjut ke info toko dan pembayaran pertama.'
                : 'Seller profile Anda sudah ada. Perbarui data akun bila perlu lalu lanjut ke perpanjangan langganan.',
            style: TextStyle(
              fontSize: 14,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
          ),
          const SizedBox(height: 16),
          _buildReadOnlyStatusCard(
            isDark,
            title: 'Email',
            icon: Icons.email_outlined,
            value: email.isNotEmpty ? email : '-',
            note: isEmailVerified ? 'Verified' : 'Not verified',
          ),
          const SizedBox(height: 12),
          _buildStatusCard(
            isDark,
            title: 'Account status',
            items: [
              'Lifecycle: $lifecycleLabel',
              if (wizardMode == _SellerUpgradeWizardMode.registration)
                'First-time seller registration uses the canonical onboarding flow'
              else
                'Existing seller identity stays intact during renewal',
              if (sellerState.isExpired)
                'Renewal is required before market actions reopen'
              else if (sellerState.isActive)
                'Early renewal is allowed while current authority remains active',
            ],
          ),
          const SizedBox(height: 24),
          if (_usernameController.text.trim().isNotEmpty)
            _buildReadOnlyStatusCard(
              isDark,
              title: 'Username',
              icon: Icons.alternate_email,
              value: _usernameController.text.trim(),
              note: 'Read only from your profile.',
            )
          else
            AppTextField(
              controller: _usernameController,
              labelText: 'Username *',
              hintText: 'your_username',
              prefixIcon: Icons.alternate_email,
              validator: (value) =>
                  value == null || value.trim().isEmpty ? 'Required' : null,
            ),
          const SizedBox(height: 16),
          AppTextField(
            controller: _bioController,
            labelText: 'Bio *',
            hintText: 'Tell customers about your store or farm',
            prefixIcon: Icons.description_outlined,
            maxLines: 4,
            validator: (value) =>
                value == null || value.trim().isEmpty ? 'Required' : null,
          ),
          const SizedBox(height: 16),
          AppTextField(
            controller: _phoneController,
            labelText: 'Phone Number *',
            hintText: '+62...',
            prefixIcon: Icons.phone_outlined,
            keyboardType: TextInputType.phone,
            validator: (value) =>
                CanonicalPhoneValidator.validationMessage(value),
          ),
          const SizedBox(height: 16),
          _buildSenderAddressSection(isDark),
          const SizedBox(height: 16),
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: AppColors.primaryBlue.withValues(alpha: 0.08),
              borderRadius: BorderRadius.circular(12),
            ),
            child: Text(
              wizardMode == _SellerUpgradeWizardMode.registration
                  ? 'Username is read only when already saved. Bio, phone, and sender address remain required for seller onboarding.'
                  : 'Username is read only when already saved. Bio, phone, and sender address remain required for seller renewal checks.',
              style: TextStyle(
                fontSize: 13,
                color: isDark
                    ? AppColors.neutralGray300
                    : AppColors.neutralGray700,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildPackageDisclosureStep(
    SellerUpgradeConfigEntity upgradeConfig,
    bool isDark,
  ) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Paket & Syarat Seller',
            style: TextStyle(
              fontSize: 24,
              fontWeight: FontWeight.bold,
              color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Lihat fee seller dari backend sebelum mengisi data akun. Pembayaran diperlukan agar seller aktif.',
            style: TextStyle(
              fontSize: 14,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
          ),
          const SizedBox(height: 24),
          _buildPaidPlanCard(upgradeConfig, isDark),
          if (!upgradeConfig.isEnabled) ...[
            const SizedBox(height: 16),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                color: AppColors.error.withValues(alpha: 0.08),
                borderRadius: BorderRadius.circular(12),
                border: Border.all(
                  color: AppColors.error.withValues(alpha: 0.2),
                ),
              ),
              child: Text(
                'Seller registration is currently disabled by backend config.',
                style: TextStyle(
                  fontSize: 13,
                  color: isDark
                      ? AppColors.neutralGray300
                      : AppColors.neutralGray700,
                ),
              ),
            ),
          ],
          const SizedBox(height: 24),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: AppColors.statusInfo.withValues(alpha: 0.08),
              borderRadius: BorderRadius.circular(12),
              border: Border.all(
                color: AppColors.statusInfo.withValues(alpha: 0.2),
              ),
            ),
            child: Text(
              'KYC dan review bank dipakai untuk payout/withdrawal, bukan untuk registrasi seller awal.',
              style: TextStyle(
                fontSize: 13,
                color: isDark
                    ? AppColors.neutralGray300
                    : AppColors.neutralGray700,
              ),
            ),
          ),
          const SizedBox(height: 24),
          _buildFeaturesList(isDark),
        ],
      ),
    );
  }

  Widget _buildPaymentStep(
    SellerUpgradeConfigEntity upgradeConfig,
    bool isDark,
    _SellerUpgradeWizardMode wizardMode,
    SellerState sellerState,
  ) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Pembayaran',
            style: TextStyle(
              fontSize: 24,
              fontWeight: FontWeight.bold,
              color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            wizardMode == _SellerUpgradeWizardMode.registration
                ? 'Onboarding hanya dipanggil setelah prerequisites valid. Subscription akan dimulai setelah onboarding sukses.'
                : 'Anda sudah memiliki seller profile. Perpanjangan langganan hanya memperbarui market authority, bukan identity seller.',
            style: TextStyle(
              fontSize: 14,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
          ),
          const SizedBox(height: 24),
          _buildPaymentSection(upgradeConfig, isDark),
          const SizedBox(height: 24),
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: AppColors.warningYellow.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(12),
            ),
            child: Text(
              sellerState.isExpired
                  ? 'KYC dan review bank dipakai nanti untuk payout/withdrawal, bukan registrasi seller awal.'
                  : 'KYC dan review bank dipakai nanti untuk payout/withdrawal, terpisah dari renewals dan authority activation.',
              style: TextStyle(
                fontSize: 13,
                color: isDark
                    ? AppColors.neutralGray300
                    : AppColors.neutralGray700,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildPackageLoadingStep(bool isDark) {
    return _buildPackageStateCard(
      isDark,
      title: 'Paket & Syarat Seller',
      message: 'Mengambil fee seller dari backend...',
      leading: const CircularProgressIndicator(strokeWidth: 2),
    );
  }

  Widget _buildPackageErrorStep(bool isDark, String error) {
    return _buildPackageStateCard(
      isDark,
      title: 'Paket & Syarat Seller',
      message: 'Gagal memuat konfigurasi seller dari backend.\n$error',
      leading: const Icon(
        Icons.error_outline,
        color: AppColors.error,
        size: 28,
      ),
      actionLabel: 'Coba lagi',
      onAction: () => ref.invalidate(config.sellerUpgradeConfigProvider),
    );
  }

  Widget _buildPackagePendingStep(bool isDark, String message) {
    return _buildPackageStateCard(
      isDark,
      title: 'Paket & Syarat Seller',
      message: message,
      leading: const Icon(
        Icons.info_outline,
        color: AppColors.primaryBlue,
        size: 28,
      ),
    );
  }

  Widget _buildPackageStateCard(
    bool isDark, {
    required String title,
    required String message,
    Widget? leading,
    String? actionLabel,
    VoidCallback? onAction,
  }) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Container(
        width: double.infinity,
        padding: const EdgeInsets.all(20),
        decoration: BoxDecoration(
          color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
          borderRadius: BorderRadius.circular(16),
          border: Border.all(
            color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
          ),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              title,
              style: TextStyle(
                fontSize: 22,
                fontWeight: FontWeight.bold,
                color: isDark
                    ? AppColors.neutralWhite
                    : AppColors.neutralGray900,
              ),
            ),
            const SizedBox(height: 16),
            if (leading != null) ...[leading, const SizedBox(height: 16)],
            Text(
              message,
              style: TextStyle(
                fontSize: 14,
                height: 1.5,
                color: isDark
                    ? AppColors.neutralGray300
                    : AppColors.neutralGray700,
              ),
            ),
            if (actionLabel != null && onAction != null) ...[
              const SizedBox(height: 16),
              SizedBox(
                width: double.infinity,
                child: ElevatedButton(
                  onPressed: onAction,
                  child: Text(actionLabel),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildStatusCard(
    bool isDark, {
    required String title,
    required List<String> items,
  }) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w700,
              color: isDark
                  ? AppColors.neutralGray200
                  : AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 8),
          ...items.map(
            (item) => Padding(
              padding: const EdgeInsets.only(bottom: 6),
              child: Text(
                '• $item',
                style: TextStyle(
                  fontSize: 13,
                  color: isDark
                      ? AppColors.neutralGray300
                      : AppColors.neutralGray700,
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildPaidPlanCard(
    SellerUpgradeConfigEntity upgradeConfig,
    bool isDark,
  ) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(24),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: isDark
              ? [
                  AppColors.primaryBlue.withValues(alpha: 0.2),
                  AppColors.primaryBlue.withValues(alpha: 0.05),
                ]
              : [
                  AppColors.primaryBlue.withValues(alpha: 0.1),
                  AppColors.primaryBlue.withValues(alpha: 0.02),
                ],
        ),
        borderRadius: BorderRadius.circular(16),
        border: Border.all(
          color: AppColors.primaryBlue.withValues(alpha: 0.5),
          width: 2,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
            decoration: BoxDecoration(
              color: AppColors.primaryBlue,
              borderRadius: BorderRadius.circular(20),
            ),
            child: const Text(
              'AKTIVASI SELLER',
              style: TextStyle(
                color: AppColors.neutralWhite,
                fontSize: 12,
                fontWeight: FontWeight.bold,
              ),
            ),
          ),
          const SizedBox(height: 16),
          Text(
            'Aktivasi Seller',
            style: TextStyle(
              fontSize: 22,
              fontWeight: FontWeight.bold,
              color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 8),
          Row(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              Text(
                'Rp ${AppFormatters.formatCurrency(upgradeConfig.yearlyFee)}',
                style: const TextStyle(
                  fontSize: 32,
                  fontWeight: FontWeight.bold,
                  color: AppColors.primaryBlue,
                ),
              ),
              const SizedBox(width: 8),
              Padding(
                padding: const EdgeInsets.only(bottom: 6),
                child: Text(
                  '/${upgradeConfig.durationDays} hari',
                  style: TextStyle(
                    fontSize: 14,
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray600,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 16),
          Text(
            'Seller access stays active for ${upgradeConfig.durationDays} days after payment is confirmed.',
            style: TextStyle(
              fontSize: 14,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Fee sourced from backend config. KYC and bank review happen later for payout access.',
            style: TextStyle(
              fontSize: 12,
              color: isDark
                  ? AppColors.neutralGray500
                  : AppColors.neutralGray600,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildFeaturesList(bool isDark) {
    final features = [
      ('Buat listing', Icons.inventory_2_outlined),
      ('Buat lelang', Icons.gavel),
      ('Buat promosi', Icons.campaign_outlined),
      (
        'Seller authority stays active while subscription is valid',
        Icons.verified_outlined,
      ),
    ];

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'What You Get',
          style: TextStyle(
            fontSize: 16,
            fontWeight: FontWeight.bold,
            color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
          ),
        ),
        const SizedBox(height: 16),
        ...features.map(
          (feature) => Padding(
            padding: const EdgeInsets.only(bottom: 12),
            child: Row(
              children: [
                Container(
                  width: 24,
                  height: 24,
                  decoration: BoxDecoration(
                    color: AppColors.successGreen.withValues(alpha: 0.1),
                    shape: BoxShape.circle,
                  ),
                  child: const Icon(
                    Icons.check,
                    color: AppColors.successGreen,
                    size: 16,
                  ),
                ),
                const SizedBox(width: 12),
                Icon(
                  feature.$2,
                  size: 20,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    feature.$1,
                    style: TextStyle(
                      fontSize: 14,
                      color: isDark
                          ? AppColors.neutralGray300
                          : AppColors.neutralGray700,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildPaymentSection(
    SellerUpgradeConfigEntity upgradeConfig,
    bool isDark,
  ) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Seller Payment Summary',
            style: TextStyle(
              fontSize: 16,
              fontWeight: FontWeight.bold,
              color: isDark
                  ? AppColors.neutralGray200
                  : AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 16),
          _buildPaymentRow(
            'Yearly subscription',
            upgradeConfig.yearlyFee,
            isDark,
          ),
          const Divider(height: 24),
          _buildPaymentRow(
            'Total',
            upgradeConfig.yearlyFee,
            isDark,
            isBold: true,
          ),
          const SizedBox(height: 12),
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: AppColors.statusInfo.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(8),
            ),
            child: Row(
              children: [
                const Icon(
                  Icons.info_outline,
                  size: 16,
                  color: AppColors.primaryBlue,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    'You will be redirected to the payment provider in a browser.',
                    style: TextStyle(
                      fontSize: 12,
                      color: isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray600,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildPaymentRow(
    String label,
    double amount,
    bool isDark, {
    bool isBold = false,
  }) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Expanded(
          child: Text(
            label,
            style: TextStyle(
              fontSize: 14,
              fontWeight: isBold ? FontWeight.bold : FontWeight.normal,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray700,
            ),
          ),
        ),
        const SizedBox(width: 8),
        Text(
          'Rp ${AppFormatters.formatCurrency(amount)}',
          style: TextStyle(
            fontSize: isBold ? 16 : 14,
            fontWeight: isBold ? FontWeight.bold : FontWeight.w600,
            color: isDark ? AppColors.neutralGray200 : AppColors.neutralGray900,
          ),
        ),
      ],
    );
  }

  Future<void> _nextStep() async {
    if (_currentStep == 0) {
      final configAsync = ref.read(config.sellerUpgradeConfigProvider);
      final packageConfig = configAsync.asData?.value;
      if (packageConfig == null) {
        AppSnackBar.showError(
          context,
          'Seller package is still loading from backend.',
        );
        return;
      }
      if (!packageConfig.isEnabled) {
        AppSnackBar.showError(
          context,
          'Seller registration is currently disabled by backend config.',
        );
        return;
      }
      await _goToStep(_currentStep + 1);
      return;
    } else if (_currentStep == 1) {
      final valid = _accountFormKey.currentState?.validate() ?? false;
      if (!valid) {
        AppSnackBar.showError(context, 'Please complete the account fields');
        return;
      }

      if (_selectedSenderAddress == null) {
        setState(() {
          _senderAddressError = 'Sender address is required.';
        });
        AppSnackBar.showError(context, 'Sender address is required');
        return;
      }

      final saved = await _saveAccountPrerequisites();
      if (!saved || !mounted) return;
    } else if (_currentStep == 2) {
      final valid = _storeFormKey.currentState?.validate() ?? false;
      if (!valid || !_isStoreStepValid) {
        AppSnackBar.showError(context, 'Please complete the store information');
        return;
      }
    } else if (_currentStep == 3) {
      if (!_agreeToTerms) {
        AppSnackBar.showError(context, 'Please agree to the seller terms');
        return;
      }
    }

    await _goToStep(_currentStep + 1);
  }

  void _previousStep() {
    if (_currentStep > 0) {
      unawaited(_goToStep(_currentStep - 1));
    }
  }

  Future<void> _goToStep(int step) async {
    if (!mounted) return;

    setState(() => _currentStep = step);
    await _pageController.animateToPage(
      _currentStep,
      duration: const Duration(milliseconds: 300),
      curve: Curves.easeInOut,
    );
  }

  Future<bool> _saveAccountPrerequisites() async {
    final authState = ref.read(authControllerProvider);
    final sellerIdentity = ref.read(authenticatedUserProvider);
    final wizardMode = _wizardModeFrom(authState, sellerIdentity);
    final requestEpoch = _principalEpoch;
    final userId = _currentAuthenticatedUserId();
    if (userId == null) {
      AppSnackBar.showError(context, 'User not authenticated');
      return false;
    }

    final emailVerified = authState is AuthStateAuthenticated
        ? authState.user.isEmailVerified
        : false;
    if (!emailVerified) {
      await showBlockedActionGate(
        context,
        actionDescription: wizardMode == _SellerUpgradeWizardMode.registration
            ? 'menjadi penjual'
            : 'memperpanjang seller',
      );
      return false;
    }

    final senderAddress = _selectedSenderAddress?.fullAddress.trim();
    if (senderAddress == null || senderAddress.isEmpty) {
      if (mounted) {
        setState(() {
          _senderAddressError = 'Sender address is required.';
        });
        AppSnackBar.showError(context, 'Sender address is required');
      }
      return false;
    }

    final result = await ref
        .read(authRepositoryProvider)
        .updateProfile(
          username: _usernameController.text.trim(),
          bio: _bioController.text.trim(),
          phoneNumber: _phoneController.text.trim(),
        );

    if (result.isError) {
      if (mounted) {
        AppSnackBar.showError(
          context,
          'Failed to save account prerequisites: ${result.error ?? 'Unknown error'}',
        );
      }
      return false;
    }

    if (!_isCurrentPrincipalRequest(requestEpoch, userId)) {
      return false;
    }

    // PATCH: Dimatikan untuk mencegah router redirect ke Home saat Step 1 -> Step 2
    // unawaited(
    //   ref.read(authControllerProvider.notifier).forceRefreshAuthState(),
    // );
    ref.invalidate(profileStreamProvider(userId));
    if (mounted) {
      setState(() {
        _senderAddressError = null;
      });
    }
    return true;
  }

  void _handleStorePhotoUpload() {
    final firebaseUser = FirebaseAuth.instance.currentUser;
    if (firebaseUser == null) {
      AppSnackBar.showError(context, 'User not authenticated');
      return;
    }

    AvatarEditorWidget.showEditModal(
      context: context,
      userId: firebaseUser.uid,
      showAdvancedCropper: true,
      onAvatarUpdated: (localPath) async {
        if (localPath == null) {
          if (!mounted) return;
          setState(() {
            _selectedStorePhotoPath = null;
            _farmPhotoUrl = null;
            _isStorePhotoUploading = false;
          });
          return;
        }

        if (!mounted) return;
        setState(() {
          _selectedStorePhotoPath = localPath;
          _farmPhotoUrl = null;
          _isStorePhotoUploading = true;
        });

        AppSnackBar.showInfo(context, 'Uploading store logo...');

        try {
          final result = await ref
              .read(storePhotoUploadServiceProvider)
              .uploadStorePhoto(userId: firebaseUser.uid, imagePath: localPath);

          if (!mounted) return;

          if (result.isSuccess && result.data != null) {
            setState(() {
              _selectedStorePhotoPath = localPath;
              _farmPhotoUrl = result.data!;
              _isStorePhotoUploading = false;
            });
            AppSnackBar.showSuccess(
              context,
              'Store logo uploaded successfully',
            );
          } else {
            setState(() {
              _selectedStorePhotoPath = null;
              _farmPhotoUrl = null;
              _isStorePhotoUploading = false;
            });
            AppSnackBar.showError(context, result.error ?? 'Upload failed');
          }
        } catch (e) {
          if (!mounted) return;
          setState(() {
            _selectedStorePhotoPath = null;
            _farmPhotoUrl = null;
            _isStorePhotoUploading = false;
          });
          AppSnackBar.showError(context, 'Gagal mengunggah logo. Coba lagi.');
        }
      },
    );
  }

  Future<void> _submitUpgrade() async {
    if (_isSubmitting) {
      return;
    }
    if (!_canSubmit) {
      AppSnackBar.showError(context, 'Please complete the onboarding steps');
      return;
    }

    final authState = ref.read(authControllerProvider);
    final authenticatedUser = ref.read(authenticatedUserProvider);
    final wizardMode = _wizardModeFrom(authState, authenticatedUser);
    if (wizardMode != _SellerUpgradeWizardMode.registration &&
        wizardMode != _SellerUpgradeWizardMode.renewal) {
      AppSnackBar.showError(context, 'Seller account is not ready yet');
      return;
    }

    final configAsync = ref.read(config.sellerUpgradeConfigProvider);
    final upgradeConfig = configAsync.asData?.value;
    if (upgradeConfig == null) {
      AppSnackBar.showError(
        context,
        'Seller package is still loading from backend.',
      );
      return;
    }
    if (!upgradeConfig.isEnabled) {
      AppSnackBar.showError(
        context,
        'Seller registration is currently disabled by backend config.',
      );
      return;
    }

    if (!mounted) return;
    setState(() => _isSubmitting = true);
    try {
      await _proceedToPayment(upgradeConfig, wizardMode);
    } finally {
      if (mounted) {
        setState(() => _isSubmitting = false);
      }
    }
  }

  Future<void> _proceedToPayment(
    SellerUpgradeConfigEntity upgradeConfig,
    _SellerUpgradeWizardMode wizardMode,
  ) async {
    if (!mounted) return;

    if (!upgradeConfig.isEnabled) {
      AppSnackBar.showError(
        context,
        'Seller registration is currently disabled by backend config.',
      );
      return;
    }

    showDialog(
      context: context,
      barrierDismissible: false,
      builder: (context) => const Center(child: CircularProgressIndicator()),
    );

    try {
      final requestEpoch = _principalEpoch;
      final userId = _currentAuthenticatedUserId();
      if (userId == null) {
        if (mounted && Navigator.of(context).canPop()) {
          Navigator.of(context).pop();
        }
        AppSnackBar.showError(context, 'User not authenticated');
        return;
      }

      final operationContext = _SellerPaymentOperationContext(
        initiatingUserId: userId,
        requestEpoch: requestEpoch,
        baselineSubscription: wizardMode == _SellerUpgradeWizardMode.renewal
            ? await _loadBaselineSellerSubscription(userId)
            : null,
      );

      if (wizardMode == _SellerUpgradeWizardMode.registration) {
        await ref
            .read(sellerRemoteDatasourceProvider)
            .performOnboarding(_farmNameController.text.trim());
      }

      if (!_isCurrentPrincipalRequest(requestEpoch, userId)) {
        if (mounted && Navigator.of(context).canPop()) {
          Navigator.of(context).pop();
        }
        return;
      }

      final paymentData = await ref
          .read(sellerRemoteDatasourceProvider)
          .initiateSubscriptionPayment();

      if (mounted && Navigator.of(context).canPop()) {
        Navigator.of(context).pop();
      }

      final paymentUrl = paymentData['payment_url'] as String?;
      if (paymentUrl == null || paymentUrl.isEmpty) {
        _showError('Gagal mendapatkan URL pembayaran');
        return;
      }

      final uri = Uri.parse(paymentUrl);
      if (!await launchUrl(uri, mode: LaunchMode.externalApplication)) {
        _showError('Gagal membuka halaman pembayaran');
        return;
      }

      if (!mounted) return;
      await _showPaymentPendingDialog(
        wizardMode: wizardMode,
        operationContext: operationContext,
      );
    } on ApiException catch (e) {
      if (mounted && Navigator.of(context).canPop()) {
        Navigator.of(context).pop();
      }
      if (!mounted) return;
      await _handleSubscriptionApiException(e, wizardMode: wizardMode);
    } catch (e) {
      if (mounted && Navigator.of(context).canPop()) {
        Navigator.of(context).pop();
      }
      if (!mounted) return;
      AppSnackBar.showError(context, 'Gagal memproses pembayaran. Coba lagi.');
    }
  }

  void _showError(String message) {
    if (!mounted) return;
    AppSnackBar.showError(context, message);
  }

  Future<void> _showPaymentPendingDialog({
    required _SellerUpgradeWizardMode wizardMode,
    required _SellerPaymentOperationContext operationContext,
  }) async {
    await showDialog<void>(
      context: context,
      barrierDismissible: false,
      builder: (dialogCtx) => _PaymentPendingDialog(
        wizardMode: wizardMode,
        operationContext: operationContext,
        isCurrentOperationPrincipal: () => _isCurrentPrincipalRequest(
          operationContext.requestEpoch,
          operationContext.initiatingUserId,
        ),
        onSuccess: () async {
          if (Navigator.of(dialogCtx).canPop()) {
            Navigator.of(dialogCtx).pop();
          }

          if (mounted) {
            AppSnackBar.showSuccess(
              context,
              wizardMode == _SellerUpgradeWizardMode.registration
                  ? 'Selamat! Anda sekarang penjual'
                  : 'Selamat! Perpanjangan seller berhasil diproses',
            );
            Navigator.of(context).pop(true);
          }
        },
      ),
    );
  }

  Future<void> _handleSubscriptionApiException(
    ApiException e, {
    required _SellerUpgradeWizardMode wizardMode,
  }) async {
    switch (e.code) {
      case 'NO_ACTIVE_CONFIG':
        AppSnackBar.showError(
          context,
          'Konfigurasi langganan tidak tersedia. Hubungi admin.',
        );
        return;
      case 'TOO_EARLY_RENEWAL':
        AppSnackBar.showError(
          context,
          'Langganan masih aktif. Perpanjangan tersedia mendekati kedaluwarsa.',
        );
        return;
      case 'MISSING_REQUIREMENTS':
        final missing = _extractMissingRequirements(e.details);
        await _showMissingRequirementsDialog(missing);
        return;
      case 'EMAIL_VERIFICATION_REQUIRED':
        await showBlockedActionGate(
          context,
          actionDescription: wizardMode == _SellerUpgradeWizardMode.registration
              ? 'menjadi penjual'
              : 'memperpanjang seller',
        );
        return;
      case 'ACCOUNT_SUSPENDED':
        await _showAccountBlockedDialog(
          title: 'Akun Ditangguhkan',
          message:
              'Akun Anda sedang ditangguhkan. Hubungi tim dukungan untuk informasi lebih lanjut.',
        );
        return;
      case 'ACCOUNT_BANNED':
        await _showAccountBlockedDialog(
          title: 'Akun Diblokir',
          message:
              'Akun Anda diblokir dan tidak dapat mengakses seller features.',
        );
        return;
      default:
        AppSnackBar.showError(context, e.message);
    }
  }

  List<String> _extractMissingRequirements(dynamic details) {
    if (details is Map<String, dynamic>) {
      final raw =
          details['missing_requirements'] ?? details['requires_verification'];
      if (raw is List) {
        return raw.map((item) => item.toString()).toList();
      }
    }
    return const [];
  }

  Future<void> _showMissingRequirementsDialog(List<String> missing) async {
    final labels = missing.isEmpty
        ? const ['account prerequisites']
        : missing.map(_formatRequirement).toList();

    await showDialog<void>(
      context: context,
      builder: (dialogCtx) => AlertDialog(
        title: const Text('Complete Seller Prerequisites'),
        content: Text(
          'Please finish these fields before payment:\n${labels.map((item) => '- $item').join('\n')}',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogCtx).pop(),
            child: const Text('OK'),
          ),
        ],
      ),
    );
  }

  String _formatRequirement(String requirement) {
    switch (requirement) {
      case 'email_verified':
        return 'Email verified';
      case 'username':
        return 'Username';
      case 'bio':
        return 'Bio';
      case 'phone_number':
        return 'Phone number';
      case 'sender_address':
      case 'location':
        return 'Structured sender address';
      case 'seller_profile':
        return 'Seller profile';
      default:
        return requirement.replaceAll('_', ' ');
    }
  }

  Future<void> _showAccountBlockedDialog({
    required String title,
    required String message,
  }) async {
    await showDialog<void>(
      context: context,
      builder: (dialogCtx) => AlertDialog(
        title: Text(title),
        content: Text(message),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogCtx).pop(),
            child: const Text('OK'),
          ),
        ],
      ),
    );
  }
}

class _PaymentPendingDialog extends ConsumerStatefulWidget {
  final _SellerUpgradeWizardMode wizardMode;
  final _SellerPaymentOperationContext operationContext;
  final bool Function() isCurrentOperationPrincipal;
  final Future<void> Function() onSuccess;

  const _PaymentPendingDialog({
    required this.wizardMode,
    required this.operationContext,
    required this.isCurrentOperationPrincipal,
    required this.onSuccess,
  });

  @override
  ConsumerState<_PaymentPendingDialog> createState() =>
      _PaymentPendingDialogState();
}

class _PaymentPendingDialogState extends ConsumerState<_PaymentPendingDialog> {
  Timer? _timer;
  bool _timedOut = false;
  int _attempts = 0;

  @override
  void initState() {
    super.initState();
    _startPolling();
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  Future<void> _startPolling() async {
    _timer = Timer.periodic(const Duration(seconds: 3), (timer) async {
      if (!mounted) {
        timer.cancel();
        return;
      }

      if (_attempts >= 20) {
        timer.cancel();
        setState(() => _timedOut = true);
        return;
      }

      if (!widget.isCurrentOperationPrincipal()) {
        timer.cancel();
        if (mounted && Navigator.of(context).canPop()) {
          Navigator.of(context).pop();
        }
        return;
      }

      _attempts++;
      await ref.read(authControllerProvider.notifier).forceRefreshAuthState();
      if (!mounted) {
        timer.cancel();
        return;
      }

      if (!widget.isCurrentOperationPrincipal()) {
        timer.cancel();
        if (mounted && Navigator.of(context).canPop()) {
          Navigator.of(context).pop();
        }
        return;
      }

      final authState = ref.read(authControllerProvider);
      if (authState is AuthStateAuthenticated &&
          authState.user.hasSellerProfile == true &&
          authState.user.hasMarketAuthority == true) {
        if (widget.wizardMode == _SellerUpgradeWizardMode.registration) {
          timer.cancel();
          await widget.onSuccess();
          return;
        }

        final baselineSubscription = widget.operationContext.baselineSubscription;
        if (baselineSubscription == null) {
          return;
        }

        final currentSubscription = await _refreshSubscriptionSnapshot();
        if (!mounted) {
          timer.cancel();
          return;
        }

        if (!widget.isCurrentOperationPrincipal()) {
          timer.cancel();
          if (mounted && Navigator.of(context).canPop()) {
            Navigator.of(context).pop();
          }
          return;
        }

        if (currentSubscription != null &&
            currentSubscription.expiryDate.isAfter(
              baselineSubscription.expiryDate,
            )) {
          timer.cancel();
          await widget.onSuccess();
        }
      }
    });
  }

  Future<SellerSubscription?> _refreshSubscriptionSnapshot() async {
    try {
      final SellerRepository repository = ref.read(sellerRepositoryProvider);
      final result = await repository.getSubscription(
        widget.operationContext.initiatingUserId,
      );
      if (result.isSuccess && result.data != null) {
        return result.data;
      }
    } catch (_) {
      // Transient refresh failure must not fabricate success.
    }
    return null;
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Processing payment'),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const LinearProgressIndicator(),
          const SizedBox(height: 16),
          Text(
            _timedOut
                ? 'Payment is still being processed. You can close this dialog and check again later.'
                : 'We are waiting for payment confirmation and seller activation.',
          ),
        ],
      ),
      actions: [
        if (_timedOut)
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('Close'),
          ),
      ],
    );
  }
}


