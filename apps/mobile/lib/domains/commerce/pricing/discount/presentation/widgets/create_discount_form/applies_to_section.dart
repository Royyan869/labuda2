import 'package:flutter/material.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Section untuk discount applicability
///
/// CANONICAL MODEL: Discount applicability is by SELLING SURFACE TYPE only.
/// No specific item/surface targeting. Seller selects For Sale / Auction / Both.
class AppliesToSection extends StatelessWidget {
  final DiscountAppliesTo appliesTo;
  final ValueChanged<DiscountAppliesTo> onAppliesToChanged;

  const AppliesToSection({
    super.key,
    required this.appliesTo,
    required this.onAppliesToChanged,
  });

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
            'Diskon Berlaku Untuk',
            style: context.typeRoles.titleSection.copyWith(
              fontWeight: FontWeight.bold,
              color: Theme.of(context).colorScheme.onSurface,
            ),
          ),
          const SizedBox(height: 12),

          DropdownButtonFormField<DiscountAppliesTo>(
            initialValue: appliesTo,
            // Border/fill come from `inputDecorationTheme` (AppTheme) — the
            // one form-field authority.
            decoration: const InputDecoration(labelText: 'Tipe Penjualan'),
            items: const [
              DropdownMenuItem(
                value: DiscountAppliesTo.forSale,
                child: Text('For Sale'),
              ),
              DropdownMenuItem(
                value: DiscountAppliesTo.auction,
                child: Text('Auction'),
              ),
              DropdownMenuItem(
                value: DiscountAppliesTo.both,
                child: Text('Both'),
              ),
            ],
            onChanged: (value) {
              if (value != null) {
                onAppliesToChanged(value);
              }
            },
          ),
          const SizedBox(height: 8),
          Text(
            'Diskon berlaku untuk semua item pada tipe penjualan yang dipilih.',
            style: context.typeRoles.bodyDense.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }
}
