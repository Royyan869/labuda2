import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/utils/app_formatters.dart';
import 'package:hishumi/shared/utils/money_input_formatter.dart';
import 'package:hishumi/shared/widgets/app_text_field.dart';
import 'package:hishumi/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:hishumi/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';

/// Auction relist (republish) screen.
///
/// BACKEND CONTRACT (owner decision, Oct 2026): relist IS republish — it
/// re-runs the create form's effect on a relistable auction (ended without
/// any bid/winner/order, or lapsed) with fresh timing/pricing. The backend
/// requires the full payload; there is no body-less relist call and no draft
/// detour.
class SellerAuctionRelistScreen extends ConsumerStatefulWidget {
  final Auction auction;

  const SellerAuctionRelistScreen({super.key, required this.auction});

  @override
  ConsumerState<SellerAuctionRelistScreen> createState() =>
      _SellerAuctionRelistScreenState();
}

class _SellerAuctionRelistScreenState
    extends ConsumerState<SellerAuctionRelistScreen> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _titleController;
  late final TextEditingController _descriptionController;
  late final TextEditingController _openingBidController;
  late final TextEditingController _bidIncrementController;
  late final TextEditingController _buyNowPriceController;

  /// Duration in days (backend bound: 1–7 days).
  int _durationDays = 3;

  /// 'now' or 'scheduled' — mirrors the create form's start modes.
  String _startMode = 'now';
  DateTime? _scheduledStartAt;

  bool _isSubmitting = false;
  String? _errorMessage;

  @override
  void initState() {
    super.initState();
    _titleController = TextEditingController(text: widget.auction.title);
    _descriptionController = TextEditingController(
      text: widget.auction.description,
    );
    // Seeded in the canonical money-input display form (grouped while the
    // business value below stays a plain int).
    _openingBidController = TextEditingController(
      text: MoneyInputFormatter.display(widget.auction.openingBid),
    );
    _bidIncrementController = TextEditingController(
      text: MoneyInputFormatter.display(widget.auction.bidIncrement),
    );
    _buyNowPriceController = TextEditingController(
      text: widget.auction.buyNowPrice == null
          ? ''
          : MoneyInputFormatter.display(widget.auction.buyNowPrice!),
    );

    // Keep the previous duration where it fits the backend bound (1–7 days).
    final previousHours = widget.auction.endTime
        .difference(widget.auction.startTime)
        .inHours;
    _durationDays = (previousHours / 24).round().clamp(1, 7);
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

  Future<void> _pickScheduledStartAt() async {
    final now = DateTime.now();
    final initial = _scheduledStartAt ?? now.add(const Duration(hours: 1));
    final picked = await showDatePicker(
      context: context,
      initialDate: initial,
      firstDate: now,
      lastDate: now.add(const Duration(days: 30)),
    );
    if (picked == null || !mounted) return;

    final time = await showTimePicker(
      context: context,
      initialTime: TimeOfDay.fromDateTime(initial),
    );
    if (time == null || !mounted) return;

    setState(() {
      _scheduledStartAt = DateTime(
        picked.year,
        picked.month,
        picked.day,
        time.hour,
        time.minute,
      );
    });
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;

    final openingBid = MoneyInputFormatter.parseAmount(
      _openingBidController.text,
    );
    final bidIncrement = MoneyInputFormatter.parseAmount(
      _bidIncrementController.text,
    );
    if (openingBid == null || bidIncrement == null) {
      setState(() {
        _errorMessage = 'Harga awal dan kenaikan bid harus berupa angka.';
      });
      return;
    }

    final buyNowPriceText = _buyNowPriceController.text.trim();
    final buyNowPrice = MoneyInputFormatter.parseAmount(buyNowPriceText);
    if (buyNowPriceText.isNotEmpty && buyNowPrice == null) {
      setState(() {
        _errorMessage = 'Buy now price harus berupa angka bila diisi.';
      });
      return;
    }

    if (_startMode == 'scheduled') {
      final scheduled = _scheduledStartAt;
      if (scheduled == null) {
        setState(() {
          _errorMessage = 'Pilih waktu mulai untuk mode terjadwal.';
        });
        return;
      }
      if (!scheduled.isAfter(DateTime.now())) {
        setState(() {
          _errorMessage = 'Waktu mulai harus di masa depan.';
        });
        return;
      }
    }

    setState(() {
      _isSubmitting = true;
      _errorMessage = null;
    });

    final success = await ref
        .read(auctionNotifierProvider.notifier)
        .relistAuction(
          auctionId: widget.auction.id,
          title: _titleController.text.trim(),
          description: _descriptionController.text.trim(),
          openingBid: openingBid,
          bidIncrement: bidIncrement,
          buyNowPrice: buyNowPrice,
          startMode: _startMode,
          scheduledStartAt: _startMode == 'scheduled'
              ? _scheduledStartAt
              : null,
          durationHours: _durationDays * 24,
        );

    if (!mounted) return;

    if (!success) {
      setState(() {
        _isSubmitting = false;
        _errorMessage =
            ref.read(auctionNotifierProvider).error ??
            'Lelang ini tidak bisa direlist.';
      });
      return;
    }

    Navigator.of(context).pop(true);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Relist Lelang')),
      body: SafeArea(
        child: Form(
          key: _formKey,
          child: ListView(
            padding: const EdgeInsets.all(AppMetrics.p16),
            children: [
              Text(
                'Relist = publish ulang dengan jadwal dan harga baru. '
                'Lelang langsung tayang sesuai mode mulai yang dipilih.',
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
              const SizedBox(height: 12),
              AppTextField(
                controller: _openingBidController,
                keyboardType: TextInputType.number,
                inputFormatters: const [MoneyInputFormatter()],
                labelText: 'Harga Awal',
                validator: (value) {
                  if (value == null || value.trim().isEmpty) {
                    return 'Harga awal wajib diisi';
                  }
                  if (MoneyInputFormatter.parseAmount(value) == null) {
                    return 'Harga awal harus berupa angka';
                  }
                  return null;
                },
              ),
              const SizedBox(height: 12),
              AppTextField(
                controller: _bidIncrementController,
                keyboardType: TextInputType.number,
                inputFormatters: const [MoneyInputFormatter()],
                labelText: 'Kenaikan Bid',
                validator: (value) {
                  if (value == null || value.trim().isEmpty) {
                    return 'Kenaikan bid wajib diisi';
                  }
                  if (MoneyInputFormatter.parseAmount(value) == null) {
                    return 'Kenaikan bid harus berupa angka';
                  }
                  return null;
                },
              ),
              const SizedBox(height: 12),
              AppTextField(
                controller: _buyNowPriceController,
                keyboardType: TextInputType.number,
                inputFormatters: const [MoneyInputFormatter()],
                labelText: 'Buy Now Price (opsional)',
                validator: (value) {
                  if (value == null || value.trim().isEmpty) return null;
                  if (MoneyInputFormatter.parseAmount(value) == null) {
                    return 'Buy now price harus berupa angka';
                  }
                  return null;
                },
              ),
              const SizedBox(height: 16),
              DropdownButtonFormField<int>(
                initialValue: _durationDays,
                decoration: const InputDecoration(
                  labelText: 'Durasi (hari)',
                ),
                items: [
                  for (var day = 1; day <= 7; day++)
                    DropdownMenuItem(value: day, child: Text('$day hari')),
                ],
                onChanged: (value) {
                  if (value != null) setState(() => _durationDays = value);
                },
              ),
              const SizedBox(height: 16),
              SegmentedButton<String>(
                segments: const [
                  ButtonSegment(value: 'now', label: Text('Mulai Sekarang')),
                  ButtonSegment(
                    value: 'scheduled',
                    label: Text('Jadwalkan'),
                  ),
                ],
                selected: {_startMode},
                onSelectionChanged: (selection) {
                  setState(() => _startMode = selection.first);
                },
              ),
              if (_startMode == 'scheduled') ...[
                const SizedBox(height: 12),
                OutlinedButton.icon(
                  onPressed: _pickScheduledStartAt,
                  icon: const Icon(Icons.schedule_outlined),
                  label: Text(
                    _scheduledStartAt == null
                        ? 'Pilih waktu mulai'
                        : 'Mulai: ${AppFormatters.formatShortDate(_scheduledStartAt!)} '
                              '${_scheduledStartAt!.hour.toString().padLeft(2, '0')}:'
                              '${_scheduledStartAt!.minute.toString().padLeft(2, '0')}',
                  ),
                ),
              ],
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
                    : const Text('Relist & Tayangkan'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
