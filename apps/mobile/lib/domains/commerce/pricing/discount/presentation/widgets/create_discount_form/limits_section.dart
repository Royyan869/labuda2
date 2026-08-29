import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/shared/widgets/app_text_field.dart';

/// Section untuk limits, minimum purchase, & status discount
///
/// CANONICAL MODEL: totalUsageLimit (optional), minPurchase (optional),
/// active status. No maxUsagePerUser.
class LimitsSection extends StatefulWidget {
  final int? totalUsageLimit;
  final double minPurchase;
  final bool isActive;
  final ValueChanged<int?> onTotalUsageLimitChanged;
  final ValueChanged<double> onMinPurchaseChanged;
  final ValueChanged<bool> onIsActiveChanged;

  const LimitsSection({
    super.key,
    this.totalUsageLimit,
    this.minPurchase = 0.0,
    required this.isActive,
    required this.onTotalUsageLimitChanged,
    required this.onMinPurchaseChanged,
    required this.onIsActiveChanged,
  });

  @override
  State<LimitsSection> createState() => _LimitsSectionState();
}

class _LimitsSectionState extends State<LimitsSection> {
  late TextEditingController _totalUsageLimitController;
  late TextEditingController _minPurchaseController;

  @override
  void initState() {
    super.initState();
    _totalUsageLimitController = TextEditingController(
      text: widget.totalUsageLimit?.toString() ?? '',
    );
    _minPurchaseController = TextEditingController(
      text: widget.minPurchase > 0 ? widget.minPurchase.toStringAsFixed(0) : '',
    );
  }

  @override
  void dispose() {
    _totalUsageLimitController.dispose();
    _minPurchaseController.dispose();
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

          // Minimum Purchase Amount
          AppTextField(
            controller: _minPurchaseController,
            labelText: 'Minimum Purchase Amount (Optional)',
            hintText: 'Example: 100000',
            prefixIcon: Icons.shopping_cart,
            keyboardType: TextInputType.number,
            inputFormatters: [FilteringTextInputFormatter.digitsOnly],
            onChanged: (value) {
              final numValue = value.isEmpty ? 0.0 : (double.tryParse(value) ?? 0.0);
              widget.onMinPurchaseChanged(numValue);
            },
          ),
          const SizedBox(height: 8),
          Text(
            'Pembeli harus membeli minimal sejumlah ini untuk menggunakan kode diskon.',
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
            'Limit total usage of this code by all buyers. Leave empty for unlimited.',
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
