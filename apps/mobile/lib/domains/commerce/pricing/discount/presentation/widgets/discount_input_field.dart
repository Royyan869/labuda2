import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_validation_result.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/use_cases/validate_discount_use_case.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/providers/discount_provider.dart';

/// Widget untuk input kode diskon di checkout.
class DiscountInputField extends ConsumerStatefulWidget {
  final double subtotal;
  final String contextType;
  final String sellerId;
  final String? listingId;
  final String? auctionId;
  final Function(Discount? discount, double discountAmount)? onDiscountApplied;

  const DiscountInputField({
    super.key,
    required this.subtotal,
    required this.contextType,
    required this.sellerId,
    this.listingId,
    this.auctionId,
    this.onDiscountApplied,
  });

  @override
  ConsumerState<DiscountInputField> createState() => _DiscountInputFieldState();
}

class _DiscountInputFieldState extends ConsumerState<DiscountInputField> {
  final TextEditingController _controller = TextEditingController();
  bool _isValidating = false;
  DiscountValidationResult? _validationResult;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _validateDiscount() async {
    final code = _controller.text.trim();
    if (code.isEmpty) {
      setState(() {
        _validationResult = null;
      });
      widget.onDiscountApplied?.call(null, 0);
      return;
    }

    setState(() {
      _isValidating = true;
      _validationResult = null;
    });

    try {
      final useCase = ref.read(validateDiscountUseCaseProvider);
      final params = ValidateDiscountParams(
        code: code,
        subtotal: widget.subtotal,
        contextType: widget.contextType,
        sellerId: widget.sellerId,
        listingId: widget.listingId,
        auctionId: widget.auctionId,
      );

      final result = await useCase(params);

      result.fold(
        (error) {
          setState(() {
            _validationResult = DiscountValidationResult.error(error);
            _isValidating = false;
          });
          widget.onDiscountApplied?.call(null, 0);
        },
        (validation) {
          setState(() {
            _validationResult = validation;
            _isValidating = false;
          });

          if (validation.isValid) {
            widget.onDiscountApplied?.call(validation.discount, 0);
          } else {
            widget.onDiscountApplied?.call(null, 0);
          }
        },
      );
    } catch (e) {
      setState(() {
        _validationResult = DiscountValidationResult.error(
          'Terjadi kesalahan: $e',
        );
        _isValidating = false;
      });
      widget.onDiscountApplied?.call(null, 0);
    }
  }

  void _clearDiscount() {
    _controller.clear();
    setState(() {
      _validationResult = null;
    });
    widget.onDiscountApplied?.call(null, 0);
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(
              child: TextField(
                controller: _controller,
                decoration: InputDecoration(
                  labelText: 'Discount Code (Optional)',
                  hintText: 'Enter code',
                  border: const OutlineInputBorder(),
                  suffixIcon: _controller.text.isNotEmpty
                      ? IconButton(
                          icon: const Icon(Icons.close),
                          onPressed: _clearDiscount,
                        )
                      : null,
                ),
                textCapitalization: TextCapitalization.characters,
                onChanged: (_) => setState(() {}),
              ),
            ),
            const SizedBox(width: 12),
            ElevatedButton(
              onPressed: _isValidating ? null : _validateDiscount,
              style: ElevatedButton.styleFrom(
                padding: const EdgeInsets.symmetric(
                  horizontal: 24,
                  vertical: 16,
                ),
              ),
              child: _isValidating
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Text('Pakai'),
            ),
          ],
        ),
        if (_validationResult != null) ...[
          const SizedBox(height: 12),
          _buildValidationResult(),
        ],
      ],
    );
  }

  Widget _buildValidationResult() {
    final result = _validationResult!;
    if (!result.isValid) {
      return Container(
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: Colors.red.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(8),
          border: Border.all(color: Colors.red),
        ),
        child: Row(
          children: [
            const Icon(Icons.error_outline, color: Colors.red, size: 20),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                result.errorMessage ?? 'Invalid discount code',
                style: const TextStyle(color: Colors.red, fontSize: 14),
              ),
            ),
          ],
        ),
      );
    }

    final discount = result.discount!;
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: Colors.green.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: Colors.green),
      ),
      child: Row(
        children: [
          const Icon(Icons.check_circle, color: Colors.green, size: 20),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              discount.description,
              style: const TextStyle(
                color: Colors.green,
                fontSize: 14,
                fontWeight: FontWeight.w600,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
