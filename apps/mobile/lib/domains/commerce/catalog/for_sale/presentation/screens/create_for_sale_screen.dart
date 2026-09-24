/// Create ForSale Screen
///
/// Creates a fixed-price sale (ForSale) — a sibling of Auction over Product,
/// never its parent. Follows clean architecture:
/// - UI renders form state
/// - UI calls controller via provider for create action
/// - No Firebase, no API calls directly
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/api/api_error_codes.dart' as api_codes;
import 'package:labuda/core/common/types/preparation_time.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/shared/widgets/media_grid_uploader.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/widgets/seller_shipping_options_selector.dart';
import 'package:labuda/domains/user/preference/seller/presentation/providers/current_seller_provider.dart';

/// Create ForSale Screen
///
/// UI only - delegates all logic to forSale application layer.
class CreateForSaleScreen extends ConsumerStatefulWidget {
  const CreateForSaleScreen({super.key});

  @override
  ConsumerState<CreateForSaleScreen> createState() =>
      _CreateForSaleScreenState();
}

class _CreateForSaleScreenState extends ConsumerState<CreateForSaleScreen> {
  final _formKey = GlobalKey<FormState>();
  final _titleController = TextEditingController();
  final _descriptionController = TextEditingController();
  final _preparationNoteController = TextEditingController();

  // Form state (UI only)
  bool _isNegotiable = true;
  double? _price;
  int _quantity = 1;
  final List<String> _mediaUrls = [];

  // Koi details (required for forSales)
  String? _variety;
  double? _sizeInCm;
  int? _ageInMonths;
  String? _gender;
  String? _breeder;
  String? _bloodline;

  // Shipping readiness
  PreparationTime _preparationTime = PreparationTime.immediate;

  // Shipping option IDs the seller selects to apply to this forSale. They
  // travel INSIDE the create request — create = publish, no separate linking.
  List<String> _selectedShippingSetupIds = const [];

  bool _isSubmitting = false;
  String? _errorMessage;

  @override
  void dispose() {
    _titleController.dispose();
    _descriptionController.dispose();
    _preparationNoteController.dispose();
    super.dispose();
  }

  /// CREATE = PUBLISH: the submit button stays disabled until the form is
  /// complete — required fields filled AND at least one shipping option
  /// selected. The backend re-validates everything (defense in depth).
  bool get _canSubmit {
    final senderAddressId = ref.watch(senderAddressIdProvider).value;
    return !_isSubmitting &&
        senderAddressId != null &&
        _titleController.text.trim().isNotEmpty &&
        _descriptionController.text.trim().isNotEmpty &&
        _mediaUrls.isNotEmpty &&
        _price != null &&
        _variety != null &&
        _sizeInCm != null &&
        _selectedShippingSetupIds.isNotEmpty;
  }

  Future<void> _submitForm() async {
    final authState = ref.read(authControllerProvider);
    final controller = ref.read(forSaleControllerProvider);

    if (!controller.canCreateForSale(authState)) {
      setState(() {
        _errorMessage = _createForSaleAccessMessage(authState);
      });
      return;
    }

    if (!_formKey.currentState!.validate()) {
      return;
    }

    // Validate media
    if (_mediaUrls.isEmpty) {
      setState(() => _errorMessage = 'Minimal 1 media wajib diupload');
      return;
    }

    // Validate required fields for forSale
    if (_price == null) {
      setState(() => _errorMessage = 'Harga wajib diisi untuk forSale');
      return;
    }
    if (_variety == null || _sizeInCm == null) {
      setState(() => _errorMessage = 'Detail koi wajib diisi untuk forSale');
      return;
    }

    // CREATE = PUBLISH: shipping selection is mandatory — the backend
    // rejects a create without at least one option that has active coverage.
    if (_selectedShippingSetupIds.isEmpty) {
      setState(() => _errorMessage = 'Pilih minimal 1 opsi pengiriman untuk forSale ini');
      return;
    }

    setState(() => _isSubmitting = true);
    _errorMessage = null;

    try {
      // CREATE = PUBLISH: one request carries everything — content, price,
      // shipping selection — and the backend publishes in the same
      // transaction. There is no draft stage in this flow.
      final request = CreateForSaleRequest(
        title: _titleController.text.trim(),
        description: _descriptionController.text.trim(),
        price: _price!,
        quantity: _quantity,
        negotiationEnabled: _isNegotiable,
        mediaUrls: _mediaUrls,
        variety: _variety,
        sizeCm: _sizeInCm,
        ageMonths: _ageInMonths ?? 0,
        gender: _gender,
        breeder: _breeder,
        bloodline: _bloodline,
        farmAddressId: ref.read(senderAddressIdProvider).value,
        shippingSetupIds: _selectedShippingSetupIds,
        preparationTime: _preparationTime,
        preparationNote: _preparationNoteController.text.trim().isEmpty
            ? null
            : _preparationNoteController.text.trim(),
      );

      // Call controller via provider (application layer)
      final result = await controller.createForSaleIfAuthorized(
        request,
        authState,
      );

      if (!mounted) return;

      if (result.isSuccess && result.data != null) {
        final forSale = result.data!;
        // Show success and navigate back with forSale data
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              'ForSale tayang dengan ${_selectedShippingSetupIds.length} opsi pengiriman.',
            ),
            backgroundColor: AppColors.successGreen,
            duration: const Duration(seconds: 3),
          ),
        );
        Navigator.of(context).pop(forSale); // Return created forSale
      } else if (result.errorCode == api_codes.emailVerificationRequired) {
        // Backend-rejection handler (defense-in-depth): the backend stays
        // the single authority for EMAIL_VERIFICATION_REQUIRED.
        AppSnackBar.showError(
          context,
          'Verifikasi email kamu diperlukan sebelum membuat iklan.',
        );
      } else {
        final consumed = CommerceRestrictionPresenter.handle(
          context,
          errorCode: result.errorCode,
          actionDescription: 'membuat forSale',
        );
        if (!consumed) {
          setState(() => _errorMessage = result.error ?? 'Gagal membuat forSale');
        }
      }
    } catch (e) {
      setState(() => _errorMessage = 'Terjadi kesalahan: ${e.toString()}');
    } finally {
      if (mounted) {
        setState(() => _isSubmitting = false);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authControllerProvider);

    switch (authState) {
      case AuthStateInitial():
      case AuthStateLoading():
      case AuthStateFirebaseAuthenticated():
      case AuthStateSyncingWithBackend():
      case AuthStateBackendFailure():
      case AuthStateBackendUnavailable():
      case AuthStateError():
        return _buildLoadingScaffold(context);

      case AuthStateUnauthenticated():
        return _buildAccessGate(
          context,
          title: 'Login Diperlukan',
          message: 'Silakan login untuk melanjutkan.',
          buttonLabel: 'Masuk',
          onPressed: () => context.push('/auth/sign-in'),
        );

      // D2 hard gate: unverified sessions never reach an authenticated
      // surface; the router parks them on the verify-email screen.
      case AuthStatePendingEmailVerification():
        return _buildLoadingScaffold(context);

      case AuthStateRequiresProfileCompletion():
        return const CompleteProfileScreen();

      case AuthStateAccountRestricted():
        return const AccountRestrictedScreen();

      case AuthStateAuthenticated(:final user):
        if (user.hasSellerProfile != true) {
          return _buildAccessGate(
            context,
            title: 'Jadi Seller Dulu',
            message:
                'Untuk membuat forSale, kamu perlu membuat seller profile terlebih dahulu.',
            buttonLabel: 'Mulai Jualan',
            onPressed: () => context.push(RoutePaths.sellerUpgrade),
          );
        }

        // CREATE = PUBLISH: an expired seller cannot create at all. Same
        // gate and same renewal CTA as the auction create screen.
        if (user.hasMarketAuthority != true) {
          final isExpired = ref.watch(isSellerSubscriptionExpiredProvider);
          return _buildAccessGate(
            context,
            title: isExpired
                ? 'Langganan Seller Habis'
                : 'Langganan Belum Aktif',
            message: isExpired
                ? 'Aktifkan kembali langganan seller agar bisa membuat forSale.'
                : 'Aktifkan langganan seller agar bisa membuat forSale.',
            buttonLabel: isExpired
                ? 'Perpanjang Langganan'
                : 'Aktifkan Langganan',
            onPressed: () => context.push(RoutePaths.sellerRenewal),
          );
        }

        return _buildFormScaffold(context);
    }
  }

  Widget _buildLoadingScaffold(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Scaffold(
      backgroundColor: isDark ? AppColors.darkGray900 : AppColors.neutralGray50,
      appBar: AppBar(
        title: const Text('Buat ForSale Baru'),
        backgroundColor: isDark
            ? AppColors.darkGray800
            : AppColors.neutralWhite,
        foregroundColor: isDark
            ? AppColors.neutralWhite
            : AppColors.neutralGray900,
        elevation: 0,
        surfaceTintColor: Colors.transparent,
        scrolledUnderElevation: 0,
      ),
      body: const Center(child: CircularProgressIndicator()),
    );
  }

  Widget _buildAccessGate(
    BuildContext context, {
    required String title,
    required String message,
    required String buttonLabel,
    required VoidCallback onPressed,
  }) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Scaffold(
      backgroundColor: isDark ? AppColors.darkGray900 : AppColors.neutralGray50,
      appBar: AppBar(
        title: const Text('Buat ForSale Baru'),
        backgroundColor: isDark
            ? AppColors.darkGray800
            : AppColors.neutralWhite,
        foregroundColor: isDark
            ? AppColors.neutralWhite
            : AppColors.neutralGray900,
        elevation: 0,
        surfaceTintColor: Colors.transparent,
        scrolledUnderElevation: 0,
      ),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                Icons.lock_outline,
                size: 56,
                color: isDark
                    ? AppColors.neutralGray300
                    : AppColors.neutralGray700,
              ),
              const SizedBox(height: 16),
              Text(
                title,
                style: const TextStyle(
                  fontSize: 20,
                  fontWeight: FontWeight.w700,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 8),
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
              const SizedBox(height: 20),
              ElevatedButton(
                onPressed: onPressed,
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

  Widget _buildFormScaffold(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Scaffold(
      backgroundColor: isDark ? AppColors.darkGray900 : AppColors.neutralGray50,
      appBar: AppBar(
        title: const Text('Buat ForSale Baru'),
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => Navigator.of(context).pop(),
        ),
        backgroundColor: isDark
            ? AppColors.darkGray800
            : AppColors.neutralWhite,
        foregroundColor: isDark
            ? AppColors.neutralWhite
            : AppColors.neutralGray900,
        elevation: 0,
        surfaceTintColor: Colors.transparent,
        scrolledUnderElevation: 0,
      ),
      body: Form(
        key: _formKey,
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            // Basic Info Section
            const _SectionTitle('Informasi Dasar'),
            const SizedBox(height: 12),
            _TitleField(controller: _titleController),
            const SizedBox(height: 16),
            _DescriptionField(controller: _descriptionController),

            const SizedBox(height: 24),

            // Media Upload Section — foto+video via 1 mesin (orchestrator)
            const _SectionTitle('Media Produk'),
            const SizedBox(height: 12),
            MediaGridUploader(
              mediaUrls: _mediaUrls,
              onMediaAdded: (url) => setState(() => _mediaUrls.add(url)),
              onMediaRemoved: (index) =>
                  setState(() => _mediaUrls.removeAt(index)),
            ),

            const SizedBox(height: 24),

            // Price & Negotiable (required for forSales)
            const _SectionTitle('Harga'),
            const SizedBox(height: 12),
            _PriceField(
              initialValue: _price,
              onChanged: (value) => setState(() => _price = value),
            ),
            const SizedBox(height: 16),
            _NegotiableToggle(
              initialValue: _isNegotiable,
              onChanged: (value) => setState(() => _isNegotiable = value),
            ),
            const SizedBox(height: 16),
            _StockField(
              initialValue: _quantity,
              onChanged: (value) => setState(() => _quantity = value),
            ),

            const SizedBox(height: 24),

            // Koi Details (required for forSales)
            const _SectionTitle('Detail Koi'),
            const SizedBox(height: 12),
            _KoiDetailsForm(
              variety: _variety,
              sizeInCm: _sizeInCm,
              ageInMonths: _ageInMonths,
              gender: _gender,
              breeder: _breeder,
              bloodline: _bloodline,
              onVarietyChanged: (value) => setState(() => _variety = value),
              onSizeChanged: (value) => setState(() => _sizeInCm = value),
              onAgeChanged: (value) => setState(() => _ageInMonths = value),
              onGenderChanged: (value) => setState(() => _gender = value),
              onBreederChanged: (value) => setState(() => _breeder = value),
              onBloodlineChanged: (value) => setState(() => _bloodline = value),
            ),

            const SizedBox(height: 24),

            // Shipping Readiness Section
            const _SectionTitle('Kesiapan Pengiriman'),
            const SizedBox(height: 8),
            Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: Text(
                'Informasikan kepada pembeli berapa lama waktu yang Anda butuhkan untuk menyiapkan ikan sebelum dikirim.',
                style: TextStyle(fontSize: 13, color: AppColors.neutralGray600),
              ),
            ),
            _PreparationTimeSelector(
              selected: _preparationTime,
              onChanged: (value) => setState(() => _preparationTime = value),
            ),
            const SizedBox(height: 12),
            _PreparationNoteField(controller: _preparationNoteController),

            const SizedBox(height: 24),

            // Phase 2: ForSale-level shipping option subset
            const _SectionTitle('Opsi Pengiriman untuk ForSale Ini'),
            const SizedBox(height: 8),
            SellerShippingSetupsSelector(
              helperText:
                  'Pilih opsi pengiriman dari katalog Anda yang berlaku untuk '
                  'forSale ini. Pembeli hanya bisa memilih dari opsi terpilih. '
                  'Untuk kasus khusus, gunakan kirim quote di chat.',
              onSelectionChanged: (ids) =>
                  setState(() => _selectedShippingSetupIds = ids),
            ),

            const SizedBox(height: 32),

            // Error message
            if (_errorMessage != null)
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

            // Submit button — disabled until the form is complete.
            ElevatedButton(
              onPressed: _canSubmit ? _submitForm : null,
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
                        valueColor: AlwaysStoppedAnimation<Color>(Colors.white),
                      ),
                    )
                  : const Text(
                      'Publikasikan ForSale',
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w600,
                        color: Colors.white,
                      ),
                    ),
            ),

            const SizedBox(height: 32),
          ],
        ),
      ),
    );
  }

  /// Blocked-action message for a failed [ForSaleController.canCreateForSale].
  ///
  /// Reaching this means the principal lacks market authority — no usable
  /// session, no seller profile, or no active seller subscription.
  String _createForSaleAccessMessage(AuthState authState) {
    return switch (authState) {
      AuthStateAuthenticated(:final user) when user.hasSellerProfile != true =>
        'Buat seller profile dulu untuk membuat forSale.',
      AuthStateAuthenticated(:final user) when user.hasMarketAuthority != true =>
        'Langganan seller belum aktif atau sudah berakhir. Perpanjang dulu untuk membuat forSale.',
      _ => 'Sesi autentikasi belum siap untuk membuat forSale.',
    };
  }
}

// ========================================================================
// INTERNAL WIDGETS (UI only components)
// ========================================================================

class _SectionTitle extends StatelessWidget {
  final String title;

  const _SectionTitle(this.title);

  @override
  Widget build(BuildContext context) {
    return Text(
      title,
      style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
    );
  }
}

class _TitleField extends StatelessWidget {
  final TextEditingController controller;

  const _TitleField({required this.controller});

  @override
  Widget build(BuildContext context) {
    return TextFormField(
      controller: controller,
      decoration: const InputDecoration(
        labelText: 'Judul *',
        hintText: 'Contoh: Kohaku 50cm Grade A',
        border: OutlineInputBorder(),
      ),
      validator: (value) {
        if (value == null || value.trim().isEmpty) {
          return 'Judul wajib diisi';
        }
        if (value.trim().length > 100) {
          return 'Judul maksimal 100 karakter';
        }
        return null;
      },
    );
  }
}

class _DescriptionField extends StatelessWidget {
  final TextEditingController controller;

  const _DescriptionField({required this.controller});

  @override
  Widget build(BuildContext context) {
    return TextFormField(
      controller: controller,
      maxLines: 4,
      decoration: const InputDecoration(
        labelText: 'Deskripsi',
        hintText: 'Ceritakan tentang koi Anda...',
        border: OutlineInputBorder(),
      ),
      validator: (value) {
        if (value != null && value.trim().length > 2000) {
          return 'Deskripsi maksimal 2000 karakter';
        }
        return null;
      },
    );
  }
}

class _NegotiableToggle extends StatelessWidget {
  final bool initialValue;
  final void Function(bool) onChanged;

  const _NegotiableToggle({
    required this.initialValue,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    return SwitchListTile(
      title: const Text('Bisa Nego'),
      subtitle: const Text('Pembeli dapat melakukan negosiasi harga'),
      value: initialValue,
      onChanged: onChanged,
      activeTrackColor: AppColors.primaryRed,
    );
  }
}

/// Stock/quantity field.
///
/// Defaults to 1 (unique item — most koi forSales are one-of-a-kind).
/// Sellers with multiple units of the same product increase this to enable
/// stock-based sale; buyers can then purchase up to the available amount.
class _StockField extends StatefulWidget {
  final int initialValue;
  final void Function(int) onChanged;

  const _StockField({required this.initialValue, required this.onChanged});

  @override
  State<_StockField> createState() => _StockFieldState();
}

class _StockFieldState extends State<_StockField> {
  late final TextEditingController _controller;

  @override
  void initState() {
    super.initState();
    _controller = TextEditingController(text: widget.initialValue.toString());
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return TextFormField(
      controller: _controller,
      keyboardType: TextInputType.number,
      decoration: const InputDecoration(
        labelText: 'Stok *',
        hintText: 'Jumlah tersedia',
        helperText:
            'Untuk koi unik, biarkan 1. Untuk produk stok, sesuaikan jumlahnya.',
        border: OutlineInputBorder(),
      ),
      validator: (value) {
        if (value == null || value.trim().isEmpty) {
          return 'Stok wajib diisi';
        }
        final stock = int.tryParse(value);
        if (stock == null || stock < 1) {
          return 'Stok minimal 1';
        }
        return null;
      },
      onChanged: (value) {
        final stock = int.tryParse(value);
        if (stock != null && stock >= 1) {
          widget.onChanged(stock);
        }
      },
    );
  }
}

class _PriceField extends StatefulWidget {
  final double? initialValue;
  final void Function(double?) onChanged;

  const _PriceField({required this.initialValue, required this.onChanged});

  @override
  State<_PriceField> createState() => _PriceFieldState();
}

class _PriceFieldState extends State<_PriceField> {
  final _controller = TextEditingController();

  @override
  void initState() {
    super.initState();
    if (widget.initialValue != null) {
      _controller.text = widget.initialValue.toString();
    }
  }

  @override
  Widget build(BuildContext context) {
    return TextFormField(
      controller: _controller,
      keyboardType: TextInputType.number,
      decoration: const InputDecoration(
        labelText: 'Harga (Rp) *',
        hintText: 'Contoh: 500000',
        prefixText: 'Rp ',
        border: OutlineInputBorder(),
      ),
      validator: (value) {
        if (value == null || value.isEmpty) {
          return 'Harga wajib diisi';
        }
        final price = double.tryParse(value);
        if (price == null || price < 10000) {
          return 'Minimal harga Rp 10.000';
        }
        return null;
      },
      onChanged: (value) {
        widget.onChanged(double.tryParse(value));
      },
    );
  }
}

class _KoiDetailsForm extends StatelessWidget {
  final String? variety;
  final double? sizeInCm;
  final int? ageInMonths;
  final String? gender;
  final String? breeder;
  final String? bloodline;
  final void Function(String) onVarietyChanged;
  final void Function(double?) onSizeChanged;
  final void Function(int?) onAgeChanged;
  final void Function(String) onGenderChanged;
  final void Function(String) onBreederChanged;
  final void Function(String) onBloodlineChanged;

  const _KoiDetailsForm({
    required this.variety,
    required this.sizeInCm,
    required this.ageInMonths,
    required this.gender,
    required this.breeder,
    required this.bloodline,
    required this.onVarietyChanged,
    required this.onSizeChanged,
    required this.onAgeChanged,
    required this.onGenderChanged,
    required this.onBreederChanged,
    required this.onBloodlineChanged,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        // Variety dropdown (show all varieties)
        DropdownButtonFormField<String>(
          initialValue: variety,
          decoration: const InputDecoration(
            labelText: 'Varietas *',
            border: OutlineInputBorder(),
          ),
          items: _koiVarieties.map((v) {
            return DropdownMenuItem(value: v, child: Text(v));
          }).toList(),
          onChanged: (value) {
            if (value != null) onVarietyChanged(value);
          },
          validator: (value) =>
              value == null || value.isEmpty ? 'Varietas wajib diisi' : null,
        ),

        const SizedBox(height: 16),

        // Size field
        TextFormField(
          initialValue: sizeInCm?.toString(),
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
          onChanged: (value) => onSizeChanged(double.tryParse(value)),
        ),

        const SizedBox(height: 16),

        // Age field
        TextFormField(
          initialValue: ageInMonths?.toString(),
          keyboardType: TextInputType.number,
          decoration: const InputDecoration(
            labelText: 'Usia (bulan)',
            hintText: 'Contoh: 24',
            suffixText: 'bulan',
            border: OutlineInputBorder(),
          ),
          onChanged: (value) => onAgeChanged(int.tryParse(value)),
        ),

        const SizedBox(height: 16),

        // Gender dropdown
        DropdownButtonFormField<String>(
          initialValue: gender,
          decoration: const InputDecoration(
            labelText: 'Jenis Kelamin',
            border: OutlineInputBorder(),
          ),
          items: _koiGenders.map((g) {
            return DropdownMenuItem(
              value: g['value'],
              child: Text(g['label'] as String),
            );
          }).toList(),
          onChanged: (value) {
            if (value != null) onGenderChanged(value);
          },
        ),

        const SizedBox(height: 16),

        // Breeder field
        TextFormField(
          initialValue: breeder,
          decoration: const InputDecoration(
            labelText: 'Breeder',
            hintText: 'Nama breeder',
            border: OutlineInputBorder(),
          ),
          onChanged: (value) => onBreederChanged(value.trim()),
        ),

        const SizedBox(height: 16),

        // Bloodline field
        TextFormField(
          initialValue: bloodline,
          decoration: const InputDecoration(
            labelText: 'Bloodline',
            hintText: 'Keturunan/bloodline',
            border: OutlineInputBorder(),
          ),
          onChanged: (value) => onBloodlineChanged(value.trim()),
        ),
      ],
    );
  }
}

// Koi varieties list
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

// Koi genders - API string values
const _koiGenders = [
  {'value': 'male', 'label': 'Jantan'},
  {'value': 'female', 'label': 'Betina'},
  {'value': 'unknown', 'label': 'Tidak Diketahui'},
];

/// Preparation time selector widget
class _PreparationTimeSelector extends StatelessWidget {
  final PreparationTime selected;
  final void Function(PreparationTime) onChanged;

  const _PreparationTimeSelector({
    required this.selected,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: AppColors.neutralGray100,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: AppColors.neutralGray300),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'Waktu Persiapan *',
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w600,
              color: AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 12),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: PreparationTime.values.map((time) {
              final isSelected = selected == time;
              return InkWell(
                onTap: () => onChanged(time),
                borderRadius: BorderRadius.circular(20),
                child: Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 16,
                    vertical: 10,
                  ),
                  decoration: BoxDecoration(
                    color: isSelected
                        ? AppColors.primaryRed
                        : AppColors.neutralWhite,
                    borderRadius: BorderRadius.circular(20),
                    border: Border.all(
                      color: isSelected
                          ? AppColors.primaryRed
                          : AppColors.neutralGray300,
                    ),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      if (isSelected)
                        const Icon(
                          Icons.check_circle,
                          size: 16,
                          color: Colors.white,
                        )
                      else
                        Icon(
                          Icons.radio_button_unchecked,
                          size: 16,
                          color: AppColors.neutralGray600,
                        ),
                      const SizedBox(width: 6),
                      Text(
                        time.displayName,
                        style: TextStyle(
                          color: isSelected
                              ? Colors.white
                              : AppColors.neutralGray900,
                          fontSize: 13,
                          fontWeight: isSelected
                              ? FontWeight.w600
                              : FontWeight.normal,
                        ),
                      ),
                    ],
                  ),
                ),
              );
            }).toList(),
          ),
          const SizedBox(height: 8),
          Text(
            selected.description,
            style: const TextStyle(
              fontSize: 12,
              color: AppColors.neutralGray600,
            ),
          ),
        ],
      ),
    );
  }
}

/// Preparation note field widget
class _PreparationNoteField extends StatelessWidget {
  final TextEditingController controller;

  const _PreparationNoteField({required this.controller});

  @override
  Widget build(BuildContext context) {
    return TextFormField(
      controller: controller,
      maxLines: 2,
      maxLength: 200,
      decoration: InputDecoration(
        labelText: 'Catatan Persiapan (Opsional)',
        hintText:
            'Contoh: Lolos karantina siap kirim, Butuh puasa 2 hari sebelum packing',
        border: const OutlineInputBorder(),
        helperText: 'Informasikan kondisi khusus yang pembeli perlu tahu',
      ),
    );
  }
}
