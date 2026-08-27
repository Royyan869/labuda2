import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:intl/intl.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';
import 'package:labuda/shared/widgets/app_text_field.dart';

/// Section untuk tipe & nilai discount
class DiscountTypeSection extends StatefulWidget {
  final DiscountType type;
  final double value;
  final double? maxDiscount;
  final ValueChanged<DiscountType> onTypeChanged;
  final ValueChanged<double> onValueChanged;
  final ValueChanged<double?> onMaxDiscountChanged;

  const DiscountTypeSection({
    super.key,
    required this.type,
    required this.value,
    this.maxDiscount,
    required this.onTypeChanged,
    required this.onValueChanged,
    required this.onMaxDiscountChanged,
  });

  @override
  State<DiscountTypeSection> createState() => _DiscountTypeSectionState();
}

class _DiscountTypeSectionState extends State<DiscountTypeSection> {
  late TextEditingController _valueController;
  late TextEditingController _maxDiscountController;

  final _currencyFormatter = NumberFormat.currency(
    locale: 'id_ID',
    symbol: '',
    decimalDigits: 0,
  );

  @override
  void initState() {
    super.initState();
    _valueController = TextEditingController(
      text: widget.value > 0 ? widget.value.toStringAsFixed(0) : '',
    );
    _maxDiscountController = TextEditingController(
      text: widget.maxDiscount != null
          ? _currencyFormatter.format(widget.maxDiscount!)
          : '',
    );
  }

  @override
  void dispose() {
    _valueController.dispose();
    _maxDiscountController.dispose();
    super.dispose();
  }

  String _getTypeLabel(DiscountType type) {
    switch (type) {
      case DiscountType.percentage:
        return 'Percentage (%)';
      case DiscountType.flatAmount:
        return 'Price Discount (Rp)';
      case DiscountType.freeShipping:
        return 'Free Shipping';
    }
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark
            ? core.AppColors.darkGray800
            : core.AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Type & Value',
            style: TextStyle(
              fontSize: 16,
              fontWeight: FontWeight.bold,
              color: isDark
                  ? core.AppColors.neutralWhite
                  : core.AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 16),

          // Tipe Diskon
          Text(
            'Discount Type *',
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w500,
              color: isDark
                  ? core.AppColors.neutralGray300
                  : core.AppColors.neutralGray700,
            ),
          ),
          const SizedBox(height: 8),

          ...DiscountType.values.map((type) {
            final isSelected = widget.type == type;
            return Padding(
              padding: const EdgeInsets.only(bottom: 8),
              child: InkWell(
                onTap: () => widget.onTypeChanged(type),
                borderRadius: BorderRadius.circular(8),
                child: Container(
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: isSelected
                        ? core.AppColors.primaryRed.withValues(alpha: 0.1)
                        : (isDark
                              ? core.AppColors.darkGray700
                              : core.AppColors.neutralGray50),
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(
                      color: isSelected
                          ? core.AppColors.primaryRed
                          : (isDark
                                ? core.AppColors.neutralGray700
                                : core.AppColors.neutralGray200),
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
                            ? core.AppColors.primaryRed
                            : (isDark
                                  ? core.AppColors.neutralGray400
                                  : core.AppColors.neutralGray500),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Text(
                          _getTypeLabel(type),
                          style: TextStyle(
                            fontSize: 14,
                            fontWeight: isSelected
                                ? FontWeight.w600
                                : FontWeight.normal,
                            color: isSelected
                                ? core.AppColors.primaryRed
                                : (isDark
                                      ? core.AppColors.neutralWhite
                                      : core.AppColors.neutralGray900),
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
          if (widget.type != DiscountType.freeShipping) ...[
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
              inputFormatters: [FilteringTextInputFormatter.digitsOnly],
              onChanged: (value) {
                final numValue = double.tryParse(value) ?? 0;
                widget.onValueChanged(numValue);
              },
              validator: (value) {
                if (value == null || value.trim().isEmpty) {
                  return 'Discount value is required';
                }
                final numValue = double.tryParse(value);
                if (numValue == null || numValue <= 0) {
                  return 'Value must be greater than 0';
                }
                if (widget.type == DiscountType.percentage && numValue > 100) {
                  return 'Maximum percentage 100%';
                }
                return null;
              },
            ),
            const SizedBox(height: 16),
          ],

          // Max Discount (only for percentage)
          if (widget.type == DiscountType.percentage) ...[
            AppTextField(
              controller: _maxDiscountController,
              labelText: 'Maximum Discount (Optional)',
              hintText: 'Example: 500000',
              prefixIcon: Icons.attach_money,
              keyboardType: TextInputType.number,
              inputFormatters: [FilteringTextInputFormatter.digitsOnly],
              onChanged: (value) {
                final cleanValue = value.replaceAll(RegExp(r'[^\d]'), '');
                final numValue = double.tryParse(cleanValue);
                widget.onMaxDiscountChanged(numValue);

                // Format display
                if (numValue != null) {
                  final formatted = _currencyFormatter.format(numValue);
                  if (formatted != value) {
                    _maxDiscountController.value = TextEditingValue(
                      text: formatted,
                      selection: TextSelection.collapsed(
                        offset: formatted.length,
                      ),
                    );
                  }
                }
              },
            ),
            const SizedBox(height: 8),
            Text(
              'If filled, maximum price discount is this value even if percentage produces larger value',
              style: TextStyle(
                fontSize: 12,
                color: isDark
                    ? core.AppColors.neutralGray400
                    : core.AppColors.neutralGray600,
              ),
            ),
          ],
        ],
      ),
    );
  }
}
