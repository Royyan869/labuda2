import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';

/// Empty state widget for address list
class AddressEmptyState extends StatelessWidget {
  final AddressPurpose purpose;
  final bool isDark;

  const AddressEmptyState({
    super.key,
    required this.purpose,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              purpose == AddressPurpose.shipping
                  ? Icons.location_off_outlined
                  : Icons.agriculture_outlined,
              size: 80,
              color: AppColors.neutralGray400,
            ),
            const SizedBox(height: 24),
            Text(
              'No ${purpose.label} Yet',
              style: TextStyle(
                fontSize: 20,
                fontWeight: FontWeight.bold,
                color: isDark
                    ? AppColors.neutralWhite
                    : AppColors.neutralGray900,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              purpose == AddressPurpose.shipping
                  ? 'Add a shipping address to start shopping'
                  : 'Add a sender address (farm/warehouse location)',
              textAlign: TextAlign.center,
              style: TextStyle(
                fontSize: 14,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
