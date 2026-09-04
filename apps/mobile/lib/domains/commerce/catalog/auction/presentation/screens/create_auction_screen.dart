/// Create Auction Screen
///
/// PASS_21B: auction creation no longer picks an existing Listing as its
/// source. Product/koi fields are entered directly in this form — the
/// backend creates the Product inline from them, exactly like
/// CreateFixedPriceSaleRequest already does for fixed-price listings.
/// Auction must never be sourced from a Listing (rejected design).
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_providers.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/widgets/for_sale_media_handler.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/widgets/seller_shipping_options_selector.dart';

/// Create Auction Screen
///
/// PASS_21C: edit-mode support (auctionToEdit) was removed. It was dead
/// code end-to-end — no route ever constructed this screen with an auction
/// to edit, and neither AuctionNotifier.updateAuction nor
/// AuctionRepository.updateAuction had any other caller either. Sellers
/// manage an existing auction via cancelAuction only; a real edit flow can
/// be added later as a deliberate feature, not resurrected from this stub.
class CreateAuctionScreen extends ConsumerStatefulWidget {
  const CreateAuctionScreen({super.key});

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

// Koi varieties list (same catalog used by create-listing).
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
  final _preparationNoteController = TextEditingController();
  final _breederController = TextEditingController();
  final _bloodlineController = TextEditingController();

  // Product/koi fields entered directly — the backend creates the Product
  // inline from these, same as fixed-price listing creation.
  final List<String> _mediaUrls = [];
  String? _variety;
  double? _sizeInCm;
  int? _ageInMonths;
  String? _gender;

  /// "now" (immediate start, default) or "scheduled" (custom future start).
  /// Backend enforces this — the picker below is a convenience only.
  String _startMode = _AuctionStartMode.now;
  DateTime? _scheduledStartTime;
  int? _durationHours;

  /// Shipping options this auction can be fulfilled through. Backend
  /// requires at least one (PASS_18E) — auction is still a physical fish
  /// that must ship, same as a fixed-price listing.
  List<String> _selectedShippingSetupIds = const [];

  bool _isSubmitting = false;
  String? _errorMessage;

  @override
  void dispose() {
    _titleController.dispose();
    _descriptionController.dispose();
    _openingBidController.dispose();
    _bidIncrementController.dispose();
    _buyNowPriceController.dispose();
    _preparationNoteController.dispose();
    _breederController.dispose();
    _bloodlineController.dispose();
    super.dispose();
  }

  /// Picks the custom scheduled start time. Only shown when _startMode is
  /// "scheduled" — backend requires this to be strictly in the future.
  Future<void> _pickScheduledStartTime() async {
    final now = DateTime.now();
    final initial = _scheduledStartTime ?? now.add(const Duration(hours: 1));
    final date = await showDatePicker(
      context: context,
      initialDate: initial.isAfter(now)
          ? initial
          : now.add(const Duration(hours: 1)),
      firstDate: now,
      lastDate: now.add(const Duration(days: 365)),
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

  Future<void> _submitForm() async {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) {
      return;
    }

    final currentUser = authState.user;
    final hasSellerProfile = currentUser.hasCreatedSellerProfile;
    final hasMarketAuthority = currentUser.hasMarketAuthority == true;

    if (!hasSellerProfile || !hasMarketAuthority) {
      setState(() {
        _errorMessage = hasSellerProfile
            ? 'Langganan seller Anda sudah berakhir. Perpanjang dulu untuk membuat lelang.'
            : 'Buat seller profile dulu untuk membuat lelang.';
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

    final openingBid = double.tryParse(_openingBidController.text.trim());
    final bidIncrement = double.tryParse(_bidIncrementController.text.trim());
    final buyNowText = _buyNowPriceController.text.trim();
    final buyNowPrice = buyNowText.isEmpty ? null : double.tryParse(buyNowText);

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
      if (!scheduled.isAfter(DateTime.now())) {
        setState(() => _errorMessage = 'Waktu mulai harus di masa depan.');
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
      breeder: _breederController.text.trim().isEmpty
          ? null
          : _breederController.text.trim(),
      bloodline: _bloodlineController.text.trim().isEmpty
          ? null
          : _bloodlineController.text.trim(),
    );

    final success = await ref
        .read(auctionNotifierProvider.notifier)
        .createAuction(
          sellerId: currentUser.id,
          sellerUsername: currentUser.username,
          sellerFarmName: null,
          sellerAvatar: currentUser.avatarUrl,
          title: _titleController.text.trim(),
          description: _descriptionController.text.trim(),
          mediaUrls: _mediaUrls,
          mediaTypes: List.filled(_mediaUrls.length, AuctionMediaType.photo),
          koiDetails: koiDetails,
          openingBid: openingBid,
          bidIncrement: bidIncrement,
          buyNowPrice: buyNowPrice,
          startMode: _startMode,
          scheduledStartAt: scheduledStartAt,
          durationHours: durationHours,
          farmAddressId: null,
          location: null,
          shippingSetupIds: _selectedShippingSetupIds,
          preparationNote: _preparationNoteController.text.trim().isEmpty
              ? null
              : _preparationNoteController.text.trim(),
        );

    if (!mounted) return;

    if (!success) {
      final notifierState = ref.read(auctionNotifierProvider);
      // Commerce restriction — canonical backend rejection.
      if (CommerceRestrictionPresenter.isCommerceRestricted(notifierState.errorCode)) {
        setState(() => _isSubmitting = false);
        CommerceRestrictionPresenter.show(
          context,
          actionDescription: 'membuat lelang',
        );
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
    Navigator.of(context).pop(true);
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authControllerProvider);
    final isDark = Theme.of(context).brightness == Brightness.dark;

    if (authState is! AuthStateAuthenticated) {
      return Scaffold(
        appBar: AppBar(title: const Text('Buat Lelang')),
        body: const Center(child: Text('Silakan login untuk melanjutkan.')),
      );
    }

    final currentUser = authState.user;

    if (!currentUser.hasCreatedSellerProfile) {
      return _buildAccessGate(
        context,
        isDark: isDark,
        title: 'Jadi Seller Dulu',
        message:
            'Untuk membuat lelang, kamu perlu membuat seller profile terlebih dahulu.',
        buttonLabel: 'Mulai Jualan',
      );
    }

    if (currentUser.hasMarketAuthority != true) {
      return _buildAccessGate(
        context,
        isDark: isDark,
        title: 'Langganan Seller Habis',
        message:
            'Aktifkan kembali langganan seller agar bisa membuat lelang di mobile.',
        buttonLabel: 'Perpanjang Langganan',
      );
    }

    return Scaffold(
      appBar: AppBar(title: const Text('Buat Lelang'), centerTitle: true),
      body: SafeArea(
        child: Form(
          key: _formKey,
          child: ListView(
            padding: const EdgeInsets.all(16),
            children: [
              const Text(
                'Informasi Dasar',
                style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: _titleController,
                decoration: const InputDecoration(
                  labelText: 'Judul *',
                  border: OutlineInputBorder(),
                ),
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
              TextFormField(
                controller: _descriptionController,
                maxLines: 4,
                decoration: const InputDecoration(
                  labelText: 'Deskripsi *',
                  border: OutlineInputBorder(),
                ),
                validator: (value) {
                  if (value == null || value.trim().isEmpty) {
                    return 'Deskripsi wajib diisi';
                  }
                  return null;
                },
              ),
              const SizedBox(height: 24),
              const Text(
                'Foto Ikan',
                style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
              ),
              const SizedBox(height: 12),
              _buildMediaSection(isDark),
              const SizedBox(height: 24),
              const Text(
                'Detail Koi',
                style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
              ),
              const SizedBox(height: 12),
              _buildKoiDetailsSection(),
              const SizedBox(height: 24),
              Row(
                children: [
                  Expanded(
                    child: TextFormField(
                      controller: _openingBidController,
                      keyboardType: TextInputType.number,
                      decoration: const InputDecoration(
                        labelText: 'Harga Awal *',
                        border: OutlineInputBorder(),
                      ),
                      validator: (value) {
                        if (value == null || value.trim().isEmpty) {
                          return 'Harga awal wajib diisi';
                        }
                        if (double.tryParse(value.trim()) == null) {
                          return 'Format angka tidak valid';
                        }
                        return null;
                      },
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: TextFormField(
                      controller: _bidIncrementController,
                      keyboardType: TextInputType.number,
                      decoration: const InputDecoration(
                        labelText: 'Kenaikan Bid *',
                        border: OutlineInputBorder(),
                      ),
                      validator: (value) {
                        if (value == null || value.trim().isEmpty) {
                          return 'Kenaikan bid wajib diisi';
                        }
                        if (double.tryParse(value.trim()) == null) {
                          return 'Format angka tidak valid';
                        }
                        return null;
                      },
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 16),
              TextFormField(
                controller: _buyNowPriceController,
                keyboardType: TextInputType.number,
                decoration: const InputDecoration(
                  labelText: 'Buy Now Price (opsional)',
                  border: OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 20),
              _buildStartModeSection(context, isDark),
              const SizedBox(height: 20),
              _buildDurationSection(context, isDark),
              const SizedBox(height: 20),
              const Text(
                'Opsi Pengiriman *',
                style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
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
              const SizedBox(height: 16),
              TextFormField(
                controller: _preparationNoteController,
                maxLines: 3,
                decoration: const InputDecoration(
                  labelText: 'Catatan Persiapan (opsional)',
                  border: OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 24),
              if (_errorMessage != null) ...[
                Container(
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: AppColors.primaryRed.withValues(alpha: 0.1),
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(
                      color: AppColors.primaryRed.withValues(alpha: 0.3),
                    ),
                  ),
                  child: Text(
                    _errorMessage!,
                    style: const TextStyle(
                      color: AppColors.primaryRed,
                      fontSize: 14,
                    ),
                  ),
                ),
                const SizedBox(height: 16),
              ],
              ElevatedButton(
                onPressed: _isSubmitting ? null : _submitForm,
                style: ElevatedButton.styleFrom(
                  minimumSize: const Size.fromHeight(50),
                  backgroundColor: AppColors.primaryRed,
                  disabledBackgroundColor: AppColors.neutralGray300,
                ),
                child: _isSubmitting
                    ? const SizedBox(
                        height: 20,
                        width: 20,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          valueColor: AlwaysStoppedAnimation<Color>(
                            Colors.white,
                          ),
                        ),
                      )
                    : const Text(
                        'Buat Lelang',
                        style: TextStyle(
                          fontSize: 16,
                          fontWeight: FontWeight.w600,
                          color: Colors.white,
                        ),
                      ),
              ),
              const SizedBox(height: 24),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildAccessGate(
    BuildContext context, {
    required bool isDark,
    required String title,
    required String message,
    required String buttonLabel,
  }) {
    return Scaffold(
      appBar: AppBar(title: const Text('Buat Lelang')),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(
                Icons.storefront_outlined,
                size: 64,
                color: isDark
                    ? AppColors.neutralGray500
                    : AppColors.neutralGray400,
              ),
              const SizedBox(height: 20),
              Text(
                title,
                style: TextStyle(
                  fontSize: 20,
                  fontWeight: FontWeight.bold,
                  color: isDark
                      ? AppColors.neutralWhite
                      : AppColors.neutralGray900,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 12),
              Text(
                message,
                style: TextStyle(
                  fontSize: 14,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 28),
              ElevatedButton(
                onPressed: () => context.push(RoutePaths.sellerUpgrade),
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppColors.primaryRed,
                  foregroundColor: AppColors.light,
                  padding: const EdgeInsets.symmetric(
                    horizontal: 24,
                    vertical: 14,
                  ),
                ),
                child: Text(buttonLabel),
              ),
            ],
          ),
        ),
      ),
    );
  }

  /// Start-mode selector: "Mulai sekarang" (default) vs "Jadwalkan".
  /// Backend is the source of truth — this UI is a convenience only.
  Widget _buildStartModeSection(BuildContext context, bool isDark) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Text(
          'Waktu Mulai *',
          style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
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
  Widget _buildDurationSection(BuildContext context, bool isDark) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Text(
          'Durasi Lelang *',
          style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
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

  Widget _buildMediaSection(bool isDark) {
    return Column(
      children: [
        if (_mediaUrls.isEmpty)
          GestureDetector(
            onTap: () {
              ForSaleMediaHandler.showMediaPicker(
                context: context,
                currentMediaCount: _mediaUrls.length,
                onMediaUploaded: (urls) async {
                  setState(() => _mediaUrls.addAll(urls));
                },
              );
            },
            child: Container(
              height: 150,
              decoration: BoxDecoration(
                color: AppColors.neutralGray100,
                borderRadius: BorderRadius.circular(12),
                border: Border.all(
                  color: AppColors.neutralGray300,
                  style: BorderStyle.solid,
                ),
              ),
              child: const Center(
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Icon(Icons.add_photo_alternate, size: 40),
                    SizedBox(height: 8),
                    Text('Tap untuk upload foto'),
                    Text('(Minimal 1 foto)', style: TextStyle(fontSize: 12)),
                  ],
                ),
              ),
            ),
          )
        else
          GridView.builder(
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
              crossAxisCount: 3,
              crossAxisSpacing: 8,
              mainAxisSpacing: 8,
            ),
            itemCount: _mediaUrls.length + 1,
            itemBuilder: (context, index) {
              if (index < _mediaUrls.length) {
                final url = _mediaUrls[index];
                return Stack(
                  children: [
                    ClipRRect(
                      borderRadius: BorderRadius.circular(8),
                      child: Image.network(url, fit: BoxFit.cover),
                    ),
                    Positioned(
                      top: 4,
                      right: 4,
                      child: GestureDetector(
                        onTap: () => setState(() => _mediaUrls.removeAt(index)),
                        child: Container(
                          padding: const EdgeInsets.all(4),
                          decoration: const BoxDecoration(
                            color: Colors.black54,
                            shape: BoxShape.circle,
                          ),
                          child: const Icon(
                            Icons.close,
                            size: 16,
                            color: Colors.white,
                          ),
                        ),
                      ),
                    ),
                  ],
                );
              }
              return GestureDetector(
                onTap: () {
                  ForSaleMediaHandler.showMediaPicker(
                    context: context,
                    currentMediaCount: _mediaUrls.length,
                    onMediaUploaded: (urls) async {
                      setState(() => _mediaUrls.addAll(urls));
                    },
                  );
                },
                child: Container(
                  decoration: BoxDecoration(
                    color: AppColors.neutralGray100,
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(
                      color: AppColors.neutralGray300,
                      style: BorderStyle.solid,
                    ),
                  ),
                  child: const Icon(Icons.add, size: 32),
                ),
              );
            },
          ),
      ],
    );
  }

  Widget _buildKoiDetailsSection() {
    return Column(
      children: [
        DropdownButtonFormField<String>(
          initialValue: _variety,
          decoration: const InputDecoration(
            labelText: 'Varietas *',
            border: OutlineInputBorder(),
          ),
          items: _koiVarieties
              .map((v) => DropdownMenuItem(value: v, child: Text(v)))
              .toList(),
          onChanged: (value) => setState(() => _variety = value),
          validator: (value) =>
              value == null || value.isEmpty ? 'Varietas wajib diisi' : null,
        ),
        const SizedBox(height: 16),
        TextFormField(
          initialValue: _sizeInCm?.toString(),
          keyboardType: TextInputType.number,
          decoration: const InputDecoration(
            labelText: 'Ukuran (cm) *',
            hintText: 'Contoh: 50',
            suffixText: 'cm',
            border: OutlineInputBorder(),
          ),
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
          onChanged: (value) => _sizeInCm = double.tryParse(value),
        ),
        const SizedBox(height: 16),
        TextFormField(
          initialValue: _ageInMonths?.toString(),
          keyboardType: TextInputType.number,
          decoration: const InputDecoration(
            labelText: 'Usia (bulan)',
            hintText: 'Contoh: 24',
            suffixText: 'bulan',
            border: OutlineInputBorder(),
          ),
          onChanged: (value) => _ageInMonths = int.tryParse(value),
        ),
        const SizedBox(height: 16),
        DropdownButtonFormField<String>(
          initialValue: _gender,
          decoration: const InputDecoration(
            labelText: 'Jenis Kelamin',
            border: OutlineInputBorder(),
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
        TextFormField(
          controller: _breederController,
          decoration: const InputDecoration(
            labelText: 'Breeder',
            hintText: 'Nama breeder',
            border: OutlineInputBorder(),
          ),
        ),
        const SizedBox(height: 16),
        TextFormField(
          controller: _bloodlineController,
          decoration: const InputDecoration(
            labelText: 'Bloodline',
            hintText: 'Keturunan/bloodline',
            border: OutlineInputBorder(),
          ),
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
      borderRadius: BorderRadius.circular(12),
      child: InputDecorator(
        decoration: InputDecoration(
          labelText: label,
          border: const OutlineInputBorder(),
        ),
        child: Row(
          children: [
            const Icon(Icons.calendar_month_outlined, size: 18),
            const SizedBox(width: 10),
            Expanded(child: Text(value)),
          ],
        ),
      ),
    );
  }
}
