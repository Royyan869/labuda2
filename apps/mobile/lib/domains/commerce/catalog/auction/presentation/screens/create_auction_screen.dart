/// Create Auction Screen
///
/// PASS_21B: auction creation no longer picks an existing ForSale as its
/// source. Product/koi fields are entered directly in this form — the
/// backend creates the Product inline from them, exactly like
/// CreateFixedPriceSaleRequest already does for fixed-price forSales.
/// Auction must never be sourced from a ForSale (rejected design).
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/core/common/types/preparation_time.dart';
import 'package:hishumi/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:hishumi/domains/commerce/catalog/auction/presentation/create_auction_route_contract.dart';
import 'package:hishumi/domains/commerce/catalog/auction/presentation/providers/auction_providers.dart';
import 'package:hishumi/features/home/home.dart';
import 'package:hishumi/domains/commerce/catalog/shared/presentation/widgets/commerce_access_gate.dart';
import 'package:hishumi/domains/commerce/catalog/shared/presentation/widgets/commerce_certificate_selector.dart';
import 'package:hishumi/domains/commerce/catalog/shared/presentation/widgets/commerce_preparation_time_selector.dart';
import 'package:hishumi/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_states.dart';
import 'package:hishumi/shared/widgets/media_grid_uploader.dart';
import 'package:hishumi/shared/shared.dart';
import 'package:hishumi/domains/commerce/transaction/shipping/presentation/widgets/seller_shipping_options_selector.dart';
import 'package:hishumi/domains/user/preference/seller/presentation/providers/current_seller_provider.dart';

/// Create Auction Screen
///
/// PASS_21C: edit-mode support (auctionToEdit) was removed. It was dead
/// code end-to-end — no route ever constructed this screen with an auction
/// to edit, and neither AuctionNotifier.updateAuction nor
/// AuctionRepository.updateAuction had any other caller either. Sellers
/// manage an existing auction via cancelAuction only; a real edit flow can
/// be added later as a deliberate feature, not resurrected from this stub.
class CreateAuctionScreen extends ConsumerStatefulWidget {
  /// Optional caller intent. `null` (the global create entry) keeps the
  /// canonical Marketplace landing; a management-page caller passes
  /// [CreateAuctionRouteArgs.stay].
  final CreateAuctionRouteArgs? routeArgs;

  const CreateAuctionScreen({super.key, this.routeArgs});

  @override
  ConsumerState<CreateAuctionScreen> createState() =>
      _CreateAuctionScreenState();
}

/// Wire values for the `start_mode` field the backend expects (PASS_18C).
abstract final class _AuctionStartMode {
  static const String now = 'now';
  static const String scheduled = 'scheduled';
}

/// A selectable auction duration preset (owner-approved: 1-7 days).
class _DurationPreset {
  final String label;
  final int hours;
  const _DurationPreset(this.label, this.hours);
}

const List<_DurationPreset> _durationPresets = [
  _DurationPreset('1 hari', 24),
  _DurationPreset('3 hari', 72),
  _DurationPreset('5 hari', 120),
  _DurationPreset('7 hari', 168),
];

// Koi varieties list (same catalog used by create-forSale).
const _koiVarieties = [
  'Kohaku',
  'Sanke',
  'Showa',
  'Utsurimono',
  'Bekko',
  'Tancho',
  'Shiro Utsuri',
  'Hi Utsuri',
  'Ki Utsuri',
  'Showa Sanshoku',
  'Goshiki',
  'Koromo',
  'Kawarimono',
  'Hikarimuji',
  'Hikarimono',
  'Ogon',
  'Platinum Ogon',
  'Yamabuki Ogon',
  'Orenji Ogon',
  'Kujaku',
  'Kikusui',
  'Hariwake',
  'Shusui',
  'Asagi',
  'Doitsu',
  'Ghost Koi',
  'Butterfly Koi',
  'Lainnya',
];

const _koiGenders = [
  {'value': 'male', 'label': 'Jantan'},
  {'value': 'female', 'label': 'Betina'},
  {'value': 'unknown', 'label': 'Tidak Diketahui'},
];

class _CreateAuctionScreenState extends ConsumerState<CreateAuctionScreen> {
  final _formKey = GlobalKey<FormState>();
  final _titleController = TextEditingController();
  final _descriptionController = TextEditingController();
  final _openingBidController = TextEditingController();
  final _bidIncrementController = TextEditingController();
  final _buyNowPriceController = TextEditingController();
  final _breederController = TextEditingController();
  final _bloodlineController = TextEditingController();

  // Product/koi fields entered directly — the backend creates the Product
  // inline from these, same as fixed-price forSale creation.
  final List<String> _mediaUrls = [];
  String? _variety;
  double? _sizeInCm;
  int? _ageInMonths;
  String? _gender;

  /// Seller-declared certificates (canonical values: breeder, contest, import,
  /// health). Optional product content — a certificate states that the fish is
  /// certified, it never asks the seller to upload a document.
  List<String> _certificates = const [];

  /// "now" (immediate start, default) or "scheduled" (custom future start).
  /// Backend enforces this — the picker below is a convenience only.
  String _startMode = _AuctionStartMode.now;
  DateTime? _scheduledStartTime;
  int? _durationHours;

  /// Shipping options this auction can be fulfilled through. Backend
  /// requires at least one (PASS_18E) — auction is still a physical fish
  /// that must ship, same as a fixed-price forSale.
  List<String> _selectedShippingSetupIds = const [];

  /// Preparation-time range this auction promises after checkout (owner:
  /// exactly 3 ranges, default 1–3 days).
  PreparationTime _preparationTime = PreparationTime.days1_3;

  bool _isSubmitting = false;
  String? _errorMessage;

  /// Controllers feeding the pre-submit completeness gate — the CTA must
  /// re-evaluate on every keystroke (a gate reading only setState-driven state
  /// would never enable while the seller types).
  Iterable<TextEditingController> get _gatedControllers => [
    _titleController,
    _descriptionController,
    _openingBidController,
    _bidIncrementController,
    _buyNowPriceController,
  ];

  void _onFormFieldsChanged() {
    // Re-evaluates the publish-CTA completeness gate after each keystroke.
    setState(() {});
  }

  @override
  void initState() {
    super.initState();
    for (final controller in _gatedControllers) {
      controller.addListener(_onFormFieldsChanged);
    }
  }

  @override
  void dispose() {
    for (final controller in _gatedControllers) {
      controller.removeListener(_onFormFieldsChanged);
    }
    _titleController.dispose();
    _descriptionController.dispose();
    _openingBidController.dispose();
    _bidIncrementController.dispose();
    _buyNowPriceController.dispose();
    _breederController.dispose();
    _bloodlineController.dispose();
    super.dispose();
  }

  /// Picks the custom scheduled start time. Only shown when _startMode is
  /// "scheduled" — backend requires this to be strictly in the future and
  /// no later than 30 days from server time (MaxScheduledStartHorizon).
  /// Mobile enforces the same 30-day UX limit; backend remains authoritative
  /// and rejects manipulated/stale client input.
  Future<void> _pickScheduledStartTime() async {
    final now = DateTime.now();
    final maxHorizon = now.add(const Duration(days: 30));
    final initial = _scheduledStartTime ?? now.add(const Duration(hours: 1));
    final clampedInitial = initial.isAfter(maxHorizon)
        ? maxHorizon
        : (initial.isAfter(now) ? initial : now.add(const Duration(hours: 1)));
    final date = await showDatePicker(
      context: context,
      initialDate: clampedInitial,
      firstDate: now,
      lastDate: maxHorizon,
    );
    if (date == null) return;
    if (!mounted) return;

    final time = await showTimePicker(
      context: context,
      initialTime: TimeOfDay.fromDateTime(initial),
    );
    if (time == null) return;

    setState(() {
      _scheduledStartTime = DateTime(
        date.year,
        date.month,
        date.day,
        time.hour,
        time.minute,
      );
    });
  }

  String _formatDateTime(DateTime? value) {
    if (value == null) return 'Pilih tanggal dan waktu';
    final local = value.toLocal();
    final day = local.day.toString().padLeft(2, '0');
    final month = local.month.toString().padLeft(2, '0');
    final hour = local.hour.toString().padLeft(2, '0');
    final minute = local.minute.toString().padLeft(2, '0');
    return '$day/$month/${local.year} $hour:$minute';
  }

  /// PRE-SUBMIT COMPLETENESS GATE (owner decision): the publish CTA stays
  /// disabled until every required field carries a usable value. Format rules
  /// (title length, numeric format, future start time) stay with the Form
  /// validators and the guards inside _submitForm — this gate only answers
  /// "is everything filled in", never "is everything valid".
  bool get _isFormComplete {
    final title = _titleController.text.trim();
    final description = _descriptionController.text.trim();
    // Canonical money parse: the field displays grouped (`1.000.000`) while
    // the business value stays an int.
    final openingBid = MoneyInputFormatter.parseAmount(
      _openingBidController.text,
    );
    final bidIncrement = MoneyInputFormatter.parseAmount(
      _bidIncrementController.text,
    );
    final buyNowText = _buyNowPriceController.text.trim();
    final buyNowPrice = MoneyInputFormatter.parseAmount(buyNowText);

    return title.isNotEmpty &&
        description.isNotEmpty &&
        _mediaUrls.isNotEmpty &&
        _variety != null &&
        _sizeInCm != null &&
        _sizeInCm! > 0 &&
        openingBid != null &&
        openingBid > 0 &&
        bidIncrement != null &&
        bidIncrement > 0 &&
        (buyNowText.isEmpty ||
            (buyNowPrice != null && buyNowPrice >= openingBid)) &&
        _durationHours != null &&
        (_startMode == _AuctionStartMode.now || _scheduledStartTime != null) &&
        _selectedShippingSetupIds.isNotEmpty;
  }

  Future<void> _submitForm() async {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) {
      return;
    }

    final currentUser = authState.user;
    final hasSellerProfile = currentUser.hasCreatedSellerProfile;
    final hasMarketAuthority = currentUser.hasMarketAuthority == true;
    // Canonical expiry axis (RF-02): capability blocks the mutation, but only
    // an ENDED subscription may claim expiry. Status 'none' = not active yet.
    final isSubscriptionExpired = ref.read(isSellerSubscriptionExpiredProvider);

    if (!hasSellerProfile || !hasMarketAuthority) {
      setState(() {
        _errorMessage = !hasSellerProfile
            ? 'Buat seller profile dulu untuk membuat lelang.'
            : isSubscriptionExpired
            ? 'Langganan seller Anda sudah berakhir. Perpanjang dulu untuk membuat lelang.'
            : 'Langganan seller belum aktif. Aktifkan dulu untuk membuat lelang.';
      });
      return;
    }

    if (!_formKey.currentState!.validate()) {
      return;
    }

    if (_mediaUrls.isEmpty) {
      setState(() => _errorMessage = 'Minimal 1 foto wajib diupload');
      return;
    }

    if (_variety == null || _sizeInCm == null) {
      setState(() => _errorMessage = 'Detail koi wajib diisi untuk lelang');
      return;
    }

    // Canonical money parse: the field displays grouped (`1.000.000`) while
    // the business value stays an int.
    final openingBid = MoneyInputFormatter.parseAmount(
      _openingBidController.text,
    );
    final bidIncrement = MoneyInputFormatter.parseAmount(
      _bidIncrementController.text,
    );
    final buyNowText = _buyNowPriceController.text.trim();
    final buyNowPrice = MoneyInputFormatter.parseAmount(buyNowText);

    if (openingBid == null || openingBid <= 0) {
      setState(() => _errorMessage = 'Harga awal harus lebih dari 0.');
      return;
    }
    if (bidIncrement == null || bidIncrement <= 0) {
      setState(() => _errorMessage = 'Kenaikan bid harus lebih dari 0.');
      return;
    }
    if (buyNowText.isNotEmpty &&
        (buyNowPrice == null || buyNowPrice < openingBid)) {
      setState(() {
        _errorMessage = 'Buy now harus sama atau lebih tinggi dari harga awal.';
      });
      return;
    }

    final durationHours = _durationHours;
    if (durationHours == null) {
      setState(() => _errorMessage = 'Pilih durasi lelang.');
      return;
    }

    DateTime? scheduledStartAt;
    if (_startMode == _AuctionStartMode.scheduled) {
      final scheduled = _scheduledStartTime;
      if (scheduled == null) {
        setState(() => _errorMessage = 'Pilih waktu mulai lelang.');
        return;
      }
      final nowCheck = DateTime.now();
      if (!scheduled.isAfter(nowCheck)) {
        setState(() => _errorMessage = 'Waktu mulai harus di masa depan.');
        return;
      }
      // UX guard mirrors backend MaxScheduledStartHorizon (30 days).
      // Backend remains authoritative for manipulated/stale input.
      if (scheduled.isAfter(nowCheck.add(const Duration(days: 30)))) {
        setState(
          () => _errorMessage =
              'Waktu mulai tidak boleh lebih dari 30 hari dari sekarang.',
        );
        return;
      }
      scheduledStartAt = scheduled;
    }

    if (_selectedShippingSetupIds.isEmpty) {
      setState(
        () => _errorMessage =
            'Pilih minimal 1 opsi pengiriman agar lelang bisa dipublish.',
      );
      return;
    }

    setState(() {
      _isSubmitting = true;
      _errorMessage = null;
    });

    final koiDetails = KoiDetails(
      variety: _variety!,
      sizeInCm: _sizeInCm!,
      ageInMonths: _ageInMonths ?? 0,
      gender: _gender ?? 'unknown',
      certificates: List<String>.of(_certificates),
      breeder: _breederController.text.trim().isEmpty
          ? null
          : _breederController.text.trim(),
      bloodline: _bloodlineController.text.trim().isEmpty
          ? null
          : _bloodlineController.text.trim(),
    );

    // SUBMISSION SNAPSHOT: the mutable media list must not change under the
    // in-flight request (the mapper reads it after an await). Snapshot it here.
    final mediaSnapshot = List<String>.of(_mediaUrls);

    final success = await ref
        .read(auctionNotifierProvider.notifier)
        .createAuction(
          sellerId: currentUser.id,
          sellerUsername: currentUser.username,
          sellerFarmName: null,
          sellerAvatar: currentUser.avatarUrl,
          title: _titleController.text.trim(),
          description: _descriptionController.text.trim(),
          mediaUrls: mediaSnapshot,
          mediaTypes: List<AuctionMediaType>.filled(
            mediaSnapshot.length,
            AuctionMediaType.photo,
          ),
          koiDetails: koiDetails,
          openingBid: openingBid,
          bidIncrement: bidIncrement,
          buyNowPrice: buyNowPrice,
          startMode: _startMode,
          scheduledStartAt: scheduledStartAt,
          durationHours: durationHours,
          preparationTime: _preparationTime,
          shippingSetupIds: List<String>.of(_selectedShippingSetupIds),
        );

    if (!mounted) return;

    if (!success) {
      final notifierState = ref.read(auctionNotifierProvider);
      // Commerce restriction family — canonical dispatch by error CODE:
      // COMMERCE_RESTRICTED → restriction snackbar,
      // MARKET_AUTHORITY_REQUIRED → canonical seller renewal. The local submit
      // flag is reset first (same ordering as before) so the form is never left
      // locked behind a consumed restriction error.
      if (CommerceRestrictionPresenter.isRestrictionPresented(
        notifierState.errorCode,
      )) {
        setState(() => _isSubmitting = false);
      }
      if (CommerceRestrictionPresenter.handle(
        context,
        errorCode: notifierState.errorCode,
        actionDescription: 'membuat lelang',
      )) {
        return;
      }
      setState(() {
        _errorMessage =
            notifierState.error ??
            'Gagal membuat lelang. Cek pesan dari backend.';
      });
      setState(() => _isSubmitting = false);
      return;
    }

    AppSnackBar.showSuccess(context, 'Lelang berhasil dibuat');

    // CALLER-AWARE LANDING: only the global create entry (no route args)
    // retargets the shell to Marketplace → Auction. A management-page caller
    // opts out via [CreateAuctionRouteArgs.stay]. List invalidation (including
    // the My Auctions pager) happens inside the notifier.
    if (widget.routeArgs?.landsOnMarketplace ?? true) {
      ref
          .read(pendingTabSwitchProvider.notifier)
          .setSwitch('marketplace', subTab: 1);
    }

    Navigator.of(context).pop(true);
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authControllerProvider);
    final scheme = Theme.of(context).colorScheme;
    // Canonical expiry axis (RF-02). Capability gates access to selling; only
    // an ENDED subscription period may produce expiry/renewal copy.
    final isSubscriptionExpired = ref.watch(
      isSellerSubscriptionExpiredProvider,
    );

    // CHANNEL PARITY with the for-sale create screen: only a RESOLVED
    // "not signed in" session gets the login gate. Hydration and a restricted
    // account never claim "please login" — they fail closed on the canonical
    // loading surface / the canonical restricted screen.
    if (authState is AuthStateAccountRestricted) {
      return const AccountRestrictedScreen();
    }

    if (authState is AuthStateUnauthenticated) {
      return CommerceAccessGate(
        screenTitle: 'Buat Lelang',
        headline: 'Login Diperlukan',
        message: 'Silakan login untuk melanjutkan.',
        buttonLabel: 'Masuk',
        onAction: () => context.push('/auth/sign-in'),
      );
    }

    if (authState is! AuthStateAuthenticated) {
      return CommerceDetailStates.loading(title: 'Buat Lelang');
    }

    final currentUser = authState.user;

    if (!currentUser.hasCreatedSellerProfile) {
      return CommerceAccessGate(
        screenTitle: 'Buat Lelang',
        headline: 'Jadi Seller Dulu',
        message:
            'Untuk membuat lelang, kamu perlu membuat seller profile terlebih dahulu.',
        buttonLabel: 'Mulai Jualan',
        onAction: () => context.push(RoutePaths.sellerUpgrade),
      );
    }

    if (currentUser.hasMarketAuthority != true) {
      // Capability gate unchanged; only its COPY follows the expiry axis.
      return CommerceAccessGate(
        screenTitle: 'Buat Lelang',
        headline: isSubscriptionExpired
            ? 'Langganan Seller Habis'
            : 'Langganan Belum Aktif',
        message: isSubscriptionExpired
            ? 'Aktifkan kembali langganan seller agar bisa membuat lelang di mobile.'
            : 'Aktifkan langganan seller agar bisa membuat lelang di mobile.',
        buttonLabel: isSubscriptionExpired
            ? 'Perpanjang Langganan'
            : 'Aktifkan Langganan',
        onAction: () => context.push(RoutePaths.sellerRenewal),
      );
    }

    return Scaffold(
      appBar: AppBar(title: const Text('Buat Lelang'), centerTitle: true),
      body: SafeArea(
        child: Form(
          key: _formKey,
          child: ListView(
            padding: const EdgeInsets.all(AppMetrics.p16),
            children: [
              Text(
                'Informasi Dasar',
                style: context.typeRoles.titleSection.copyWith(
                  fontWeight: FontWeight.bold,
                ),
              ),
              const SizedBox(height: 12),
              AppTextField(
                controller: _titleController,
                labelText: 'Judul *',
                validator: (value) {
                  if (value == null || value.trim().isEmpty) {
                    return 'Judul wajib diisi';
                  }
                  if (value.trim().length < 5) {
                    return 'Judul terlalu pendek';
                  }
                  return null;
                },
              ),
              const SizedBox(height: 16),
              AppTextField(
                controller: _descriptionController,
                maxLines: 4,
                labelText: 'Deskripsi *',
                validator: (value) {
                  if (value == null || value.trim().isEmpty) {
                    return 'Deskripsi wajib diisi';
                  }
                  return null;
                },
              ),
              const SizedBox(height: 24),
              Text(
                'Foto Ikan',
                style: context.typeRoles.titleSection.copyWith(
                  fontWeight: FontWeight.bold,
                ),
              ),
              const SizedBox(height: 12),
              _buildMediaSection(),
              const SizedBox(height: 24),
              Text(
                'Detail Koi',
                style: context.typeRoles.titleSection.copyWith(
                  fontWeight: FontWeight.bold,
                ),
              ),
              const SizedBox(height: 12),
              _buildKoiDetailsSection(),
              const SizedBox(height: 24),
              Row(
                children: [
                  Expanded(
                    child: AppTextField(
                      controller: _openingBidController,
                      keyboardType: TextInputType.number,
                      inputFormatters: const [MoneyInputFormatter()],
                      labelText: 'Harga Awal *',
                      validator: (value) {
                        if (value == null || value.trim().isEmpty) {
                          return 'Harga awal wajib diisi';
                        }
                        if (MoneyInputFormatter.parseAmount(value) == null) {
                          return 'Format angka tidak valid';
                        }
                        return null;
                      },
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: AppTextField(
                      controller: _bidIncrementController,
                      keyboardType: TextInputType.number,
                      inputFormatters: const [MoneyInputFormatter()],
                      labelText: 'Kenaikan Bid *',
                      validator: (value) {
                        if (value == null || value.trim().isEmpty) {
                          return 'Kenaikan bid wajib diisi';
                        }
                        if (MoneyInputFormatter.parseAmount(value) == null) {
                          return 'Format angka tidak valid';
                        }
                        return null;
                      },
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 16),
              AppTextField(
                controller: _buyNowPriceController,
                keyboardType: TextInputType.number,
                inputFormatters: const [MoneyInputFormatter()],
                labelText: 'Buy Now Price (opsional)',
              ),
              const SizedBox(height: 20),
              _buildStartModeSection(context),
              const SizedBox(height: 20),
              _buildDurationSection(context),
              const SizedBox(height: 20),
              CommercePreparationTimeSelector(
                selected: _preparationTime,
                onChanged: (value) => setState(() => _preparationTime = value),
              ),
              const SizedBox(height: 20),
              Text(
                'Opsi Pengiriman *',
                style: context.typeRoles.titleSection.copyWith(
                  fontWeight: FontWeight.w600,
                ),
              ),
              const SizedBox(height: 8),
              SellerShippingSetupsSelector(
                initialSelectedIds: _selectedShippingSetupIds,
                helperText:
                    'Pilih opsi pengiriman yang berlaku untuk lelang ini. '
                    'Wajib diisi karena ikan tetap perlu dikirim ke pemenang.',
                onSelectionChanged: (ids) =>
                    setState(() => _selectedShippingSetupIds = ids),
              ),
              const SizedBox(height: 24),
              if (_errorMessage != null) ...[
                Container(
                  padding: const EdgeInsets.all(AppMetrics.p12),
                  decoration: BoxDecoration(
                    color: scheme.error.withValues(alpha: 0.12),
                    borderRadius: BorderRadius.circular(AppShape.r8),
                    border: Border.all(
                      color: scheme.error.withValues(alpha: 0.3),
                    ),
                  ),
                  child: Text(
                    _errorMessage!,
                    style: context.typeRoles.bodyDense.copyWith(
                      color: scheme.onSurface,
                    ),
                  ),
                ),
                const SizedBox(height: 16),
              ],
              ElevatedButton(
                onPressed: _isSubmitting || !_isFormComplete
                    ? null
                    : _submitForm,
                style: ElevatedButton.styleFrom(
                  minimumSize: const Size.fromHeight(50),
                ),
                child: _isSubmitting
                    ? SizedBox(
                        height: 20,
                        width: 20,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          valueColor: AlwaysStoppedAnimation<Color>(
                            scheme.onSurfaceVariant,
                          ),
                        ),
                      )
                    : Text(
                        'Buat Lelang',
                        style: Theme.of(context).textTheme.labelLarge?.copyWith(
                          fontWeight: FontWeight.w600,
                        ),
                      ),
              ),
              if (!_isFormComplete && !_isSubmitting) ...[
                const SizedBox(height: 8),
                Text(
                  'Lengkapi semua field wajib untuk mengaktifkan tombol terbit.',
                  textAlign: TextAlign.center,
                  style: context.typeRoles.labelMicro.copyWith(
                    color: scheme.onSurfaceVariant,
                  ),
                ),
              ],
              const SizedBox(height: 24),
            ],
          ),
        ),
      ),
    );
  }

  /// Start-mode selector: "Mulai sekarang" (default) vs "Jadwalkan".
  /// Backend is the source of truth — this UI is a convenience only.
  Widget _buildStartModeSection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Waktu Mulai *',
          style: context.typeRoles.titleSection.copyWith(
            fontWeight: FontWeight.w600,
          ),
        ),
        const SizedBox(height: 8),
        SegmentedButton<String>(
          segments: const [
            ButtonSegment(
              value: _AuctionStartMode.now,
              label: Text('Mulai Sekarang'),
              icon: Icon(Icons.flash_on_outlined),
            ),
            ButtonSegment(
              value: _AuctionStartMode.scheduled,
              label: Text('Jadwalkan'),
              icon: Icon(Icons.schedule_outlined),
            ),
          ],
          selected: {_startMode},
          onSelectionChanged: (selection) {
            setState(() => _startMode = selection.first);
          },
        ),
        if (_startMode == _AuctionStartMode.scheduled) ...[
          const SizedBox(height: 12),
          _DateTimeField(
            label: 'Waktu Mulai Terjadwal *',
            value: _formatDateTime(_scheduledStartTime),
            onTap: _pickScheduledStartTime,
          ),
        ],
      ],
    );
  }

  /// Duration selector: owner-approved presets (1/3/5/7 days). The backend
  /// enforces the 1-7 day bound regardless of what this UI offers.
  Widget _buildDurationSection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Durasi Lelang *',
          style: context.typeRoles.titleSection.copyWith(
            fontWeight: FontWeight.w600,
          ),
        ),
        const SizedBox(height: 8),
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: _durationPresets.map((preset) {
            final selected = _durationHours == preset.hours;
            return ChoiceChip(
              label: Text(preset.label),
              selected: selected,
              onSelected: (_) {
                setState(() => _durationHours = preset.hours);
              },
            );
          }).toList(),
        ),
      ],
    );
  }

  Widget _buildMediaSection() {
    return MediaGridUploader(
      mediaUrls: _mediaUrls,
      onMediaAdded: (url) => setState(() => _mediaUrls.add(url)),
      onMediaRemoved: (index) => setState(() => _mediaUrls.removeAt(index)),
      onMediaReordered: (oldIndex, newIndex) => setState(() {
        final item = _mediaUrls.removeAt(oldIndex);
        _mediaUrls.insert(newIndex, item);
      }),
    );
  }

  Widget _buildKoiDetailsSection() {
    return Column(
      children: [
        DropdownButtonFormField<String>(
          initialValue: _variety,
          decoration: const InputDecoration(
            labelText: 'Varietas *',
          ),
          items: _koiVarieties
              .map((v) => DropdownMenuItem(value: v, child: Text(v)))
              .toList(),
          onChanged: (value) => setState(() => _variety = value),
          validator: (value) =>
              value == null || value.isEmpty ? 'Varietas wajib diisi' : null,
        ),
        const SizedBox(height: 16),
        AppTextField(
          initialValue: _sizeInCm?.toString(),
          keyboardType: TextInputType.number,
          labelText: 'Ukuran (cm) *',
          hintText: 'Contoh: 50',
          suffixText: 'cm',
          validator: (value) {
            if (value == null || value.isEmpty) {
              return 'Ukuran wajib diisi';
            }
            final size = double.tryParse(value);
            if (size == null || size <= 0) {
              return 'Ukuran harus lebih dari 0';
            }
            return null;
          },
          onChanged: (value) => setState(() => _sizeInCm = double.tryParse(value)),
        ),
        const SizedBox(height: 16),
        AppTextField(
          initialValue: _ageInMonths?.toString(),
          keyboardType: TextInputType.number,
          labelText: 'Usia (bulan)',
          hintText: 'Contoh: 24',
          suffixText: 'bulan',
          onChanged: (value) => setState(() => _ageInMonths = int.tryParse(value)),
        ),
        const SizedBox(height: 16),
        DropdownButtonFormField<String>(
          initialValue: _gender,
          decoration: const InputDecoration(
            labelText: 'Jenis Kelamin',
          ),
          items: _koiGenders
              .map(
                (g) => DropdownMenuItem(
                  value: g['value'],
                  child: Text(g['label'] as String),
                ),
              )
              .toList(),
          onChanged: (value) => setState(() => _gender = value),
        ),
        const SizedBox(height: 16),
        AppTextField(
          controller: _breederController,
          labelText: 'Breeder',
          hintText: 'Nama breeder',
        ),
        const SizedBox(height: 16),
        AppTextField(
          controller: _bloodlineController,
          labelText: 'Bloodline',
          hintText: 'Keturunan/bloodline',
        ),
        const SizedBox(height: 16),
        CommerceCertificateSelector(
          selectedCertificates: _certificates,
          onChanged: (value) => setState(() => _certificates = value),
          helperText:
              'Pilih jenis sertifikat yang ikan ini miliki. Sertifikat adalah '
              'keterangan dari seller, bukan unggahan dokumen.',
        ),
      ],
    );
  }
}

class _DateTimeField extends StatelessWidget {
  final String label;
  final String value;
  final VoidCallback onTap;

  const _DateTimeField({
    required this.label,
    required this.value,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(AppShape.r12),
      child: InputDecorator(
        // Border/fill/geometry come from `inputDecorationTheme`
        // (AppTheme) — the one form-field authority.
        decoration: InputDecoration(labelText: label),
        child: Row(
          children: [
            const Icon(Icons.calendar_month_outlined, size: AppIconSize.action),
            const SizedBox(width: 10),
            Expanded(child: Text(value)),
          ],
        ),
      ),
    );
  }
}
