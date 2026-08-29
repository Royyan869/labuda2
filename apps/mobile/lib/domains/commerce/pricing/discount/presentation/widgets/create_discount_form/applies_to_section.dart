import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';

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
            'Diskon Berlaku Untuk',
            style: TextStyle(
              fontSize: 16,
              fontWeight: FontWeight.bold,
              color: isDark
                  ? core.AppColors.neutralWhite
                  : core.AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 12),

          DropdownButtonFormField<DiscountAppliesTo>(
            initialValue: appliesTo,
            decoration: const InputDecoration(
              labelText: 'Tipe Penjualan',
              border: OutlineInputBorder(),
            ),
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
            style: TextStyle(
              fontSize: 12,
              color: isDark
                  ? core.AppColors.neutralGray400
                  : core.AppColors.neutralGray600,
            ),
          ),
        ],
      ),
    );
  }
}
