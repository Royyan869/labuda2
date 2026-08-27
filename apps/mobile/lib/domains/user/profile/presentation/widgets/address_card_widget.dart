import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';

class AddressCardWidget extends StatelessWidget {
  final AddressEntity address;
  final VoidCallback onEdit;
  final VoidCallback onDelete;
  final VoidCallback onSetPrimary;
  final bool isDark;
  final bool canDelete; // Based on min addresses requirement

  const AddressCardWidget({
    super.key,
    required this.address,
    required this.onEdit,
    required this.onDelete,
    required this.onSetPrimary,
    required this.isDark,
    required this.canDelete,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: 16),
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(
          color: address.isPrimary
              ? AppColors.primaryRed.withValues(alpha: 0.3)
              : (isDark ? AppColors.darkGray600 : AppColors.neutralGray200),
        ),
        gradient: address.isPrimary
            ? LinearGradient(
                colors: [
                  AppColors.primaryRed.withValues(alpha: 0.05),
                  AppColors.primaryRed.withValues(alpha: 0.02),
                ],
              )
            : null,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header dengan label, primary badge, dan more options
          Row(
            children: [
              // Address icon
              Container(
                padding: const EdgeInsets.all(8),
                decoration: BoxDecoration(
                  color: AppColors.primaryRed.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Icon(
                  _getIconForPurpose(address.purpose),
                  color: AppColors.primaryRed,
                  size: 20,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      address.displayLabel,
                      style: TextStyle(
                        color: isDark
                            ? AppColors.neutralGray200
                            : AppColors.neutralGray900,
                        fontSize: 16,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    if (address.isPrimary) ...[
                      const SizedBox(height: 2),
                      Row(
                        children: [
                          Icon(
                            Icons.check_circle,
                            size: 12,
                            color: AppColors.primaryRed,
                          ),
                          const SizedBox(width: 4),
                          Text(
                            'Primary Address',
                            style: TextStyle(
                              color: AppColors.primaryRed,
                              fontSize: 11,
                              fontWeight: FontWeight.w500,
                            ),
                          ),
                        ],
                      ),
                    ],
                  ],
                ),
              ),
              // More options menu
              PopupMenuButton<String>(
                icon: Icon(
                  Icons.more_vert,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
                color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
                onSelected: (value) {
                  if (value == 'edit') {
                    onEdit();
                  } else if (value == 'set_primary') {
                    onSetPrimary();
                  } else if (value == 'delete') {
                    onDelete();
                  }
                },
                itemBuilder: (context) => [
                  // Edit option (always available)
                  PopupMenuItem(
                    value: 'edit',
                    child: Row(
                      children: [
                        Icon(
                          Icons.edit_outlined,
                          size: 20,
                          color: isDark
                              ? AppColors.neutralGray300
                              : AppColors.neutralGray700,
                        ),
                        const SizedBox(width: 12),
                        Text(
                          'Edit',
                          style: TextStyle(
                            color: isDark
                                ? AppColors.neutralGray200
                                : AppColors.neutralGray900,
                          ),
                        ),
                      ],
                    ),
                  ),
                  // Set as Primary (only for non-primary addresses)
                  if (!address.isPrimary)
                    PopupMenuItem(
                      value: 'set_primary',
                      child: Row(
                        children: [
                          Icon(
                            Icons.check_circle_outline,
                            size: 20,
                            color: isDark
                                ? AppColors.neutralGray300
                                : AppColors.neutralGray700,
                          ),
                          const SizedBox(width: 12),
                          Text(
                            'Set as Primary',
                            style: TextStyle(
                              color: isDark
                                  ? AppColors.neutralGray200
                                  : AppColors.neutralGray900,
                            ),
                          ),
                        ],
                      ),
                    ),
                  // Delete (only for non-primary addresses and if canDelete)
                  if (!address.isPrimary && canDelete)
                    PopupMenuItem(
                      value: 'delete',
                      child: Row(
                        children: [
                          Icon(
                            Icons.delete_outline,
                            size: 20,
                            color: AppColors.statusError,
                          ),
                          const SizedBox(width: 12),
                          Text(
                            'Delete',
                            style: TextStyle(color: AppColors.statusError),
                          ),
                        ],
                      ),
                    ),
                ],
              ),
            ],
          ),
          const SizedBox(height: 16),

          // Address details
          _buildAddressDetails(),

          if (address.notes != null && address.notes!.isNotEmpty) ...[
            const SizedBox(height: 12),
            _buildNotes(),
          ],
        ],
      ),
    );
  }

  IconData _getIconForPurpose(AddressPurpose purpose) {
    switch (purpose) {
      case AddressPurpose.shipping:
        return Icons.home; // Shipping destination
      case AddressPurpose.sender:
        return Icons.agriculture; // Sender origin (farm/warehouse)
    }
  }

  Widget _buildAddressDetails() {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: isDark
            ? AppColors.darkGray800.withValues(alpha: 0.5)
            : AppColors.neutralGray50,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            address.streetAddress,
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray200
                  : AppColors.neutralGray900,
              fontSize: 14,
              fontWeight: FontWeight.w500,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            '${address.village.name}, ${address.district.name}',
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
              fontSize: 13,
            ),
          ),
          const SizedBox(height: 2),
          Text(
            '${address.city.name}, ${address.province.name} ${address.postalCode}',
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
              fontSize: 13,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildNotes() {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: AppColors.warning.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: AppColors.warning.withValues(alpha: 0.3)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(Icons.info_outline, size: 16, color: AppColors.warning),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              address.notes!,
              style: TextStyle(
                color: isDark
                    ? AppColors.neutralGray300
                    : AppColors.neutralGray700,
                fontSize: 12,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
