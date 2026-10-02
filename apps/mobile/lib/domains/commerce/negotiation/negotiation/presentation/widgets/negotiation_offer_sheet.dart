/// Canonical negotiation offer sheet — shared by the For Sale detail Nego CTA
/// and the chat banner Counter action (single form authority for the
/// "input nominal" step of the negotiation cycle).
///
/// CANONICAL NEGOTIATION ENTRY: the offer/counter nominal is entered HERE. Nothing navigates to chat
/// and no message is sent silently — the offer is posted to the canonical
/// chat-room-scoped negotiation endpoint (`POST /chat/rooms/:id/negotiate`)
/// and this sheet reports the outcome while the detail stays on screen:
///
/// - success → sheet closes; the caller shows the "terkirim" info;
/// - failure → inline error, sheet stays open with the typed input intact.
library;

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

class NegotiationOfferSheet extends StatefulWidget {
  const NegotiationOfferSheet({
    super.key,
    required this.productTitle,
    required this.onSubmit,
  });

  final String productTitle;

  /// `null` → offer accepted (sheet closes, caller shows the success info).
  /// A string → failure copy rendered inline; the sheet stays open.
  final Future<String?> Function(int price) onSubmit;

  /// Opens the sheet. Returns `true` only when the offer was accepted.
  static Future<bool> show({
    required BuildContext context,
    required String productTitle,
    required Future<String?> Function(int price) onSubmit,
  }) async {
    final sent = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (context) => NegotiationOfferSheet(
        productTitle: productTitle,
        onSubmit: onSubmit,
      ),
    );
    return sent ?? false;
  }

  @override
  State<NegotiationOfferSheet> createState() =>
      _NegotiationOfferSheetState();
}

class _NegotiationOfferSheetState
    extends State<NegotiationOfferSheet> {
  final _formKey = GlobalKey<FormState>();
  final _priceController = TextEditingController();
  bool _submitting = false;
  String? _failure;

  @override
  void dispose() {
    _priceController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (_submitting) return;
    if (!(_formKey.currentState?.validate() ?? false)) return;
    final price = (double.tryParse(_priceController.text) ?? 0).toInt();
    setState(() {
      _submitting = true;
      _failure = null;
    });
    final failure = await widget.onSubmit(price);
    if (!mounted) return;
    if (failure == null) {
      Navigator.of(context).pop(true);
    } else {
      setState(() {
        _submitting = false;
        _failure = failure;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: EdgeInsets.only(
        bottom: MediaQuery.of(context).viewInsets.bottom,
      ),
      child: SafeArea(
        top: false,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(
            AppMetrics.p16,
            AppMetrics.p0,
            AppMetrics.p16,
            AppMetrics.p16,
          ),
          child: Form(
            key: _formKey,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Negosiasi Harga',
                  style: theme.textTheme.titleLarge?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
                ),
                const SizedBox(height: 12),
                Text(
                  widget.productTitle,
                  style: const TextStyle(
                    fontSize: AppType.s14,
                    fontWeight: FontWeight.w500,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  'Masukkan harga tawaran Anda',
                  style: TextStyle(
                    fontSize: AppType.s12,
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 16),
                TextFormField(
                  controller: _priceController,
                  keyboardType: TextInputType.number,
                  enabled: !_submitting,
                  autofocus: true,
                  decoration: const InputDecoration(
                    labelText: 'Tawaran harga Anda',
                    prefixText: 'Rp ',
                    border: OutlineInputBorder(),
                  ),
                  validator: (value) {
                    if (value == null || value.isEmpty) {
                      return 'Masukkan harga tawaran';
                    }
                    final price = double.tryParse(value);
                    if (price == null) return 'Harga tidak valid';
                    if (price <= 0) return 'Harga harus lebih dari 0';
                    return null;
                  },
                ),
                if (_failure != null) ...[
                  const SizedBox(height: 8),
                  Text(
                    _failure!,
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.error,
                    ),
                  ),
                ],
                const SizedBox(height: 16),
                Row(
                  children: [
                    Expanded(
                      child: OutlinedButton(
                        onPressed: _submitting
                            ? null
                            : () => Navigator.of(context).pop(false),
                        child: const Text('Batal'),
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: ElevatedButton(
                        onPressed: _submitting ? null : _submit,
                        child: _submitting
                            ? const SizedBox(
                                height: 18,
                                width: 18,
                                child: CircularProgressIndicator(strokeWidth: 2),
                              )
                            : const Text(
                                'Kirim Penawaran',
                                style: TextStyle(fontWeight: FontWeight.bold),
                              ),
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
