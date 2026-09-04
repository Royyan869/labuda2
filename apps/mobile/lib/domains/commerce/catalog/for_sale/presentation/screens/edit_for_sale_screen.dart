/// Edit ForSale Screen
///
/// Allows sellers to edit their existing forSales.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/utils/media_extensions.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/widgets/for_sale_media_handler.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/widgets/seller_shipping_options_selector.dart';

// Koi varieties
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

/// Edit ForSale Screen
class EditForSaleScreen extends ConsumerStatefulWidget {
  final String forSaleId;

  const EditForSaleScreen({super.key, required this.forSaleId});

  @override
  ConsumerState<EditForSaleScreen> createState() => _EditForSaleScreenState();
}

class _EditForSaleScreenState extends ConsumerState<EditForSaleScreen> {
  final _formKey = GlobalKey<FormState>();
  final _titleController = TextEditingController();
  final _descriptionController = TextEditingController();

  // Form state
  bool _isNegotiable = true;
  double? _price;
  int _quantity = 1;
  final List<String> _mediaUrls = [];

  // Koi details
  String? _variety;
  double? _sizeInCm;
  int? _ageInMonths;
  String? _gender;
  String? _breeder;
  String? _bloodline;

  bool _isSubmitting = false;
  String? _errorMessage;
  ForSale? _originalListing;

  // Phase 2: re-selectable shipping subset for this listing. The backend has
  // no GET endpoint for current selection, so we let the seller re-pick from
  // scratch; on save, an empty selection means "no change requested" (we
  // skip the PUT call) and a non-empty selection means "overwrite the
  // server-side subset with this list".
  List<String> _selectedShippingSetupIds = const [];
  bool _shippingSelectionDirty = false;

  @override
  void initState() {
    super.initState();
    _loadListing();
  }

  @override
  void dispose() {
    _titleController.dispose();
    _descriptionController.dispose();
    super.dispose();
  }

  Future<void> _loadListing() async {
    final controller = ref.read(forSaleControllerProvider);
    final result = await controller.getForSaleById(widget.forSaleId);

    if (mounted) {
      result.fold(
        (error) {
          setState(() => _errorMessage = 'Gagal memuat listing: $error');
        },
        (listing) {
          if (listing != null) {
            // Check ownership
            final authState = ref.read(authControllerProvider);
            if (authState is AuthStateAuthenticated &&
                authState.user.id == listing.sellerId) {
              setState(() {
                _originalListing = listing;
                _titleController.text = listing.title;
                _descriptionController.text = listing.description;
                _price = listing.price;
                _isNegotiable = listing.price > 0;
                _quantity = listing.stock > 0 ? listing.stock : 1;
                _mediaUrls.clear();
                _mediaUrls.addAll(listing.media.urls);
                _variety = listing.variety;
                _sizeInCm = listing.sizeCm;
                _ageInMonths = listing.ageMonths;
                _gender = listing.gender;
                _breeder = listing.breeder;
                _bloodline = listing.bloodline;
              });
            } else {
              setState(
                () => _errorMessage =
                    'Anda tidak memiliki izin untuk mengedit listing ini',
              );
            }
          }
        },
      );
    }
  }

  Future<void> _submitForm() async {
    if (!_formKey.currentState!.validate()) {
      return;
    }

    if (_mediaUrls.isEmpty) {
      setState(() => _errorMessage = 'Minimal 1 media wajib diupload');
      return;
    }

    if (_price == null) {
      setState(() => _errorMessage = 'Harga wajib diisi');
      return;
    }
    if (_variety == null || _sizeInCm == null) {
      setState(() => _errorMessage = 'Detail koi wajib diisi');
      return;
    }

    setState(() => _isSubmitting = true);
    _errorMessage = null;

    try {
      final request = UpdateForSaleRequest(
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
      );

      final controller = ref.read(forSaleControllerProvider);
      final result = await controller.updateForSale(widget.forSaleId, request);

      if (!mounted) return;

      if (!result.isSuccess) {
        setState(() => _errorMessage = result.error);
        return;
      }

      // Phase 2: only call PUT /listings/:id/shipping when the seller actually
      // re-picked. Leaving the section untouched preserves the existing
      // server-side subset (avoids accidentally clearing all options).
      if (_shippingSelectionDirty) {
        final productId = _originalListing?.productId;
        if (productId == null || productId.isEmpty) {
          setState(() {
            _errorMessage =
                'Listing tersimpan, tetapi product_id belum tersedia untuk memperbarui opsi pengiriman.';
          });
          return;
        }
        final linkResult = await ref
            .read(shippingRepositoryProvider)
            .setProductShippingSetups(productId, _selectedShippingSetupIds);
        if (!mounted) return;
        if (linkResult.isError) {
          // Backend rejects when there are active orders (or when option
          // IDs don't belong to the seller). Surface the message; the
          // listing itself was already updated successfully.
          setState(() {
            _errorMessage =
                'Listing tersimpan, tapi opsi pengiriman gagal diperbarui: '
                '${linkResult.error ?? 'kesalahan tidak diketahui'}.';
          });
          return;
        }
      }

      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Listing berhasil diperbarui'),
          backgroundColor: AppColors.successGreen,
        ),
      );
      Navigator.of(context).pop(true);
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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    if (_originalListing == null && _errorMessage == null) {
      return Scaffold(
        appBar: AppBar(
          title: const Text('Edit Listing'),
          backgroundColor: isDark
              ? AppColors.darkGray800
              : AppColors.neutralWhite,
          foregroundColor: isDark
              ? AppColors.neutralWhite
              : AppColors.neutralGray900,
        ),
        body: const Center(child: CircularProgressIndicator()),
      );
    }

    if (_errorMessage != null && _errorMessage!.contains('izin')) {
      return Scaffold(
        appBar: AppBar(
          title: const Text('Edit Listing'),
          backgroundColor: isDark
              ? AppColors.darkGray800
              : AppColors.neutralWhite,
        ),
        body: Center(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                const Icon(Icons.lock, size: 64, color: AppColors.primaryRed),
                const SizedBox(height: 16),
                Text(
                  _errorMessage!,
                  style: const TextStyle(fontSize: 16),
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: 24),
                ElevatedButton(
                  onPressed: () => Navigator.of(context).pop(),
                  child: const Text('Kembali'),
                ),
              ],
            ),
          ),
        ),
      );
    }

    return Scaffold(
      backgroundColor: isDark ? AppColors.darkGray900 : AppColors.neutralGray50,
      appBar: AppBar(
        title: const Text('Edit Listing'),
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
        actions: [
          TextButton(
            onPressed: _isSubmitting ? null : _submitForm,
            child: _isSubmitting
                ? const SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      valueColor: AlwaysStoppedAnimation<Color>(
                        AppColors.primaryRed,
                      ),
                    ),
                  )
                : const Text(
                    'Simpan',
                    style: TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.w600,
                      color: AppColors.primaryRed,
                    ),
                  ),
          ),
        ],
      ),
      body: Form(
        key: _formKey,
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            const _SectionTitle('Informasi Dasar'),
            const SizedBox(height: 12),
            _TitleField(controller: _titleController),
            const SizedBox(height: 16),
            _DescriptionField(controller: _descriptionController),

            const SizedBox(height: 24),

            const _SectionTitle('Media Produk'),
            const SizedBox(height: 12),
            _MediaUploadSection(
              mediaUrls: _mediaUrls,
              onMediaAdded: (url) => setState(() => _mediaUrls.add(url)),
              onMediaRemoved: (index) =>
                  setState(() => _mediaUrls.removeAt(index)),
            ),

            const SizedBox(height: 24),

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

            // Phase 2: listing-level shipping subset re-selector.
            const _SectionTitle('Opsi Pengiriman untuk Listing Ini'),
            const SizedBox(height: 8),
            SellerShippingSetupsSelector(
              helperText:
                  'Pilih ulang opsi pengiriman yang berlaku untuk listing ini. '
                  'Selama tidak diubah, opsi pengiriman saat ini tetap aktif. '
                  'Backend menolak perubahan jika listing memiliki pesanan aktif.',
              onSelectionChanged: (ids) => setState(() {
                _selectedShippingSetupIds = ids;
                _shippingSelectionDirty = true;
              }),
            ),

            const SizedBox(height: 32),

            if (_errorMessage != null && !_errorMessage!.contains('izin'))
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

            const SizedBox(height: 32),
          ],
        ),
      ),
    );
  }
}

// ============================================================================
// INTERNAL WIDGETS
// ============================================================================

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
                    Text('Tap untuk upload foto/video'),
                    Text('(Minimal 1 media)', style: TextStyle(fontSize: 12)),
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
/// Backend rejects changing this field once orders exist for the listing.
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
