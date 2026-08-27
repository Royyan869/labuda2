import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:intl/intl.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/shared/widgets/app_text_field.dart';

/// Section untuk limits & status discount
class LimitsSection extends StatefulWidget {
  final double? minPurchase;
  final int? maxUsagePerUser;
  final int? totalUsageLimit;
  final bool isActive;
  final ValueChanged<double?> onMinPurchaseChanged;
  final ValueChanged<int?> onMaxUsagePerUserChanged;
  final ValueChanged<int?> onTotalUsageLimitChanged;
  final ValueChanged<bool> onIsActiveChanged;

  const LimitsSection({
    super.key,
    this.minPurchase,
    this.maxUsagePerUser,
    this.totalUsageLimit,
    required this.isActive,
    required this.onMinPurchaseChanged,
    required this.onMaxUsagePerUserChanged,
    required this.onTotalUsageLimitChanged,
    required this.onIsActiveChanged,
  });

  @override
  State<LimitsSection> createState() => _LimitsSectionState();
}

class _LimitsSectionState extends State<LimitsSection> {
  late TextEditingController _minPurchaseController;
  late TextEditingController _maxUsagePerUserController;
  late TextEditingController _totalUsageLimitController;

  final _currencyFormatter = NumberFormat.currency(
    locale: 'id_ID',
    symbol: '',
    decimalDigits: 0,
  );

  @override
  void initState() {
    super.initState();
    _minPurchaseController = TextEditingController(
      text: widget.minPurchase != null
          ? _currencyFormatter.format(widget.minPurchase!)
          : '',
    );
    _maxUsagePerUserController = TextEditingController(
      text: widget.maxUsagePerUser?.toString() ?? '',
    );
    _totalUsageLimitController = TextEditingController(
      text: widget.totalUsageLimit?.toString() ?? '',
    );
  }

  @override
  void dispose() {
    _minPurchaseController.dispose();
    _maxUsagePerUserController.dispose();
    _totalUsageLimitController.dispose();
    super.dispose();
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
            'Limits & Status',
            style: TextStyle(
              fontSize: 16,
              fontWeight: FontWeight.bold,
              color: isDark
                  ? core.AppColors.neutralWhite
                  : core.AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 16),

          // Min Purchase
          AppTextField(
            controller: _minPurchaseController,
            labelText: 'Minimum Purchase (Optional)',
            hintText: 'Example: 500000',
            prefixIcon: Icons.shopping_cart,
            keyboardType: TextInputType.number,
            inputFormatters: [FilteringTextInputFormatter.digitsOnly],
            onChanged: (value) {
              final cleanValue = value.replaceAll(RegExp(r'[^\d]'), '');
              final numValue = cleanValue.isEmpty
                  ? null
                  : double.tryParse(cleanValue);
              widget.onMinPurchaseChanged(numValue);

              // Format display
              if (numValue != null) {
                final formatted = _currencyFormatter.format(numValue);
                if (formatted != value) {
                  _minPurchaseController.value = TextEditingValue(
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
            'Discount only applies if purchase subtotal reaches this value',
            style: TextStyle(
              fontSize: 12,
              color: isDark
                  ? core.AppColors.neutralGray400
                  : core.AppColors.neutralGray600,
            ),
          ),
          const SizedBox(height: 16),

          // Max Usage Per User
          AppTextField(
            controller: _maxUsagePerUserController,
            labelText: 'Max Usage Per User (Optional)',
            hintText: 'Example: 3',
            prefixIcon: Icons.person,
            keyboardType: TextInputType.number,
            inputFormatters: [FilteringTextInputFormatter.digitsOnly],
            onChanged: (value) {
              final numValue = value.isEmpty ? null : int.tryParse(value);
              widget.onMaxUsagePerUserChanged(numValue);
            },
          ),
          const SizedBox(height: 8),
          Text(
            'Limit how many times one user can use this code',
            style: TextStyle(
              fontSize: 12,
              color: isDark
                  ? core.AppColors.neutralGray400
                  : core.AppColors.neutralGray600,
            ),
          ),
          const SizedBox(height: 16),

          // Total Usage Limit
          AppTextField(
            controller: _totalUsageLimitController,
            labelText: 'Total Usage Limit (Optional)',
            hintText: 'Example: 100',
            prefixIcon: Icons.groups,
            keyboardType: TextInputType.number,
            inputFormatters: [FilteringTextInputFormatter.digitsOnly],
            onChanged: (value) {
              final numValue = value.isEmpty ? null : int.tryParse(value);
              widget.onTotalUsageLimitChanged(numValue);
            },
          ),
          const SizedBox(height: 8),
          Text(
            'Limit total usage of this code by all users',
            style: TextStyle(
              fontSize: 12,
              color: isDark
                  ? core.AppColors.neutralGray400
                  : core.AppColors.neutralGray600,
            ),
          ),
          const SizedBox(height: 24),

          // Active Status
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: isDark
                  ? core.AppColors.darkGray700
                  : core.AppColors.neutralGray50,
              borderRadius: BorderRadius.circular(8),
            ),
            child: Row(
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Active Status',
                        style: TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w500,
                          color: isDark
                              ? core.AppColors.neutralWhite
                              : core.AppColors.neutralGray900,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        widget.isActive
                            ? 'Discount can be used by buyers'
                            : 'Discount is inactive and cannot be used',
                        style: TextStyle(
                          fontSize: 12,
                          color: isDark
                              ? core.AppColors.neutralGray400
                              : core.AppColors.neutralGray600,
                        ),
                      ),
                    ],
                  ),
                ),
                Switch(
                  value: widget.isActive,
                  onChanged: widget.onIsActiveChanged,
                  activeThumbColor: core.AppColors.neutralWhite,
                  activeTrackColor: core.AppColors.primaryRed,
                  inactiveThumbColor: core.AppColors.neutralGray400,
                  inactiveTrackColor: core.AppColors.neutralGray300,
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
