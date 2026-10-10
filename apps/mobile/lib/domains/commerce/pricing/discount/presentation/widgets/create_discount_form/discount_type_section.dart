import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:hishumi/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';
import 'package:hishumi/shared/utils/money_input_formatter.dart';
import 'package:hishumi/shared/widgets/app_text_field.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

/// Section untuk tipe & nilai discount
///
/// CANONICAL MODEL: Only percentage and flat_amount types.
class DiscountTypeSection extends StatefulWidget {
  final DiscountType type;
  final double value;
  final ValueChanged<DiscountType> onTypeChanged;
  final ValueChanged<double> onValueChanged;

  const DiscountTypeSection({
    super.key,
    required this.type,
    required this.value,
    required this.onTypeChanged,
    required this.onValueChanged,
  });

  @override
  State<DiscountTypeSection> createState() => _DiscountTypeSectionState();
}

class _DiscountTypeSectionState extends State<DiscountTypeSection> {
  late TextEditingController _valueController;

  @override
  void initState() {
    super.initState();
    _valueController = TextEditingController(
      // Flat-amount discounts are money: seed in the canonical grouped form.
      // Percentage values stay plain numbers.
      text: widget.value <= 0
          ? ''
          : widget.type == DiscountType.flatAmount
          ? MoneyInputFormatter.display(widget.value.round())
          : widget.value.toStringAsFixed(0),
    );
  }

  @override
  void dispose() {
    _valueController.dispose();
    super.dispose();
  }

  String _getTypeLabel(DiscountType type) {
    switch (type) {
      case DiscountType.percentage:
        return 'Percentage (%)';
      case DiscountType.flatAmount:
        return 'Price Discount (Rp)';
    }
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Type & Value',
            style: context.typeRoles.titleSection.copyWith(
              fontWeight: FontWeight.bold,
              color: Theme.of(context).colorScheme.onSurface,
            ),
          ),
          const SizedBox(height: 16),

          // Tipe Diskon
          Text(
            'Discount Type *',
            style: context.typeRoles.bodyDense.copyWith(
              fontWeight: FontWeight.w500,
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 8),

          ...DiscountType.values.map((type) {
            final isSelected = widget.type == type;
            return Padding(
              padding: const EdgeInsets.only(bottom: AppMetrics.p8),
              child: InkWell(
                onTap: () => widget.onTypeChanged(type),
                borderRadius: BorderRadius.circular(AppShape.r8),
                child: Container(
                  padding: const EdgeInsets.all(AppMetrics.p12),
                  decoration: BoxDecoration(
                    color: isSelected
                        ? Theme.of(
                            context,
                          ).colorScheme.primary.withValues(alpha: 0.1)
                        : (Theme.of(context).colorScheme.surfaceContainer),
                    borderRadius: BorderRadius.circular(AppShape.r8),
                    border: Border.all(
                      color: isSelected
                          ? Theme.of(context).colorScheme.primary
                          : (Theme.of(context).colorScheme.outlineVariant),
                      width: isSelected ? 2 : 1,
                    ),
                  ),
                  child: Row(
                    children: [
                      Icon(
                        isSelected
                            ? Icons.radio_button_checked
                            : Icons.radio_button_unchecked,
                        color: isSelected
                            ? Theme.of(context).colorScheme.primary
                            : (Theme.of(context).colorScheme.outline),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Text(
                          _getTypeLabel(type),
                          style: context.typeRoles.bodyDense.copyWith(
                            fontWeight: isSelected
                                ? FontWeight.w600
                                : FontWeight.normal,
                            color: isSelected
                                ? Theme.of(context).colorScheme.primary
                                : (Theme.of(context).colorScheme.onSurface),
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            );
          }),

          const SizedBox(height: 16),

          // Nilai Diskon
          AppTextField(
            controller: _valueController,
            labelText: widget.type == DiscountType.percentage
                ? 'Percentage Value (%) *'
                : 'Discount Amount (Rp) *',
            hintText: widget.type == DiscountType.percentage
                ? 'Example: 50'
                : 'Example: 100000',
            prefixIcon: widget.type == DiscountType.percentage
                ? Icons.percent
                : Icons.money_off,
            keyboardType: TextInputType.number,
            // Flat amount is money (canonical grouping mask); percentage is a
            // plain 0–100 number and stays digits-only.
            inputFormatters: widget.type == DiscountType.flatAmount
                ? const [MoneyInputFormatter()]
                : [FilteringTextInputFormatter.digitsOnly],
            onChanged: (value) {
              final numValue = widget.type == DiscountType.flatAmount
                  ? (MoneyInputFormatter.parseAmount(value) ?? 0).toDouble()
                  : (double.tryParse(value) ?? 0);
              widget.onValueChanged(numValue);
            },
            validator: (value) {
              if (value == null || value.trim().isEmpty) {
                return 'Discount value is required';
              }
              final numValue = widget.type == DiscountType.flatAmount
                  ? MoneyInputFormatter.parseAmount(value)?.toDouble()
                  : double.tryParse(value);
              if (numValue == null || numValue <= 0) {
                return 'Value must be greater than 0';
              }
              if (widget.type == DiscountType.percentage && numValue > 100) {
                return 'Maximum percentage 100%';
              }
              return null;
            },
          ),
        ],
      ),
    );
  }
}
