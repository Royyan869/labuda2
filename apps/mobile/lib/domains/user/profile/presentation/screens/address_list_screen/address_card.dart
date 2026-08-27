import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';

/// Address card widget displaying address information with actions
class AddressCard extends StatelessWidget {
  final AddressEntity address;
  final bool canDelete;
  final bool isDark;
  final VoidCallback onEdit;
  final VoidCallback? onSetPrimary;
  final VoidCallback? onDelete;
  final VoidCallback? onCoordinateTap;

  const AddressCard({
    super.key,
    required this.address,
    required this.canDelete,
    required this.isDark,
    required this.onEdit,
    this.onSetPrimary,
    this.onDelete,
    this.onCoordinateTap,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: address.isPrimary
              ? AppColors.primaryRed
              : (isDark ? AppColors.darkGray600 : AppColors.neutralGray200),
          width: address.isPrimary ? 2 : 1,
        ),
      ),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _buildHeader(context),
            const SizedBox(height: 12),
            _buildRecipientInfo(),
            const SizedBox(height: 8),
            _buildFullAddress(),
            if (address.notes != null && address.notes!.isNotEmpty) ...[
              const SizedBox(height: 8),
              _buildNotes(),
            ],
            if (address.hasCoordinates) ...[
              const SizedBox(height: 8),
              _buildCoordinateIndicator(),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildHeader(BuildContext context) {
    return Row(
      children: [
        Icon(
          _getPurposeIcon(address.purpose),
          size: 20,
          color: AppColors.primaryRed,
        ),
        const SizedBox(width: 8),
        Text(
          address.displayLabel,
          style: TextStyle(
            fontSize: 16,
            fontWeight: FontWeight.bold,
            color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
          ),
        ),
        if (address.isPrimary) ...[
          const SizedBox(width: 8),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
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
        const Spacer(),
        _buildPopupMenu(context),
      ],
    );
  }

  Widget _buildPopupMenu(BuildContext context) {
    return PopupMenuButton<String>(
      onSelected: (value) {
        switch (value) {
          case 'edit':
            onEdit();
            break;
          case 'setPrimary':
            onSetPrimary?.call();
            break;
          case 'delete':
            onDelete?.call();
            break;
        }
      },
      itemBuilder: (context) => [
        const PopupMenuItem(
          value: 'edit',
          child: Row(
            children: [
              Icon(Icons.edit, size: 18),
              SizedBox(width: 8),
              Text('Edit'),
            ],
          ),
        ),
        if (!address.isPrimary && onSetPrimary != null)
          const PopupMenuItem(
            value: 'setPrimary',
            child: Row(
              children: [
                Icon(Icons.star, size: 18),
                SizedBox(width: 8),
                Text('Set as Primary'),
              ],
            ),
          ),
        if (canDelete && !address.isPrimary && onDelete != null)
          PopupMenuItem(
            value: 'delete',
            child: Row(
              children: [
                Icon(Icons.delete, size: 18, color: AppColors.error),
                const SizedBox(width: 8),
                Text('Delete', style: TextStyle(color: AppColors.error)),
              ],
            ),
          ),
      ],
    );
  }

  Widget _buildRecipientInfo() {
    return Row(
      children: [
        Icon(
          Icons.person_outline,
          size: 14,
          color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray500,
        ),
        const SizedBox(width: 6),
        Expanded(
          child: Text(
            '${address.recipientName} • ${address.phone}',
            style: TextStyle(
              fontSize: 13,
              fontWeight: FontWeight.w500,
              color: isDark
                  ? AppColors.neutralGray200
                  : AppColors.neutralGray800,
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildFullAddress() {
    return Text(
      address.fullAddress,
      style: TextStyle(
        fontSize: 14,
        color: isDark ? AppColors.neutralGray300 : AppColors.neutralGray700,
      ),
    );
  }

  Widget _buildNotes() {
    return Container(
      padding: const EdgeInsets.all(8),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralGray100,
        borderRadius: BorderRadius.circular(6),
      ),
      child: Row(
        children: [
          Icon(Icons.note, size: 14, color: AppColors.neutralGray500),
          const SizedBox(width: 6),
          Expanded(
            child: Text(
              address.notes!,
              style: TextStyle(
                fontSize: 12,
                fontStyle: FontStyle.italic,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildCoordinateIndicator() {
    return InkWell(
      onTap: onCoordinateTap,
      borderRadius: BorderRadius.circular(6),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
        decoration: BoxDecoration(
          color: AppColors.success.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(6),
          border: Border.all(color: AppColors.success.withValues(alpha: 0.3)),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.location_on, size: 14, color: AppColors.success),
            const SizedBox(width: 6),
            Text(
              'Pinpoint Location Saved',
              style: TextStyle(
                fontSize: 12,
                fontWeight: FontWeight.w500,
                color: AppColors.success,
              ),
            ),
            const SizedBox(width: 4),
            Icon(Icons.chevron_right, size: 14, color: AppColors.success),
          ],
        ),
      ),
    );
  }

  IconData _getPurposeIcon(AddressPurpose purpose) {
    switch (purpose) {
      case AddressPurpose.shipping:
        return Icons.home;
      case AddressPurpose.sender:
        return Icons.agriculture;
    }
  }
}
