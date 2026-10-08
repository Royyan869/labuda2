import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';
import 'package:labuda/shared/widgets/app_text_field.dart';

/// Scheduled-auction edit screen.
///
/// BACKEND CONTRACT (create = publish): there is no draft state — scheduled
/// is the only editable state, and only title/description may change.
/// Pricing, media, variety and preparation are immutable after create.
class SellerAuctionEditScreen extends ConsumerStatefulWidget {
  final Auction auction;

  const SellerAuctionEditScreen({super.key, required this.auction});

  @override
  ConsumerState<SellerAuctionEditScreen> createState() =>
      _SellerAuctionEditScreenState();
}

class _SellerAuctionEditScreenState
    extends ConsumerState<SellerAuctionEditScreen> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _titleController;
  late final TextEditingController _descriptionController;
  bool _isSubmitting = false;
  String? _errorMessage;

  @override
  void initState() {
    super.initState();
    _titleController = TextEditingController(text: widget.auction.title);
    _descriptionController = TextEditingController(
      text: widget.auction.description,
    );
  }

  @override
  void dispose() {
    _titleController.dispose();
    _descriptionController.dispose();
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

    if (widget.auction.status != AuctionStatus.scheduled) {
      setState(() {
        _errorMessage = 'Hanya lelang terjadwal yang bisa diedit.';
      });
      return;
    }

    setState(() {
      _isSubmitting = true;
      _errorMessage = null;
    });

    final success = await ref
        .read(auctionNotifierProvider.notifier)
        .updateAuction(widget.auction.id, {
          'title': _titleController.text.trim(),
          'description': _descriptionController.text.trim(),
        });

    if (!mounted) return;

    if (!success) {
      setState(() {
        _isSubmitting = false;
        _errorMessage =
            ref.read(auctionNotifierProvider).error ??
            'Gagal menyimpan perubahan.';
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
    final canEdit =
        isOwner && widget.auction.status == AuctionStatus.scheduled;

    return Scaffold(
      appBar: AppBar(title: const Text('Edit Lelang')),
      body: SafeArea(
        child: canEdit
            ? Form(
                key: _formKey,
                child: ListView(
                  padding: const EdgeInsets.all(AppMetrics.p16),
                  children: [
                    Text(
                      'Hanya lelang terjadwal milik Anda yang bisa diedit '
                      '(judul dan deskripsi).',
                      style: Theme.of(context).textTheme.bodyMedium,
                    ),
                    const SizedBox(height: 16),
                    AppTextField(
                      controller: _titleController,
                      labelText: 'Judul',
                      validator: (value) {
                        if (value == null || value.trim().isEmpty) {
                          return 'Judul wajib diisi';
                        }
                        return null;
                      },
                    ),
                    const SizedBox(height: 12),
                    AppTextField(
                      controller: _descriptionController,
                      maxLines: 4,
                      labelText: 'Deskripsi',
                      validator: (value) {
                        if (value == null || value.trim().isEmpty) {
                          return 'Deskripsi wajib diisi';
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
                          : const Text('Simpan Perubahan'),
                    ),
                  ],
                ),
              )
            : Center(
                child: Padding(
                  padding: const EdgeInsets.all(AppMetrics.p24),
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Icon(
                        Icons.lock_outline,
                        size: AppIconSize.display,
                        color: Theme.of(context).colorScheme.onSurfaceVariant,
                      ),
                      const SizedBox(height: 12),
                      Text(
                        isOwner
                            ? 'Hanya lelang terjadwal yang bisa diedit.'
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
