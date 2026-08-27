import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Address Display Section
///
/// Reusable section for displaying selected address with details.
/// Used in checkout and profile settings.
class AddressDisplaySection extends StatelessWidget {
  final String? label;
  final String recipientName;
  final String? phone;
  final String streetAddress;
  final String? villageDistrict;
  final String? city;
  final String? province;
  final String? postalCode;
  final bool isPrimary;
  final bool hasCoordinates;
  final VoidCallback? onSelectAddress;
  final VoidCallback? onEditAddress;

  const AddressDisplaySection({
    super.key,
    this.label,
    required this.recipientName,
    this.phone,
    required this.streetAddress,
    this.villageDistrict,
    this.city,
    this.province,
    this.postalCode,
    this.isPrimary = false,
    this.hasCoordinates = false,
    this.onSelectAddress,
    this.onEditAddress,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? AppColors.neutralGray800 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Label with action button
          if (label != null) ...[
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text(
                  label!,
                  style: AppTypography.h6.copyWith(
                    color: isDark
                        ? AppColors.neutralWhite
                        : AppColors.neutralGray900,
                  ),
                ),
                Row(
                  children: [
                    if (onSelectAddress != null)
                      TextButton(
                        onPressed: onSelectAddress,
                        child: Text(
                          'Select',
                          style: AppTypography.button.copyWith(
                            color: AppColors.primaryRed,
                          ),
                        ),
                      ),
                    if (onEditAddress != null) ...[
                      const SizedBox(width: 8),
                      TextButton(
                        onPressed: onEditAddress,
                        child: const Text('Edit'),
                      ),
                    ],
                  ],
                ),
              ],
            ),
            const SizedBox(height: 16),
          ],

          // Recipient name with badge
          if (isPrimary) ...[
            Row(
              children: [
                Text(
                  recipientName,
                  style: AppTypography.labelEmphasized.copyWith(
                    color: isDark
                        ? AppColors.neutralWhite
                        : AppColors.neutralGray900,
                  ),
                ),
                const SizedBox(width: 8),
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 6,
                    vertical: 2,
                  ),
                  decoration: BoxDecoration(
                    color: AppColors.primaryRed,
                    borderRadius: BorderRadius.circular(4),
                  ),
                  child: const Text(
                    'Primary',
                    style: TextStyle(
                      fontSize: 10,
                      fontWeight: FontWeight.w600,
                      color: Colors.white,
                    ),
                  ),
                ),
              ],
            ),
          ] else
            Text(
              recipientName,
              style: AppTypography.labelEmphasized.copyWith(
                color: isDark
                    ? AppColors.neutralWhite
                    : AppColors.neutralGray900,
              ),
            ),

          const SizedBox(height: 12),

          // Phone
          if (phone != null) ...[
            Row(
              children: [
                Icon(
                  Icons.person_outline,
                  size: 14,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
                const SizedBox(width: 4),
                Expanded(
                  child: Text(
                    phone!,
                    style: AppTypography.bodyMedium.copyWith(
                      color: isDark
                          ? AppColors.neutralGray300
                          : AppColors.neutralGray700,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
          ],

          // Street address
          Row(
            children: [
              Icon(
                Icons.home_outlined,
                size: 14,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
              ),
              const SizedBox(width: 4),
              Expanded(
                child: Text(
                  streetAddress,
                  style: AppTypography.bodyMedium.copyWith(
                    color: isDark
                        ? AppColors.neutralGray300
                        : AppColors.neutralGray700,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),

          // Village, District
          if (villageDistrict != null) ...[
            Row(
              children: [
                Icon(
                  Icons.location_on,
                  size: 14,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
                const SizedBox(width: 4),
                Expanded(
                  child: Text(
                    villageDistrict!,
                    style: AppTypography.bodyMedium.copyWith(
                      color: isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray600,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
          ],

          // City
          if (city != null) ...[
            Row(
              children: [
                Icon(
                  Icons.location_city,
                  size: 14,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
                const SizedBox(width: 4),
                Text(
                  city!,
                  style: AppTypography.bodyMedium.copyWith(
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray600,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
          ],

          // Province
          if (province != null) ...[
            Row(
              children: [
                Icon(
                  Icons.map,
                  size: 14,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
                const SizedBox(width: 4),
                Expanded(
                  child: Text(
                    province!,
                    style: AppTypography.bodyMedium.copyWith(
                      color: isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray600,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
          ],

          // Postal Code
          if (postalCode != null) ...[
            Text(
              postalCode!,
              style: AppTypography.bodyMedium.copyWith(
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
              ),
            ),
            const SizedBox(height: 16),
          ],

          // Coordinates indicator
          if (hasCoordinates) ...[
            Row(
              children: [
                Icon(
                  Icons.location_on,
                  size: 14,
                  color: AppColors.statusSuccess,
                ),
                const SizedBox(width: 4),
                Expanded(
                  child: Text(
                    'Pinpoint location available',
                    style: AppTypography.caption.copyWith(
                      color: AppColors.statusSuccess,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 16),
          ],
        ],
      ),
    );
  }
}
