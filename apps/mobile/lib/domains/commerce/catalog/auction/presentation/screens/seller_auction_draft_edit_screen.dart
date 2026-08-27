import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';

class SellerAuctionDraftEditScreen extends ConsumerStatefulWidget {
  final Auction auction;

  const SellerAuctionDraftEditScreen({super.key, required this.auction});

  @override
  ConsumerState<SellerAuctionDraftEditScreen> createState() =>
      _SellerAuctionDraftEditScreenState();
}

class _SellerAuctionDraftEditScreenState
    extends ConsumerState<SellerAuctionDraftEditScreen> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _titleController;
  late final TextEditingController _descriptionController;
  late final TextEditingController _openingBidController;
  late final TextEditingController _bidIncrementController;
  late final TextEditingController _buyNowPriceController;
  bool _isSubmitting = false;
  String? _errorMessage;

  @override
  void initState() {
    super.initState();
    _titleController = TextEditingController(text: widget.auction.title);
    _descriptionController = TextEditingController(
      text: widget.auction.description,
    );
    _openingBidController = TextEditingController(
      text: widget.auction.openingBid.toString(),
    );
    _bidIncrementController = TextEditingController(
      text: widget.auction.bidIncrement.toString(),
    );
    _buyNowPriceController = TextEditingController(
      text: widget.auction.buyNowPrice?.toString() ?? '',
    );
  }

  @override
  void dispose() {
    _titleController.dispose();
    _descriptionController.dispose();
    _openingBidController.dispose();
    _bidIncrementController.dispose();
    _buyNowPriceController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;

    final authState = ref.read(authControllerProvider);
    final currentUser = switch (authState) {
      AuthStateAuthenticated(:final user) => user,
      _ => null,
    };

    if (currentUser == null || currentUser.id != widget.auction.sellerId) {
      setState(() {
        _errorMessage = 'Anda tidak memiliki izin untuk mengedit lelang ini.';
      });
      return;
    }

    if (widget.auction.status != AuctionStatus.draft) {
      setState(() {
        _errorMessage = 'Hanya draft yang bisa diedit di workspace ini.';
      });
      return;
    }

    final title = _titleController.text.trim();
    final description = _descriptionController.text.trim();
    final openingBid = int.tryParse(_openingBidController.text.trim());
    final bidIncrement = int.tryParse(_bidIncrementController.text.trim());
    final buyNowPriceText = _buyNowPriceController.text.trim();
    final buyNowPrice = buyNowPriceText.isEmpty
        ? null
        : int.tryParse(buyNowPriceText);

    if (openingBid == null || bidIncrement == null) {
      setState(() {
        _errorMessage = 'Harga awal dan kenaikan bid harus berupa angka.';
      });
      return;
    }

    if (buyNowPriceText.isNotEmpty &&
        buyNowPrice == null) {
      setState(() {
        _errorMessage = 'Buy now price harus berupa angka bila diisi.';
      });
      return;
    }

    setState(() {
      _isSubmitting = true;
      _errorMessage = null;
    });

    final success = await ref.read(auctionNotifierProvider.notifier).updateAuction(
      widget.auction.id,
      {
        'title': title,
        'description': description,
        'startPrice': openingBid,
        'bidIncrement': bidIncrement,
        'buyNowPrice': buyNowPrice,
      },
    );

    if (!mounted) return;

    if (!success) {
      setState(() {
        _isSubmitting = false;
        _errorMessage =
            ref.read(auctionNotifierProvider).error ?? 'Gagal menyimpan draft.';
      });
      return;
    }

    Navigator.of(context).pop(true);
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authControllerProvider);
    final currentUser = switch (authState) {
      AuthStateAuthenticated(:final user) => user,
      _ => null,
    };
    final isOwner = currentUser?.id == widget.auction.sellerId;
    final canEdit = isOwner && widget.auction.status == AuctionStatus.draft;

    return Scaffold(
      appBar: AppBar(title: const Text('Edit Draft Lelang')),
      body: SafeArea(
        child: canEdit
            ? Form(
                key: _formKey,
                child: ListView(
                  padding: const EdgeInsets.all(16),
                  children: [
                    Text(
                      'Hanya draft milik Anda yang bisa diedit.',
                      style: Theme.of(context).textTheme.bodyMedium,
                    ),
                    const SizedBox(height: 16),
                    TextFormField(
                      controller: _titleController,
                      decoration: const InputDecoration(
                        labelText: 'Judul',
                        border: OutlineInputBorder(),
                      ),
                      validator: (value) {
                        if (value == null || value.trim().isEmpty) {
                          return 'Judul wajib diisi';
                        }
                        return null;
                      },
                    ),
                    const SizedBox(height: 12),
                    TextFormField(
                      controller: _descriptionController,
                      maxLines: 4,
                      decoration: const InputDecoration(
                        labelText: 'Deskripsi',
                        border: OutlineInputBorder(),
                      ),
                      validator: (value) {
                        if (value == null || value.trim().isEmpty) {
                          return 'Deskripsi wajib diisi';
                        }
                        return null;
                      },
                    ),
                    const SizedBox(height: 12),
                    TextFormField(
                      controller: _openingBidController,
                      keyboardType: TextInputType.number,
                      decoration: const InputDecoration(
                        labelText: 'Harga Awal',
                        border: OutlineInputBorder(),
                      ),
                      validator: (value) {
                        if (value == null || value.trim().isEmpty) {
                          return 'Harga awal wajib diisi';
                        }
                        if (int.tryParse(value.trim()) == null) {
                          return 'Harga awal harus berupa angka';
                        }
                        return null;
                      },
                    ),
                    const SizedBox(height: 12),
                    TextFormField(
                      controller: _bidIncrementController,
                      keyboardType: TextInputType.number,
                      decoration: const InputDecoration(
                        labelText: 'Kenaikan Bid',
                        border: OutlineInputBorder(),
                      ),
                      validator: (value) {
                        if (value == null || value.trim().isEmpty) {
                          return 'Kenaikan bid wajib diisi';
                        }
                        if (int.tryParse(value.trim()) == null) {
                          return 'Kenaikan bid harus berupa angka';
                        }
                        return null;
                      },
                    ),
                    const SizedBox(height: 12),
                    TextFormField(
                      controller: _buyNowPriceController,
                      keyboardType: TextInputType.number,
                      decoration: const InputDecoration(
                        labelText: 'Buy Now Price (opsional)',
                        border: OutlineInputBorder(),
                      ),
                      validator: (value) {
                        if (value == null || value.trim().isEmpty) return null;
                        if (int.tryParse(value.trim()) == null) {
                          return 'Buy now price harus berupa angka';
                        }
                        return null;
                      },
                    ),
                    const SizedBox(height: 16),
                    if (_errorMessage != null) ...[
                      Text(
                        _errorMessage!,
                        style: TextStyle(
                          color: Theme.of(context).colorScheme.error,
                        ),
                      ),
                      const SizedBox(height: 12),
                    ],
                    FilledButton(
                      onPressed: _isSubmitting ? null : _submit,
                      child: _isSubmitting
                          ? const SizedBox(
                              width: 16,
                              height: 16,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : const Text('Simpan Draft'),
                    ),
                  ],
                ),
              )
            : Center(
                child: Padding(
                  padding: const EdgeInsets.all(24),
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Icon(
                        Icons.lock_outline,
                        size: 48,
                        color: Theme.of(context).colorScheme.onSurfaceVariant,
                      ),
                      const SizedBox(height: 12),
                      Text(
                        isOwner
                            ? 'Hanya draft yang bisa diedit.'
                            : 'Anda tidak memiliki izin untuk mengedit lelang ini.',
                        textAlign: TextAlign.center,
                      ),
                    ],
                  ),
                ),
              ),
      ),
    );
  }
}
