import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/config/seller_upgrade_config_entity.dart';
import 'package:labuda/core/config/seller_upgrade_config_provider.dart'
    as config;
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/finance/transaction/payment/domain/entities/payment.dart'
    show PaymentMethodOption;
import 'package:labuda/domains/finance/transaction/payment/presentation/providers/payment_providers.dart'
    show paymentRemoteDatasourceProvider;
import 'package:labuda/domains/finance/transaction/payment/presentation/widgets/payment_method_picker_sheet.dart';
import 'package:labuda/domains/user/preference/seller/data/dto/seller_dto.dart';
import 'package:labuda/domains/user/preference/seller/data/seller_providers.dart'
    show sellerRemoteDatasourceProvider, storePhotoUploadServiceProvider;
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_state.dart';
import 'package:labuda/domains/user/preference/seller/presentation/widgets/wizard/seller_wizard_helpers.dart';
import 'package:labuda/domains/user/preference/seller/presentation/widgets/wizard/seller_wizard_navigation_buttons.dart';
import 'package:labuda/domains/user/preference/seller/presentation/widgets/wizard/seller_wizard_preview_widget.dart';
import 'package:labuda/domains/user/preference/seller/presentation/widgets/wizard/seller_wizard_step2_widget.dart';
import 'package:labuda/domains/user/profile/profile.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/helpers/canonical_phone_validator.dart';
import 'package:labuda/shared/helpers/canonical_username_validator.dart';
import 'package:go_router/go_router.dart';

/// Seller Upgrade Wizard
///
/// **REGISTRATION LIFECYCLE ONLY** — for users who are NOT yet sellers.
/// Renewal is a separate payment-only lifecycle owned by `SellerRenewalScreen`
/// (`/seller/renewal`). This wizard never runs onboarding, seller profile
/// mutation, or registration terms for an existing seller: it fails closed
/// with the `existingSeller` gate.
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
  existingSeller,
  restricted,
}

class _SellerPaymentOperationContext {
  final String initiatingUserId;
  final int requestEpoch;

  /// Payment id returned by the initiate call (POST /seller/subscription/initiate).
  /// Non-null once the Snap session exists; drives the on-demand status sync.
  final String? paymentId;

  const _SellerPaymentOperationContext({
    required this.initiatingUserId,
    required this.requestEpoch,
    this.paymentId,
  });

  _SellerPaymentOperationContext withPaymentId(String paymentId) {
    return _SellerPaymentOperationContext(
      initiatingUserId: initiatingUserId,
      requestEpoch: requestEpoch,
      paymentId: paymentId,
    );
  }
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
  final _phoneController = TextEditingController();
  final _farmNameController = TextEditingController();

  String _initialUsername = '';
  String _initialPhone = '';
  String? _initialPrimaryAddressId;
  String _initialFarmName = '';
  String? _initialFarmPhotoDisplayUrl;
  String? _initialSelectedStorePhotoPath;
  final bool _initialAgreeToTerms = false;

  /// The account's primary address. Every product (and the seller store)
  /// uses this as its origin — there is no separate sender address.
  AddressEntity? _primaryAddress;
  String? _primaryAddressError;
  bool _isLoadingPrimaryAddress = false;

  // Canonical store photo state. The STORAGE KEY (images/stores/{user_id}.jpg)
  // is the ONLY value persisted to the backend (POST /seller/onboarding
  // store_image_url); the display URL is the canonical read_url and is used
  // for wizard preview rendering only. Never conflate the two.
  String? _farmPhotoStorageKey;
  String? _farmPhotoDisplayUrl;
  String? _selectedStorePhotoPath;
  bool _isStorePhotoUploading = false;
  bool _agreeToTerms = false;
  bool _isSubmitting = false;

  // PMF-02: the seller must explicitly choose a payment method, and the backend
  // is the sole fee authority. The methods payload carries the canonical
  // principal A plus, per method, the fee F and the gross A + F.
  SellerSubscriptionPaymentMethodsDto? _subscriptionPaymentMethods;
  SellerSubscriptionPaymentMethodDto? _selectedSubscriptionMethod;
  bool _isLoadingSubscriptionMethods = false;
  String? _subscriptionMethodsError;

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
      _phoneController,
      _farmNameController,
    ]) {
      controller.addListener(_markDirty);
    }
  }

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
          ? _SellerUpgradeWizardMode.existingSeller
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
      _phoneController.clear();
      _farmNameController.clear();
      _initialUsername = '';
      _initialPhone = '';
      _initialPrimaryAddressId = null;
      _initialFarmName = '';
      _initialFarmPhotoDisplayUrl = null;
      _initialSelectedStorePhotoPath = null;
      _primaryAddress = null;
      _primaryAddressError = null;
      _farmPhotoStorageKey = null;
      _farmPhotoDisplayUrl = null;
      _selectedStorePhotoPath = null;
      _agreeToTerms = false;
      _isLoadingPrimaryAddress = false;
      _isStorePhotoUploading = false;
      _isSubmitting = false;
    });

    if (_pageController.hasClients) {
      _pageController.jumpToPage(0);
    }
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
      _phoneController.text = user.phoneNumber ?? '';
      _initialUsername = _usernameController.text.trim();
      _initialPhone = _phoneController.text.trim();
    });

    _bindProfileListener(user.id);
    unawaited(_loadPrimaryAddress());
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
        _farmPhotoDisplayUrl ??= farm.farmPhotoUrl;
      }

      if (_initialFarmName.isEmpty &&
          _farmNameController.text.trim().isNotEmpty) {
        _initialFarmName = _farmNameController.text.trim();
      }
      _initialFarmPhotoDisplayUrl ??= _farmPhotoDisplayUrl;
    });
  }

  Future<void> _loadPrimaryAddress() async {
    final requestEpoch = _principalEpoch;
    final userId = _currentAuthenticatedUserId();
    if (userId == null) return;

    if (mounted) {
      setState(() {
        _isLoadingPrimaryAddress = true;
        _primaryAddressError = null;
      });
    }

    try {
      // The account's primary address. Every product uses it as its origin;
      // there is no separate sender address.
      final repository = ref.read(addressRepositoryProvider);
      final result = await repository.getPrimaryAddress(userId);

      if (!mounted || !_isCurrentPrincipalRequest(requestEpoch, userId)) {
        return;
      }

      result.fold(
        (error) {
          if (!_isCurrentPrincipalRequest(requestEpoch, userId)) return;
          setState(() {
            _primaryAddress = null;
            _primaryAddressError =
                'Tidak bisa memuat alamat utama. Coba lagi.';
          });
        },
        (primaryAddress) {
          if (!_isCurrentPrincipalRequest(requestEpoch, userId)) return;
          setState(() {
            _primaryAddress = primaryAddress;
            _primaryAddressError = primaryAddress == null
                ? 'Tambahkan alamat utama dulu untuk melanjutkan.'
                : null;
            if (primaryAddress != null) {
              _initialPrimaryAddressId ??= primaryAddress.id;
            }
          });
        },
      );
    } catch (e) {
      if (!mounted || !_isCurrentPrincipalRequest(requestEpoch, userId)) {
        return;
      }
      setState(() {
        _primaryAddress = null;
        _primaryAddressError = 'Tidak bisa memuat alamat utama. Coba lagi.';
      });
    } finally {
      if (mounted && _isCurrentPrincipalRequest(requestEpoch, userId)) {
        setState(() => _isLoadingPrimaryAddress = false);
      }
    }
  }

  Widget _buildReadOnlyStatusCard({
    required String title,
    required IconData icon,
    required String value,
    String? note,
  }) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: Theme.of(context).colorScheme.outlineVariant),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(
            icon,
            size: AppIconSize.action,
            color: Theme.of(context).colorScheme.onSurfaceVariant,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: context.typeRoles.bodyDense.copyWith(
                    fontWeight: FontWeight.w600,
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  value,
                  style: context.typeRoles.bodyDense.copyWith(
                    color: Theme.of(context).colorScheme.onSurface,
                  ),
                ),
                if (note != null) ...[
                  const SizedBox(height: 4),
                  Text(
                    note,
                    style: context.typeRoles.labelMicro.copyWith(
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
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

  Widget _buildPrimaryAddressSection() {
    final address = _primaryAddress;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                'Alamat Utama *',
                style: context.typeRoles.bodyDense.copyWith(
                  fontWeight: FontWeight.w600,
                  color: Theme.of(context).colorScheme.onSurface,
                ),
              ),
            ),
            TextButton.icon(
              onPressed: _isLoadingPrimaryAddress
                  ? null
                  : _showPrimaryAddressDialog,
              icon: Icon(
                address == null ? Icons.add_location_alt_outlined : Icons.edit,
                size: AppIconSize.action,
              ),
              label: Text(address == null ? 'Tambah alamat' : 'Edit'),
            ),
          ],
        ),
        const SizedBox(height: 12),
        if (_isLoadingPrimaryAddress)
          const LinearProgressIndicator(minHeight: 2),
        if (_isLoadingPrimaryAddress) const SizedBox(height: 12),
        if (address != null)
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(AppMetrics.p16),
            decoration: BoxDecoration(
              color: Theme.of(context).colorScheme.surface,
              borderRadius: BorderRadius.circular(AppShape.r12),
              border: Border.all(
                color: Theme.of(context).colorScheme.outlineVariant,
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
                      size: AppIconSize.action,
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Text(
                        (address.nickname?.trim().isNotEmpty ?? false)
                            ? address.nickname!.trim()
                            : address.recipientName,
                        style: context.typeRoles.bodyDense.copyWith(
                          fontWeight: FontWeight.w600,
                          color: Theme.of(context).colorScheme.onSurface,
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 12),
                Text(
                  'Recipient: ${address.recipientName}',
                  style: context.typeRoles.bodyDense.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  'Phone: ${address.phone}',
                  style: context.typeRoles.bodyDense.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 4),
                AddressLocationText(
                  location: address.fullAddress,
                  mode: AddressLocationMode.detail,
                  style: context.typeRoles.bodyDense.copyWith(
                    height: 1.5,
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          )
        else
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(AppMetrics.p16),
            decoration: BoxDecoration(
              color: Theme.of(context).colorScheme.surfaceContainerLow,
              borderRadius: BorderRadius.circular(AppShape.r12),
              border: Border.all(
                color: Theme.of(context).colorScheme.outlineVariant,
              ),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Belum ada alamat utama.',
                  style: context.typeRoles.bodyDense.copyWith(
                    fontWeight: FontWeight.w600,
                    color: Theme.of(context).colorScheme.onSurface,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  'Tambahkan alamat utama (provinsi, kota, kecamatan, kelurahan, alamat jalan, dan kode pos) untuk melanjutkan.',
                  style: context.typeRoles.labelMicro.copyWith(
                    height: 1.4,
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ),
        if (_primaryAddressError != null) ...[
          const SizedBox(height: 8),
          Text(
            _primaryAddressError!,
            style: context.typeRoles.labelMicro.copyWith(
              color: context.statusColors.error,
            ),
          ),
        ],
      ],
    );
  }

  Future<void> _showPrimaryAddressDialog() async {
    final userId = _currentAuthenticatedUserId();
    if (userId == null) {
      ref.read(navigationHandlerProvider).navigateToSignIn();
      return;
    }

    FocusManager.instance.primaryFocus?.unfocus();

    final saved = await AppBottomSheetBase.show<bool>(
      context: context,
      content: AddressFormDialog(addressToEdit: _primaryAddress),
    );

    if (saved == true) {
      await _loadPrimaryAddress();
    }
  }

  @override
  void dispose() {
    _authSubscription?.close();
    _profileSubscription?.close();
    for (final controller in [
      _usernameController,
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
    return SellerWizardHelpers.isAccountStepValid(
      username: _usernameController.text.trim(),
      phoneNumber: _phoneController.text.trim(),
      primaryAddress: _primaryAddress?.fullAddress.trim() ?? '',
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
        _phoneController.text.trim() != _initialPhone ||
        _primaryAddress?.id != _initialPrimaryAddressId ||
        _farmNameController.text.trim() != _initialFarmName ||
        _farmPhotoDisplayUrl != _initialFarmPhotoDisplayUrl ||
        _selectedStorePhotoPath != _initialSelectedStorePhotoPath ||
        _agreeToTerms != _initialAgreeToTerms;
  }

  @override
  Widget build(BuildContext context) {
    final configAsync = ref.watch(config.sellerUpgradeConfigProvider);
    final packageConfig = configAsync.asData?.value;
    final packageStepWidget = configAsync.when(
      data: (data) => _buildPackageDisclosureStep(data),
      loading: () => _buildPackageLoadingStep(),
      error: (error, _) => _buildPackageErrorStep(error.toString()),
    );
    final authState = ref.watch(authControllerProvider);
    final authenticatedUser = ref.watch(authenticatedUserProvider);
    final wizardMode = _wizardModeFrom(authState, authenticatedUser);
    // Canonical seller lifecycle (RF-02): identity (hasSellerProfile) alone
    // must NOT imply the seller ever paid. Expiry / renewal copy is owned by
    // sellerSubscriptionStatus via SellerState — pendingActivation vs expired.
    final sellerState = SellerState.fromAuthUser(authenticatedUser);
    final isEmailVerified = authState is AuthStateAuthenticated
        ? authState.user.isEmailVerified
        : false;
    final previewStepWidget = packageConfig == null
        ? _buildPackagePendingStep(
            'Seller package must load before you can preview the onboarding summary.',
          )
        : SellerWizardPreviewWidget(
            username: _usernameController.text.trim(),
            phoneNumber: _phoneController.text.trim(),
            primaryAddress: _primaryAddress?.fullAddress.trim() ?? '',
            emailVerified: isEmailVerified,
            farmName: _farmNameController.text.trim(),
            farmPhotoUrl: _farmPhotoDisplayUrl,
            selectedStorePhotoPath: _selectedStorePhotoPath,
            isStorePhotoUploading: _isStorePhotoUploading,
            packageFee: packageConfig.yearlyFee,
            packageDurationDays: packageConfig.durationDays,
            agreeToTerms: _agreeToTerms,
            onAgreeToTermsChanged: (value) =>
                setState(() => _agreeToTerms = value),
          );
    final paymentStepWidget = packageConfig == null
        ? _buildPackagePendingStep(
            'Seller package must load before payment can continue.',
          )
        : _buildPaymentStep(packageConfig);
    final canAdvanceFromPackage =
        packageConfig != null && packageConfig.isEnabled;
    // Registration wizard is only for first-time sellers; renewal uses SellerRenewalScreen.
    final isOperationalMode =
        wizardMode == _SellerUpgradeWizardMode.registration;

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
            _SellerUpgradeWizardMode.existingSeller => 'Seller Upgrade',
            _SellerUpgradeWizardMode.restricted => 'Akun Dibatasi',
            _SellerUpgradeWizardMode.unauthenticated => 'Login Diperlukan',
            _SellerUpgradeWizardMode.unhydrated => 'Memuat Seller',
          },
          leading: IconButton(
            icon: const Icon(Icons.close, semanticLabel: 'Tutup'),
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
          // Canonical Page Info trigger: the page owns the action, the surface
          // is the shared AppDialog.info authority.
          actions: [
            IconButton(
              icon: const Icon(Icons.help_outline),
              tooltip: 'Bantuan',
              onPressed: _showSellerInfo,
            ),
          ],
        ),
        body: isOperationalMode
            ? Column(
                children: [
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
                          isEmailVerified,
                          authState is AuthStateAuthenticated
                              ? authState.user.email
                              : '',
                        ),
                        SellerWizardStep2Widget(
                          formKey: _storeFormKey,
                          farmNameController: _farmNameController,
                          onStorePhotoUpload: _handleStorePhotoUpload,
                          farmPhotoUrl: _farmPhotoDisplayUrl,
                          selectedStorePhotoPath: _selectedStorePhotoPath,
                          isStorePhotoUploading: _isStorePhotoUploading,
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
                      4 =>
                        _canSubmit &&
                            canAdvanceFromPackage &&
                            _selectedSubscriptionMethod != null,
                      _ => false,
                    },
                    canSubmit: _canSubmit && canAdvanceFromPackage,
                    onPrevious: _previousStep,
                    onNext: _nextStep,
                    onSubmit: _submitUpgrade,
                  ),
                ],
              )
            : _buildWizardGate(
                icon: switch (wizardMode) {
                  _SellerUpgradeWizardMode.restricted => Icons.block,
                  _SellerUpgradeWizardMode.unauthenticated => Icons.login,
                  _SellerUpgradeWizardMode.existingSeller => Icons.storefront,
                  _SellerUpgradeWizardMode.unhydrated => Icons.hourglass_bottom,
                  _SellerUpgradeWizardMode.registration =>
                    Icons.hourglass_bottom,
                },
                title: switch (wizardMode) {
                  _SellerUpgradeWizardMode.restricted => 'Akun dibatasi',
                  _SellerUpgradeWizardMode.unauthenticated =>
                    'Login diperlukan',
                  _SellerUpgradeWizardMode.unhydrated =>
                    'Seller account is loading',
                  _SellerUpgradeWizardMode.existingSeller =>
                    sellerState.isExpired
                    ? 'Sudah Menjadi Seller'
                    : sellerState.isPendingActivation
                    ? 'Seller Terdaftar'
                    : 'Sudah Menjadi Seller',
                  _SellerUpgradeWizardMode.registration => 'Seller upgrade',
                },
                message: switch (wizardMode) {
                  _SellerUpgradeWizardMode.restricted =>
                    'This seller account is restricted and cannot continue here.',
                  _SellerUpgradeWizardMode.unauthenticated =>
                    'Sign in to continue with seller registration.',
                  _SellerUpgradeWizardMode.unhydrated =>
                    'Waiting for the current authenticated principal to hydrate before seller actions are enabled.',
                  _SellerUpgradeWizardMode.existingSeller =>
                    sellerState.isExpired
                    ? 'Wizard ini hanya untuk registrasi seller baru. Perpanjangan langganan adalah lifecycle pembayaran terpisah di layar Perpanjang Seller.'
                    : sellerState.isPendingActivation
                    ? 'Wizard ini hanya untuk registrasi seller baru. Aktivasi langganan adalah lifecycle pembayaran terpisah di layar Aktifkan Seller.'
                    : 'Wizard ini hanya untuk registrasi seller baru. Akun Anda sudah menjadi seller aktif.',
                  _SellerUpgradeWizardMode.registration =>
                    'Seller content is available only after the current account is operational.',
                },
                // Existing sellers cannot re-register. The payment-only
                // lifecycle hand-off uses canonical SellerState copy:
                // pendingActivation → Aktifkan; expired → Perpanjang.
                // Active sellers get no renewal CTA.
                actionLabel:
                    wizardMode == _SellerUpgradeWizardMode.existingSeller
                    ? sellerState.isExpired
                    ? 'Buka Perpanjang Seller'
                    : sellerState.isPendingActivation
                    ? 'Buka Aktifkan Seller'
                    : null
                    : null,
                onAction: wizardMode == _SellerUpgradeWizardMode.existingSeller
                    ? (sellerState.isExpired || sellerState.isPendingActivation)
                    ? () => context.push(RoutePaths.sellerRenewal)
                    : null
                    : null,
              ),
      ),
    );
  }

  Widget _buildWizardGate({
    required IconData icon,
    required String title,
    required String message,
    String? actionLabel,
    VoidCallback? onAction,
  }) {
    return Center(
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(AppMetrics.p24),
        child: Container(
          width: double.infinity,
          padding: const EdgeInsets.all(AppMetrics.p24),
          margin: const EdgeInsets.symmetric(horizontal: AppMetrics.p8),
          decoration: BoxDecoration(
            color: Theme.of(context).colorScheme.surface,
            borderRadius: BorderRadius.circular(AppShape.r16),
            border: Border.all(
              color: Theme.of(context).colorScheme.outlineVariant,
            ),
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                icon,
                size: AppIconSize.display,
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
              const SizedBox(height: 16),
              Text(
                title,
                textAlign: TextAlign.center,
                style: context.typeRoles.titleSection.copyWith(
                  fontWeight: FontWeight.bold,
                  color: Theme.of(context).colorScheme.onSurface,
                ),
              ),
              const SizedBox(height: 12),
              Text(
                message,
                textAlign: TextAlign.center,
                style: context.typeRoles.bodyDense.copyWith(
                  height: 1.5,
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
              ),
              if (actionLabel != null && onAction != null) ...[
                const SizedBox(height: 20),
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
      ),
    );
  }

  Widget _buildAccountStep(bool isEmailVerified, String email) {
    return Form(
      key: _accountFormKey,
      child: ListView(
        padding: const EdgeInsets.all(AppMetrics.p24),
        children: [
          Text(
            'Lengkapi Akun Seller Baru',
            style: context.typeRoles.titleSection.copyWith(
              fontWeight: FontWeight.bold,
              color: Theme.of(context).colorScheme.onSurface,
            ),
          ),
          const SizedBox(height: 16),
          _buildReadOnlyStatusCard(
            title: 'Email',
            icon: Icons.email_outlined,
            value: email.isNotEmpty ? email : '-',
            note: isEmailVerified ? 'Verified' : 'Not verified',
          ),
          const SizedBox(height: 24),
          if (_usernameController.text.trim().isNotEmpty)
            _buildReadOnlyStatusCard(
              title: 'Username',
              icon: Icons.alternate_email,
              value: _usernameController.text.trim(),
            )
          else
            AppTextField(
              controller: _usernameController,
              labelText: 'Username *',
              hintText: 'your_username',
              prefixIcon: Icons.alternate_email,
              // Canonical username rule (same authority as sign-up /
              // complete-profile): no surface may accept a username the
              // backend identityusername contract would reject.
              validator: (value) =>
                  CanonicalUsernameValidator.normalizeAndValidate(value),
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
          _buildPrimaryAddressSection(),
        ],
      ),
    );
  }

  Widget _buildPackageDisclosureStep(SellerUpgradeConfigEntity upgradeConfig) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(AppMetrics.p16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Paket & Syarat Seller',
            style: context.typeRoles.titleProminent.copyWith(
              fontWeight: FontWeight.bold,
              color: Theme.of(context).colorScheme.onSurface,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Lihat biaya dan manfaat paket seller sebelum mengisi data akun.',
            style: context.typeRoles.bodyDense.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 24),
          _buildPaidPlanCard(upgradeConfig),
          if (!upgradeConfig.isEnabled) ...[
            const SizedBox(height: 16),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(AppMetrics.p16),
              decoration: BoxDecoration(
                color: context.statusColors.error.withValues(alpha: 0.08),
                borderRadius: BorderRadius.circular(AppShape.r12),
                border: Border.all(
                  color: context.statusColors.error.withValues(alpha: 0.2),
                ),
              ),
              child: Text(
                'Seller registration is currently disabled by backend config.',
                style: context.typeRoles.bodyDense.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
              ),
            ),
          ],
          const SizedBox(height: 24),
          _buildFeaturesList(),
        ],
      ),
    );
  }

  Widget _buildPaymentStep(SellerUpgradeConfigEntity upgradeConfig) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(AppMetrics.p16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Pembayaran',
            style: context.typeRoles.titleProminent.copyWith(
              fontWeight: FontWeight.bold,
              color: Theme.of(context).colorScheme.onSurface,
            ),
          ),
          const SizedBox(height: 16),
          _buildPaymentSection(upgradeConfig),
        ],
      ),
    );
  }

  Widget _buildPackageLoadingStep() {
    return _buildPackageStateCard(
      title: 'Paket & Syarat Seller',
      message: 'Mengambil fee seller dari backend...',
      leading: const CircularProgressIndicator(strokeWidth: 2),
    );
  }

  Widget _buildPackageErrorStep(String error) {
    return _buildPackageStateCard(
      title: 'Paket & Syarat Seller',
      message: 'Gagal memuat konfigurasi seller dari backend.\n$error',
      leading: Icon(
        Icons.error_outline,
        color: context.statusColors.error,
        size: AppIconSize.emphasis,
      ),
      actionLabel: 'Coba lagi',
      onAction: () => ref.invalidate(config.sellerUpgradeConfigProvider),
    );
  }

  Widget _buildPackagePendingStep(String message) {
    return _buildPackageStateCard(
      title: 'Paket & Syarat Seller',
      message: message,
      leading: Icon(
        Icons.info_outline,
        color: Theme.of(context).colorScheme.secondary,
        size: AppIconSize.emphasis,
      ),
    );
  }

  Widget _buildPackageStateCard({
    required String title,
    required String message,
    Widget? leading,
    String? actionLabel,
    VoidCallback? onAction,
  }) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(AppMetrics.p16),
      child: Container(
        width: double.infinity,
        padding: const EdgeInsets.all(AppMetrics.p24),
        decoration: BoxDecoration(
          color: Theme.of(context).colorScheme.surface,
          borderRadius: BorderRadius.circular(AppShape.r16),
          border: Border.all(
            color: Theme.of(context).colorScheme.outlineVariant,
          ),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              title,
              style: context.typeRoles.titleProminent.copyWith(
                fontWeight: FontWeight.bold,
                color: Theme.of(context).colorScheme.onSurface,
              ),
            ),
            const SizedBox(height: 16),
            if (leading != null) ...[leading, const SizedBox(height: 16)],
            Text(
              message,
              style: context.typeRoles.bodyDense.copyWith(
                height: 1.5,
                color: Theme.of(context).colorScheme.onSurfaceVariant,
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

  Widget _buildPaidPlanCard(SellerUpgradeConfigEntity upgradeConfig) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(AppMetrics.p24),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [
            Theme.of(context).colorScheme.secondary.withValues(alpha: 0.12),
            Theme.of(context).colorScheme.secondary.withValues(alpha: 0.06),
          ],
        ),
        borderRadius: BorderRadius.circular(AppShape.r16),
        border: Border.all(
          color: Theme.of(context).colorScheme.secondary.withValues(alpha: 0.5),
          width: 2,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            padding: const EdgeInsets.symmetric(
              horizontal: AppMetrics.p12,
              vertical: AppMetrics.p8,
            ),
            decoration: BoxDecoration(
              color: Theme.of(context).colorScheme.secondary,
              borderRadius: BorderRadius.circular(AppShape.r20),
            ),
            child: Text(
              'AKTIVASI SELLER',
              style: context.typeRoles.labelMicro.copyWith(
                color: Theme.of(context).colorScheme.onSecondary,
                fontWeight: FontWeight.bold,
              ),
            ),
          ),
          const SizedBox(height: 16),
          Text(
            'Aktivasi Seller',
            style: context.typeRoles.titleProminent.copyWith(
              fontWeight: FontWeight.bold,
              color: Theme.of(context).colorScheme.onSurface,
            ),
          ),
          const SizedBox(height: 8),
          Row(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              Text(
                AppFormatters.formatCurrency(upgradeConfig.yearlyFee),
                style: context.typeRoles.titleProminent.copyWith(
                  fontWeight: FontWeight.bold,
                  color: Theme.of(context).colorScheme.secondary,
                ),
              ),
              const SizedBox(width: 8),
              Padding(
                padding: const EdgeInsets.only(bottom: AppMetrics.p8),
                child: Text(
                  '/${upgradeConfig.durationDays} hari',
                  style: context.typeRoles.bodyDense.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildFeaturesList() {
    final features = [
      ('Buat For Sale', Icons.inventory_2_outlined),
      ('Buat Auction', Icons.gavel),
      ('Buat Promotion', Icons.campaign_outlined),
      ('Berlaku selama subscription aktif', Icons.verified_outlined),
    ];

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'What You Get',
          style: context.typeRoles.titleCompact.copyWith(
            fontWeight: FontWeight.bold,
            color: Theme.of(context).colorScheme.onSurface,
          ),
        ),
        const SizedBox(height: 16),
        ...features.map(
          (feature) => Padding(
            padding: const EdgeInsets.only(bottom: AppMetrics.p12),
            child: Row(
              children: [
                Container(
                  width: 24,
                  height: 24,
                  decoration: BoxDecoration(
                    color: context.statusColors.success.withValues(alpha: 0.1),
                    shape: BoxShape.circle,
                  ),
                  child: Icon(
                    Icons.check,
                    color: context.statusColors.success,
                    size: AppIconSize.inlineGlyph,
                  ),
                ),
                const SizedBox(width: 12),
                Icon(
                  feature.$2,
                  size: AppIconSize.action,
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    feature.$1,
                    style: context.typeRoles.bodyDense.copyWith(
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 4),
        // Business truth (Owner decision A): the Seller status does not expire;
        // only the For Sale / Auction / Promotion capability is unavailable
        // while the subscription is expired.
        Text(
          'Jika subscription berakhir, For Sale, Auction, dan Promotion tidak '
          'dapat digunakan, tetapi status Seller Anda tetap.',
          style: context.typeRoles.labelMicro.copyWith(
            color: Theme.of(context).colorScheme.onSurfaceVariant,
          ),
        ),
      ],
    );
  }

  Widget _buildPaymentSection(SellerUpgradeConfigEntity upgradeConfig) {
    // PMF-02: once the canonical methods payload is loaded it is the money
    // authority for the whole summary — principal A, fee F per method, and the
    // gross A + F. The config disclosure value is only a placeholder while the
    // methods are still loading.
    final methods = _subscriptionPaymentMethods;
    final availableMethods = methods?.methods ?? const [];
    final principalAmount =
        (methods?.principalAmount ?? upgradeConfig.yearlyFee.round())
            .toDouble();
    final selectedMethod = _selectedSubscriptionMethod;
    final feeAmount = (selectedMethod?.serviceFeeAmount ?? 0).toDouble();

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: Theme.of(context).colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Seller Payment Summary',
            style: context.typeRoles.titleCompact.copyWith(
              fontWeight: FontWeight.bold,
              color: Theme.of(context).colorScheme.onSurface,
            ),
          ),
          const SizedBox(height: 16),
          _buildPaymentRow('Yearly subscription', principalAmount),
          const SizedBox(height: 12),
          PaymentMethodTrigger(
            selectedMethodCode: selectedMethod?.methodCode,
            selectedMethodDisplayName: selectedMethod?.displayName,
            isLoading: _isLoadingSubscriptionMethods,
            hasMethods: availableMethods.isNotEmpty,
            errorMessage: _subscriptionMethodsError,
            onTap: availableMethods.isEmpty
                ? () => unawaited(_ensureSubscriptionPaymentMethodsLoaded())
                : () => unawaited(_selectSubscriptionPaymentMethod()),
            onRetry: () => unawaited(_ensureSubscriptionPaymentMethodsLoaded()),
          ),
          if (selectedMethod != null) ...[
            const SizedBox(height: 12),
            _buildPaymentRow('Payment method fee', feeAmount),
          ],
          const Divider(height: 24),
          _buildPaymentRowText(
            'Total',
            selectedMethod == null
                ? 'Belum dipilih'
                : AppFormatters.formatCurrency(
                    selectedMethod.grossAmount.toDouble(),
                  ),
            isBold: true,
          ),
          const SizedBox(height: 12),
          Container(
            padding: const EdgeInsets.all(AppMetrics.p12),
            decoration: BoxDecoration(
              color: context.statusColors.info.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(AppShape.r8),
            ),
            child: Row(
              children: [
                Icon(
                  Icons.info_outline,
                  size: AppIconSize.inlineGlyph,
                  color: Theme.of(context).colorScheme.secondary,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    'You will be redirected to the payment provider in a browser.',
                    style: context.typeRoles.labelMicro.copyWith(
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
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



  Widget _buildPaymentRowText(

    String label,
    String amountText, {
    bool isBold = false,
  }) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Expanded(
          child: Text(
            label,
            style: context.typeRoles.bodyDense.copyWith(
              fontWeight: isBold ? FontWeight.bold : FontWeight.normal,
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
        ),
        const SizedBox(width: 8),
        Text(
          amountText,
          style:
              (isBold
                      ? context.typeRoles.titleCompact
                      : context.typeRoles.bodyDense)
                  .copyWith(
                    fontWeight: isBold ? FontWeight.bold : FontWeight.w600,
                    color: Theme.of(context).colorScheme.onSurface,
                  ),
        ),
      ],
    );
  }

  Widget _buildPaymentRow(String label, double amount, {bool isBold = false}) {
    return _buildPaymentRowText(
      label,
      AppFormatters.formatCurrency(amount),
      isBold: isBold,
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
        AppSnackBar.showError(context, 'Lengkapi data akun');
        return;
      }

      if (_primaryAddress == null) {
        setState(() {
          _primaryAddressError = 'Alamat utama wajib diisi.';
        });
        AppSnackBar.showError(context, 'Alamat utama wajib diisi');
        return;
      }

      final saved = await _saveAccountPrerequisites();
      if (!saved || !mounted) return;
    } else if (_currentStep == 2) {
      final valid = _storeFormKey.currentState?.validate() ?? false;
      if (!valid || !_isStoreStepValid) {
        AppSnackBar.showError(context, 'Lengkapi informasi toko');
        return;
      }
    } else if (_currentStep == 3) {
      if (!_agreeToTerms) {
        AppSnackBar.showError(context, 'Setujui syarat penjual');
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
      duration: AppMotion.settled,
      curve: Curves.easeInOut,
    );

    if (step == _totalSteps - 1) {
      // PMF-02: the payment step cannot render without the canonical methods
      // (each carries the backend fee and gross), so load them on entry.
      unawaited(_ensureSubscriptionPaymentMethodsLoaded());
    }
  }

  Future<bool> _saveAccountPrerequisites() async {
    final requestEpoch = _principalEpoch;
    final userId = _currentAuthenticatedUserId();
    if (userId == null) {
      ref.read(navigationHandlerProvider).navigateToSignIn();
      return false;
    }

    // D2 HARD GATE (design scope v2): no client-side email-verification
    // preflight — every authenticated user is already verified. The backend
    // stays authoritative (EMAIL_VERIFICATION_REQUIRED handler on submit).

    final primaryAddress = _primaryAddress?.fullAddress.trim();
    if (primaryAddress == null || primaryAddress.isEmpty) {
      if (mounted) {
        setState(() {
          _primaryAddressError = 'Alamat utama wajib diisi.';
        });
        AppSnackBar.showError(context, 'Alamat utama wajib diisi');
      }
      return false;
    }

    final result = await ref
        .read(authRepositoryProvider)
        .updateProfile(
          username: _usernameController.text.trim(),
          phoneNumber: _phoneController.text.trim(),
        );

    if (result.isError) {
      if (mounted) {
        AppSnackBar.showError(
          context,
          'Gagal menyimpan prasyarat akun. Coba lagi.',
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
        _primaryAddressError = null;
      });
    }
    return true;
  }

  void _handleStorePhotoUpload() {
    // AUTHORITY: the store-photo owner is the Labuda user ID from the
    // canonical auth state. Backend fixed-key validation
    // (images/stores/{user_id}.jpg) checks ownership against the JWT user ID —
    // a Firebase UID would always be rejected with INVALID_STORAGE_KEY.
    final requestEpoch = _principalEpoch;
    final userId = _currentAuthenticatedUserId();
    if (userId == null) {
      ref.read(navigationHandlerProvider).navigateToSignIn();
      return;
    }

    AvatarEditorWidget.showEditModal(
      context: context,
      cropTitle: 'Potong Foto Toko',
      onAvatarUpdated: (localPath) async {
        if (localPath == null) {
          if (!_isCurrentPrincipalRequest(requestEpoch, userId)) return;
          setState(() {
            _selectedStorePhotoPath = null;
            _farmPhotoDisplayUrl = null;
            _farmPhotoStorageKey = null;
            _isStorePhotoUploading = false;
          });
          return;
        }

        if (!_isCurrentPrincipalRequest(requestEpoch, userId)) return;
        setState(() {
          _selectedStorePhotoPath = localPath;
          _farmPhotoDisplayUrl = null;
          _farmPhotoStorageKey = null;
          _isStorePhotoUploading = true;
        });

        AppSnackBar.showInfo(context, 'Mengunggah logo toko...');

        try {
          final result = await ref
              .read(storePhotoUploadServiceProvider)
              .uploadStorePhoto(userId: userId, imagePath: localPath);

          if (!mounted || !_isCurrentPrincipalRequest(requestEpoch, userId)) {
            return;
          }

          if (result.isSuccess && result.data != null) {
            setState(() {
              _selectedStorePhotoPath = localPath;
              // Persist the canonical STORAGE KEY via onboarding; the read
              // URL is display-only for the wizard preview.
              _farmPhotoStorageKey = result.data!.storageKey;
              _farmPhotoDisplayUrl = result.data!.displayUrl;
              _isStorePhotoUploading = false;
            });
            AppSnackBar.showSuccess(
              context,
              'Logo toko berhasil diunggah',
            );
          } else {
            if (!mounted || !_isCurrentPrincipalRequest(requestEpoch, userId)) {
              return;
            }
            setState(() {
              _selectedStorePhotoPath = null;
              _farmPhotoDisplayUrl = null;
              _farmPhotoStorageKey = null;
              _isStorePhotoUploading = false;
            });
            AppSnackBar.showError(context, result.error ?? 'Gagal mengunggah');
          }
        } catch (e) {
          if (!mounted || !_isCurrentPrincipalRequest(requestEpoch, userId)) {
            return;
          }
          setState(() {
            _selectedStorePhotoPath = null;
            _farmPhotoDisplayUrl = null;
            _farmPhotoStorageKey = null;
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
      AppSnackBar.showError(context, 'Lengkapi langkah onboarding');
      return;
    }

    // PMF-02: every payment flow carries a payment-method fee, so an explicit
    // method choice is a prerequisite, not a defaulted detail.
    if (_selectedSubscriptionMethod == null) {
      AppSnackBar.showError(context, 'Pilih metode pembayaran');
      return;
    }

    final authState = ref.read(authControllerProvider);
    final authenticatedUser = ref.read(authenticatedUserProvider);
    final wizardMode = _wizardModeFrom(authState, authenticatedUser);
    if (wizardMode != _SellerUpgradeWizardMode.registration) {
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
      await _proceedToPayment(upgradeConfig);
    } finally {
      if (mounted) {
        setState(() => _isSubmitting = false);
      }
    }
  }

  Future<void> _proceedToPayment(
    SellerUpgradeConfigEntity upgradeConfig,
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
        ref.read(navigationHandlerProvider).navigateToSignIn();
        return;
      }

      final operationContext = _SellerPaymentOperationContext(
        initiatingUserId: userId,
        requestEpoch: requestEpoch,
      );

      // SUBMISSION SNAPSHOT: capture the chosen subscription method BEFORE the
      // onboarding await so a step/selection change mid-flight cannot switch
      // the method that is actually paid.
      final selectedMethod = _selectedSubscriptionMethod;

      // Registration lifecycle: onboarding creates the seller profile before the
      // subscription payment is initiated. Renewal is payment-only and lives in
      // SellerRenewalScreen — it never reaches this code path.
      await ref
          .read(sellerRemoteDatasourceProvider)
          .performOnboarding(
            _farmNameController.text.trim(),
            storeImageUrl: _farmPhotoStorageKey,
          );

      if (!_isCurrentPrincipalRequest(requestEpoch, userId)) {
        if (mounted && Navigator.of(context).canPop()) {
          Navigator.of(context).pop();
        }
        return;
      }

      // PMF-02: the selected method code is the only payment input the client
      // sends; the backend resolves the method, validates it and calculates the
      // fee it will snapshot. A pending payment already exists → the backend
      // reuses its immutable snapshot (Owner decision) rather than superseding it.
      if (selectedMethod == null) {
        if (mounted && Navigator.of(context).canPop()) {
          Navigator.of(context).pop();
        }
        if (!mounted) return;
        AppSnackBar.showError(
          context,
          'Pilih metode pembayaran terlebih dahulu',
        );
        return;
      }

      final paymentData = await ref
          .read(sellerRemoteDatasourceProvider)
          .initiateSubscriptionPayment(
            paymentMethodCode: selectedMethod.methodCode,
          );

      if (mounted && Navigator.of(context).canPop()) {
        Navigator.of(context).pop();
      }

      final paymentUrl = paymentData['payment_url'] as String?;
      if (paymentUrl == null || paymentUrl.isEmpty) {
        _showError('Gagal mendapatkan URL pembayaran');
        return;
      }
      // Thread the canonical payment id into the awaiting flow so every
      // subsequent "Cek status" actively syncs gateway truth instead of only
      // re-reading the local auth snapshot.
      final paymentId = paymentData['payment_id']?.toString();
      final paymentOperationContext = paymentId == null || paymentId.isEmpty
          ? operationContext
          : operationContext.withPaymentId(paymentId);

      if (!mounted) return;
      // Payment URLs are presented exclusively inside Labuda's internal WebView.
      // External-browser payment navigation is obsolete and must not be reintroduced.
      await context.push(
        '/payment-webview?url=${Uri.encodeComponent(paymentUrl)}',
      );

      if (!mounted) return;
      await _showPaymentPendingDialog(
        operationContext: paymentOperationContext,
      );
    } on ApiException catch (e) {
      if (mounted && Navigator.of(context).canPop()) {
        Navigator.of(context).pop();
      }
      if (!mounted) return;
      await _handleSubscriptionApiException(e);
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

  /// PMF-02: loads the canonical enabled subscription payment methods for the
  /// payment step. Each option carries the backend-calculated fee F and the
  /// resulting gross A + F — this client never computes either value.
  ///
  /// Idempotent: safe to call on every entry to the payment step, and used as
  /// the retry path when the first load failed.
  Future<void> _ensureSubscriptionPaymentMethodsLoaded() async {
    // Called fire-and-forget from _goToStep, which may resolve after the wizard
    // has been torn down (e.g. the principal switched mid-flow).
    if (!mounted) return;
    if (_isLoadingSubscriptionMethods || _subscriptionPaymentMethods != null) {
      return;
    }

    setState(() {
      _isLoadingSubscriptionMethods = true;
      _subscriptionMethodsError = null;
    });

    try {
      final methods = await ref
          .read(sellerRemoteDatasourceProvider)
          .getSubscriptionPaymentMethods();
      if (!mounted) return;

      // Drop a selection whose method is no longer enabled at the backend.
      final current = _selectedSubscriptionMethod;
      final stillAvailable =
          current != null &&
          methods.methods.any((m) => m.methodCode == current.methodCode);

      setState(() {
        _subscriptionPaymentMethods = methods;
        _selectedSubscriptionMethod = stillAvailable ? current : null;
        _isLoadingSubscriptionMethods = false;
      });
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _isLoadingSubscriptionMethods = false;
        _subscriptionMethodsError = e.code == 'NO_ACTIVE_CONFIG'
            ? 'Konfigurasi langganan belum tersedia.'
            : 'Gagal memuat metode pembayaran. Coba lagi.';
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _isLoadingSubscriptionMethods = false;
        _subscriptionMethodsError =
            'Gagal memuat metode pembayaran. Coba lagi.';
      });
    }
  }

  /// Opens the canonical payment-method picker and records the seller's
  /// explicit choice. The picker renders backend-calculated fee and total only.
  Future<void> _selectSubscriptionPaymentMethod() async {
    final available = _subscriptionPaymentMethods?.methods ?? const [];
    if (available.isEmpty) return;

    final options = available
        .map(
          (m) => PaymentMethodOption(
            methodCode: m.methodCode,
            displayName: m.displayName,
            // The canonical option entity predates PMF-02 and names these
            // fields for the buyer checkout flow. For a subscription payment
            // they carry the seller subscription fee F and gross A + F exactly
            // as the backend calculated them.
            buyerPaymentFeeAmount: m.serviceFeeAmount,
            totalPayableAmount: m.grossAmount,
          ),
        )
        .toList();

    final selectedCode = await PaymentMethodPickerSheet.show(
      context,
      methods: options,
      selectedMethodCode: _selectedSubscriptionMethod?.methodCode,
    );
    if (!mounted || selectedCode == null) return;

    for (final method in available) {
      if (method.methodCode == selectedCode) {
        setState(() => _selectedSubscriptionMethod = method);
        return;
      }
    }
  }

  Future<void> _showPaymentPendingDialog({
    required _SellerPaymentOperationContext operationContext,
  }) async {
    // Batch 2 parity with SellerRenewalScreen: settlement can outlive the 60s
    // polling window (VA takes minutes-hours). After the user closes the
    // pending dialog, the wizard keeps offering a manual status check until
    // the backend confirms — the fresh seller is never stranded on step 5.
    //
    // successHandled breaks the loop the moment success was surfaced once —
    // onSuccess may pop this screen, and the loop must never re-run its body
    // after that (double snackbar / double pop).
    var successHandled = false;
    var firstIteration = true;
    bool isCurrentPrincipal() => _isCurrentPrincipalRequest(
      operationContext.requestEpoch,
      operationContext.initiatingUserId,
    );
    while (mounted && !successHandled && isCurrentPrincipal()) {
      if (firstIteration) {
        // Right after submit: the auto-polling window is the check.
        firstIteration = false;
      } else {
        // Manual re-entry: check backend truth FIRST ("Cek status" must
        // check, not reopen the polling dialog), then offer to poll again.
        final confirmed = await _checkRegistrationPaymentConfirmed(
          operationContext,
        );
        if (successHandled || !mounted || !isCurrentPrincipal()) return;
        if (confirmed) {
          successHandled = true;
          AppSnackBar.showSuccess(context, 'Selamat! Anda sekarang penjual');
          Navigator.of(context).pop(true);
          return;
        }
        // F9(a) convergence: pure recheck yes/no decision consumes the
        // canonical AppDialog.confirm grammar (same order and meaning).
        final recheck = await AppDialog.confirm(
          context: context,
          title: 'Pembayaran masih diproses',
          message:
              'Pembayaran Anda belum terkonfirmasi. Cek ulang statusnya sekarang?',
          confirmLabel: 'Cek status',
          cancelLabel: 'Nanti saja',
        );
        if (!recheck) return;
        // "Cek status" must CHECK, not reopen the polling dialog.
        continue;
      }

      await showDialog<void>(
        context: context,
        barrierDismissible: false,
        builder: (dialogCtx) => _PaymentPendingDialog(
          operationContext: operationContext,
          isCurrentOperationPrincipal: isCurrentPrincipal,
          onSuccess: () async {
            successHandled = true;
            if (Navigator.of(dialogCtx).canPop()) {
              Navigator.of(dialogCtx).pop();
            }

            if (mounted) {
              AppSnackBar.showSuccess(
                context,
                'Selamat! Anda sekarang penjual',
              );
              Navigator.of(context).pop(true);
            }
          },
        ),
      );
      if (successHandled || !mounted || !isCurrentPrincipal()) return;
    }
  }

  /// Manual status check for the Batch 2 re-entry loop. The registration
  /// success predicate mirrors the polling dialog: seller profile exists AND
  /// market authority is active (only ProcessSuccessfulPaymentTx writes the
  /// first active interval).
  Future<bool> _checkRegistrationPaymentConfirmed(
    _SellerPaymentOperationContext operationContext,
  ) async {
    // PAYMENT_SYNC_ON_DEMAND: "Cek status" must CHECK — sync the gateway
    // truth for the tracked payment first, then read the resulting snapshot.
    final paymentId = operationContext.paymentId;
    if (paymentId != null && paymentId.isNotEmpty) {
      try {
        await ref.read(paymentRemoteDatasourceProvider).syncPayment(paymentId);
      } catch (_) {
        // Best-effort; the auth re-read below remains the fallback truth.
      }
    }
    await ref.read(authControllerProvider.notifier).forceRefreshAuthState();
    if (!mounted) return false;
    if (!_isCurrentPrincipalRequest(
      operationContext.requestEpoch,
      operationContext.initiatingUserId,
    )) {
      return false;
    }
    final s = ref.read(authControllerProvider);
    return s is AuthStateAuthenticated &&
        s.user.hasSellerProfile == true &&
        s.user.hasMarketAuthority == true;
  }

  Future<void> _handleSubscriptionApiException(ApiException e) async {
    switch (e.code) {
      case 'NO_ACTIVE_CONFIG':
        AppSnackBar.showError(
          context,
          'Konfigurasi langganan tidak tersedia. Hubungi admin.',
        );
        return;
      case 'MISSING_REQUIREMENTS':
        final missing = _extractMissingRequirements(e.details);
        final labels = missing.isEmpty
            ? const ['prasyarat akun']
            : missing.map(_formatRequirement).toList();
        await AppDialog.info(
          context: context,
          title: 'Lengkapi Prasyarat Seller',
          message:
              'Selesaikan data berikut sebelum pembayaran:\n${labels.map((item) => '- $item').join('\n')}',
          closeLabel: 'OK',
        );
        return;
      case 'EMAIL_VERIFICATION_REQUIRED':
        // Backend-rejection handler (defense-in-depth): the backend stays
        // the single authority for EMAIL_VERIFICATION_REQUIRED.
        AppSnackBar.showError(
          context,
          'Verifikasi email kamu diperlukan sebelum menjadi penjual.',
        );
        return;
      case 'ACCOUNT_SUSPENDED':
        await AppDialog.info(
          context: context,
          title: 'Akun Ditangguhkan',
          message:
              'Akun Anda sedang ditangguhkan. Hubungi tim dukungan untuk informasi lebih lanjut.',
          closeLabel: 'OK',
        );
        return;
      case 'ACCOUNT_BANNED':
        await AppDialog.info(
          context: context,
          title: 'Akun Diblokir',
          message:
              'Akun Anda diblokir dan tidak dapat mengakses seller features.',
          closeLabel: 'OK',
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

  String _formatRequirement(String requirement) {
    switch (requirement) {
      case 'email_verified':
        return 'Email terverifikasi';
      case 'username':
        return 'Username';
      case 'phone_number':
        return 'Nomor telepon';
      case 'primary_address':
      case 'location':
        return 'Alamat utama';
      case 'seller_profile':
        return 'Profil seller';
      default:
        return requirement.replaceAll('_', ' ');
    }
  }

  /// Canonical Page Info / Help surface for the registration wizard.
  ///
  /// The page owns the trigger; the surface is the shared `AppDialog.info`.
  /// This consolidates the secondary explanations that used to occupy wizard
  /// steps (registration mode, KYC/payout, account guidance, payment and
  /// activation).
  void _showSellerInfo() {
    AppDialog.info(
      context: context,
      title: 'Tentang Daftar Seller',
      content: _buildSellerInfoContent(context),
      closeLabel: 'Tutup',
    );
  }

  Widget _buildSellerInfoContent(BuildContext context) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _buildSellerInfoSection(
          context,
          title: 'Pendaftaran Seller',
          body:
              'Wizard ini untuk pendaftaran seller baru. Setelah pembayaran '
              'dikonfirmasi, akun Anda aktif sebagai Seller. Perpanjangan '
              'langganan dilakukan terpisah dari pendaftaran ini.',
        ),
        const SizedBox(height: 16),
        _buildSellerInfoSection(
          context,
          title: 'Paket & Review',
          body:
              'Langganan seller berlaku 365 hari sejak pembayaran '
              'dikonfirmasi. Verifikasi KYC dan review bank dilakukan '
              'setelahnya dan hanya diperlukan untuk pencairan dana '
              '(withdrawal), bukan untuk mendaftar sebagai seller.',
        ),
        const SizedBox(height: 16),
        _buildSellerInfoSection(
          context,
          title: 'Akun',
          body:
              'Username hanya dapat dibaca jika sudah tersimpan. Nomor '
              'telepon dan alamat utama tetap wajib diisi untuk menyelesaikan '
              'pendaftaran seller.',
        ),
        const SizedBox(height: 16),
        _buildSellerInfoSection(
          context,
          title: 'Pembayaran & Aktivasi',
          body:
              'Kemampuan For Sale, Auction, dan Promotion aktif selama '
              'subscription berlaku. Jika subscription berakhir, ketiga '
              'kemampuan tersebut tidak dapat digunakan, namun status Seller '
              'Anda tetap.',
        ),
      ],
    );
  }

  Widget _buildSellerInfoSection(
    BuildContext context, {
    required String title,
    required String body,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          title,
          style: context.typeRoles.titleCompact.copyWith(
            fontWeight: FontWeight.bold,
            color: Theme.of(context).colorScheme.onSurface,
          ),
        ),
        const SizedBox(height: 4),
        Text(
          body,
          style: context.typeRoles.bodyDense.copyWith(
            height: 1.5,
            color: Theme.of(context).colorScheme.onSurfaceVariant,
          ),
        ),
      ],
    );
  }
}

class _PaymentPendingDialog extends ConsumerStatefulWidget {
  final _SellerPaymentOperationContext operationContext;
  final bool Function() isCurrentOperationPrincipal;
  final Future<void> Function() onSuccess;

  const _PaymentPendingDialog({
    required this.operationContext,
    required this.isCurrentOperationPrincipal,
    required this.onSuccess,
  });

  @override
  ConsumerState<_PaymentPendingDialog> createState() =>
      _PaymentPendingDialogState();
}

class _PaymentPendingDialogState extends ConsumerState<_PaymentPendingDialog>
    with WidgetsBindingObserver {
  Timer? _timer;
  bool _timedOut = false;
  int _attempts = 0;

  @override
  void initState() {
    super.initState();
    // Batch 2: settlement can land while the dialog is backgrounded (user
    // switches to m-banking / wallet app). Resume is the natural moment the
    // truth changed — refresh immediately instead of waiting for the next 3s
    // tick or the 60s timeout.
    WidgetsBinding.instance.addObserver(this);
    _startPolling();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _timer?.cancel();
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state != AppLifecycleState.resumed) return;
    if (!widget.isCurrentOperationPrincipal()) return;
    // Refresh once per resume; the periodic poll handles the rest.
    ref.read(authControllerProvider.notifier).forceRefreshAuthState();
  }

  /// PAYMENT_SYNC_ON_DEMAND: fire the backend sync for the tracked payment
  /// (POST /payments/:id/sync). Best-effort — every failure mode (unknown id,
  /// gateway timeout, network) leaves the following auth re-read as the
  /// fallback truth source, exactly like the pre-sync behavior.
  Future<void> _syncPaymentIfTracked() async {
    final paymentId = widget.operationContext.paymentId;
    if (paymentId == null || paymentId.isEmpty) return;
    try {
      await ref.read(paymentRemoteDatasourceProvider).syncPayment(paymentId);
    } catch (_) {
      // Swallowed on purpose: the polling loop and the manual re-check both
      // re-read the authoritative auth state afterwards.
    }
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
      // PAYMENT_SYNC_ON_DEMAND: actively ask the backend to sync gateway truth
      // for THIS payment before re-reading the auth snapshot. A webhook cannot
      // reach a non-public backend and the discovery worker enforces an inquiry
      // eligibility age, so without this the settled payment stays invisible.
      await _syncPaymentIfTracked();
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
      // Registration success signal only: the seller profile now exists AND
      // market authority is active. Renewal success (subscription expiry
      // extension) belongs to SellerRenewalScreen and is never detected here.
      if (authState is AuthStateAuthenticated &&
          authState.user.hasSellerProfile == true &&
          authState.user.hasMarketAuthority == true) {
        timer.cancel();
        await widget.onSuccess();
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Memproses pembayaran'),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const LinearProgressIndicator(),
          const SizedBox(height: 16),
          Text(
            _timedOut
                ? 'Pembayaran masih diproses. Anda bisa menutup dialog ini dan memeriksa lagi nanti.'
                : 'Kami menunggu konfirmasi pembayaran dan aktivasi seller Anda.',
          ),
        ],
      ),
      actions: [
        // Batch 2: manual re-entry point. Settlement can outlive the polling
        // window (VA can take minutes-hours), so the user must never be left
        // without a way to close this dialog and re-check. Closing keeps the
        // wizard on step 5 for a fresh initiate (the backend reuses the same
        // pending payment idempotently).
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('Cek status pembayaran'),
        ),
      ],
    );
  }
}
