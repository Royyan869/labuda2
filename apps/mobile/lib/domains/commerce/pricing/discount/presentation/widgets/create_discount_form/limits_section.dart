import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:hishumi/shared/utils/money_input_formatter.dart';
import 'package:hishumi/shared/widgets/app_text_field.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

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
      // Minimum purchase is money: seed in the canonical grouped form.
      text: widget.minPurchase > 0
          ? MoneyInputFormatter.display(widget.minPurchase.round())
          : '',
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
            'Limits & Status',
            style: context.typeRoles.titleSection.copyWith(
              fontWeight: FontWeight.bold,
              color: Theme.of(context).colorScheme.onSurface,
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
            inputFormatters: const [MoneyInputFormatter()],
            onChanged: (value) {
              // Punctuation-free business value: grouped display in, plain
              // number out (no double detour through the separators).
              final numValue =
                  (MoneyInputFormatter.parseAmount(value) ?? 0).toDouble();
              widget.onMinPurchaseChanged(numValue);
            },
          ),
          const SizedBox(height: 8),
          Text(
            'Pembeli harus membeli minimal sejumlah ini untuk menggunakan kode diskon.',
            style: context.typeRoles.bodyDense.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
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
            style: context.typeRoles.bodyDense.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 24),

          // Active Status
          Container(
            padding: const EdgeInsets.all(AppMetrics.p12),
            decoration: BoxDecoration(
              color: Theme.of(context).colorScheme.surfaceContainer,
              borderRadius: BorderRadius.circular(AppShape.r8),
            ),
            child: Row(
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Active Status',
                        style: context.typeRoles.bodyDense.copyWith(
                          fontWeight: FontWeight.w500,
                          color: Theme.of(context).colorScheme.onSurface,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        widget.isActive
                            ? 'Discount can be used by buyers'
                            : 'Discount is inactive and cannot be used',
                        style: context.typeRoles.bodyDense.copyWith(
                          color: Theme.of(context).colorScheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ),
                ),
                // Selection colour/state comes from `switchTheme` (AppTheme) —
                // the one selection-control authority.
                Switch(
                  value: widget.isActive,
                  onChanged: widget.onIsActiveChanged,
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
