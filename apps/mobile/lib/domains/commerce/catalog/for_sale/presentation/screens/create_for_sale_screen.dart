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
import 'package:labuda/core/common/types/preparation_time.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/widgets/for_sale_media_handler.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/widgets/seller_shipping_options_selector.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/widgets/blocked_action_gate.dart';

/// Create ForSale Screen
///
/// UI only - delegates all logic to forSale application layer.
class CreateForSaleScreen extends ConsumerStatefulWidget {
  /// Optional origin tracking for analytics
  /// - 'request_context': Created from a buyer request
  /// - 'chat_context': Created from chat conversation
  /// - null or 'direct_create': Created directly from seller dashboard
  final String? origin;

  const CreateForSaleScreen({super.key, this.origin});

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

  // Koi details (required for listings)
  String? _variety;
  double? _sizeInCm;
  int? _ageInMonths;
  String? _gender;
  String? _breeder;
  String? _bloodline;

  // Shipping readiness
  PreparationTime _preparationTime = PreparationTime.immediate;

  // Phase 2: shipping option IDs the seller selects to apply to this listing.
  // Drives the post-create PUT /listings/:id/shipping call.
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
      setState(() => _errorMessage = 'Minimal 1 foto wajib diupload');
      return;
    }

    // Validate required fields for listing
    if (_price == null) {
      setState(() => _errorMessage = 'Harga wajib diisi untuk listing');
      return;
    }
    if (_variety == null || _sizeInCm == null) {
      setState(() => _errorMessage = 'Detail koi wajib diisi untuk listing');
      return;
    }

    setState(() => _isSubmitting = true);
    _errorMessage = null;

    try {
      // Build request using listing domain model
      // NOTE: Create as draft (private visibility) - publish happens later with validation
      final request = CreateForSaleRequest(
        title: _titleController.text.trim(),
        description: _descriptionController.text.trim(),
        price: _price!,
        quantity: _quantity,
        negotiationEnabled: _isNegotiable,
        visibility: 'private', // Create as draft - publish separately
        mediaUrls: _mediaUrls,
        variety: _variety,
        sizeCm: _sizeInCm,
        ageMonths: _ageInMonths ?? 0,
        gender: _gender,
        breeder: _breeder,
        bloodline: _bloodline,
        origin: widget.origin,
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
        final listing = result.data!;
        // Phase 2: link the seller-selected shipping subset to the brand-new
        // listing. We only call PUT when the seller actually picked something;
        // an empty selection is permitted for draft, but the publish gate
        // (SHIPPING_NOT_CONFIGURED) will fire later if the seller never links.
        if (_selectedShippingSetupIds.isNotEmpty) {
          final productId = listing.productId;
          if (productId == null || productId.isEmpty) {
            setState(() {
              _errorMessage =
                  'Draft listing tersimpan, tetapi product_id belum tersedia untuk menautkan opsi pengiriman.';
            });
            return;
          }
          final linkResult = await ref
              .read(shippingRepositoryProvider)
              .setProductShippingSetups(productId, _selectedShippingSetupIds);
          if (!mounted) return;
          if (linkResult.isError) {
            // Listing exists (as draft) but shipping linking failed — be
            // explicit so the seller can retry from the edit screen.
            setState(() {
              _errorMessage =
                  'Draft listing tersimpan, tapi opsi pengiriman gagal ditautkan: '
                  '${linkResult.error ?? 'kesalahan tidak diketahui'}. '
                  'Buka Edit Listing untuk mencoba lagi sebelum publish.';
            });
            return;
          }
        }
        // Show success and navigate back with listing data
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              _selectedShippingSetupIds.isEmpty
                  ? 'Draft listing tersimpan. Pilih opsi pengiriman sebelum publish.'
                  : 'Draft listing tersimpan dengan ${_selectedShippingSetupIds.length} opsi pengiriman.',
            ),
            backgroundColor: AppColors.successGreen,
            duration: const Duration(seconds: 3),
          ),
        );
        Navigator.of(context).pop(listing); // Return created listing
      } else if (result.errorCode == 'EMAIL_VERIFICATION_REQUIRED') {
        // Defensive backend fail-close: keep honoring the server's rejection.
        await showBlockedActionGate(
          context,
          actionDescription: 'membuat listing',
        );
      } else {
        setState(() => _errorMessage = result.error ?? 'Gagal membuat listing');
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
                'Untuk membuat listing, kamu perlu membuat seller profile terlebih dahulu.',
            buttonLabel: 'Mulai Jualan',
            onPressed: () => context.push(RoutePaths.sellerUpgrade),
          );
        }

        if (user.hasMarketAuthority != true) {
          return _buildAccessGate(
            context,
            title: 'Langganan Seller Habis',
            message:
                'Aktifkan kembali langganan seller agar bisa membuat listing di mobile.',
            buttonLabel: 'Perpanjang Langganan',
            onPressed: () => context.push(RoutePaths.sellerUpgrade),
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
        title: const Text('Buat Listing Baru'),
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
        title: const Text('Buat Listing Baru'),
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
        title: const Text('Buat Listing Baru'),
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

            // Media Upload Section
            const _SectionTitle('Foto Produk'),
            const SizedBox(height: 12),
            _MediaUploadSection(
              mediaUrls: _mediaUrls,
              onMediaAdded: (url) => setState(() => _mediaUrls.add(url)),
              onMediaRemoved: (index) =>
                  setState(() => _mediaUrls.removeAt(index)),
            ),

            const SizedBox(height: 24),

            // Price & Negotiable (required for listings)
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

            // Koi Details (required for listings)
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

            // Phase 2: Listing-level shipping option subset
            const _SectionTitle('Opsi Pengiriman untuk Listing Ini'),
            const SizedBox(height: 8),
            SellerShippingSetupsSelector(
              helperText:
                  'Pilih opsi pengiriman dari katalog Anda yang berlaku untuk '
                  'listing ini. Pembeli hanya bisa memilih dari opsi terpilih. '
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

            // Submit button
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
                        valueColor: AlwaysStoppedAnimation<Color>(Colors.white),
                      ),
                    )
                  : const Text(
                      'Buat Listing',
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

  String _createForSaleAccessMessage(AuthState authState) {
    return switch (authState) {
      AuthStateAuthenticated(:final user) =>
        user.hasSellerProfile == true
            ? 'Langganan seller Anda sudah berakhir. Perpanjang dulu untuk membuat forSale.'
            : 'Buat seller profile dulu untuk membuat forSale.',
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

class _MediaUploadSection extends StatelessWidget {
  final List<String> mediaUrls;
  final void Function(String) onMediaAdded;
  final void Function(int) onMediaRemoved;

  const _MediaUploadSection({
    required this.mediaUrls,
    required this.onMediaAdded,
    required this.onMediaRemoved,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        if (mediaUrls.isEmpty)
          GestureDetector(
            onTap: () {
              ForSaleMediaHandler.showMediaPicker(
                context: context,
                currentMediaCount: mediaUrls.length,
                onMediaUploaded: (urls) async {
                  for (final url in urls) {
                    onMediaAdded(url);
                  }
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
            itemCount: mediaUrls.length + 1,
            itemBuilder: (context, index) {
              if (index < mediaUrls.length) {
                return _MediaTile(
                  url: mediaUrls[index],
                  onRemove: () => onMediaRemoved(index),
                );
              }
              return _AddMediaTile(
                onTap: () {
                  ForSaleMediaHandler.showMediaPicker(
                    context: context,
                    currentMediaCount: mediaUrls.length,
                    onMediaUploaded: (urls) async {
                      // Add uploaded URLs to the list
                      for (final url in urls) {
                        onMediaAdded(url);
                      }
                    },
                  );
                },
              );
            },
          ),
      ],
    );
  }
}

class _MediaTile extends StatelessWidget {
  final String url;
  final VoidCallback onRemove;

  const _MediaTile({required this.url, required this.onRemove});

  @override
  Widget build(BuildContext context) {
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
            onTap: onRemove,
            child: Container(
              padding: const EdgeInsets.all(4),
              decoration: const BoxDecoration(
                color: Colors.black54,
                shape: BoxShape.circle,
              ),
              child: const Icon(Icons.close, size: 16, color: Colors.white),
            ),
          ),
        ),
      ],
    );
  }
}

class _AddMediaTile extends StatelessWidget {
  final VoidCallback onTap;

  const _AddMediaTile({required this.onTap});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
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
/// Defaults to 1 (unique item — most koi listings are one-of-a-kind).
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
