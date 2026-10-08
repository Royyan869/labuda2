import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';
import 'package:labuda/shared/utils/app_formatters.dart';

/// Widget card untuk display discount info
class DiscountCard extends StatelessWidget {
  final Discount discount;
  final VoidCallback? onTap;
  final VoidCallback? onEdit;
  final ValueChanged<bool>? onToggleActive;
  final VoidCallback? onDelete;

  const DiscountCard({
    super.key,
    required this.discount,
    this.onTap,
    this.onEdit,
    this.onToggleActive,
    this.onDelete,
  });

  @override
  Widget build(BuildContext context) {
    final isExpired = discount.isExpired;
    final isActive = discount.isActive;

    return Card(
      margin: const EdgeInsets.only(bottom: AppMetrics.p12),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppShape.r12),
        child: Padding(
          padding: const EdgeInsets.all(AppMetrics.p16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Header: Code + Status + More Button
              Row(
                children: [
                  Expanded(
                    child: Text(
                      discount.code,
                      style: context.typeRoles.titleCompact.copyWith(
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                  ),
                  _buildStatusBadge(context, isActive, isExpired),
                  const SizedBox(width: 8),
                  _buildMoreButton(context),
                ],
              ),
              const SizedBox(height: 8),

              // Description
              Text(
                discount.description,
                style: context.typeRoles.bodyDense.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: 12),

              // Discount value
              Text(
                _getDiscountValueText(),
                style: context.typeRoles.titleCompact.copyWith(
                  fontWeight: FontWeight.w600,
                  color: context.statusColors.success,
                ),
              ),
              const SizedBox(height: 8),

              // Expiry
              Text(
                'Expires: ${AppFormatters.formatDate(discount.validUntil)}',
                style: context.typeRoles.labelMicro.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
              ),

              // Usage stats
              if (discount.totalUsageLimit != null) ...[
                const SizedBox(height: 8),
                Text(
                  'Used: ${discount.currentUsageCount}/${discount.totalUsageLimit}',
                  style: context.typeRoles.labelMicro.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildStatusBadge(
    BuildContext context,
    bool isActive,
    bool isExpired,
  ) {
    Color color;
    String text;

    if (isExpired) {
      color = Theme.of(context).colorScheme.onSurfaceVariant;
      text = 'Expired';
    } else if (!isActive) {
      color = context.statusColors.warning;
      text = 'Inactive';
    } else {
      color = context.statusColors.success;
      text = 'Active';
    }

    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p12,
        vertical: AppMetrics.p4,
      ),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: color),
      ),
      child: Text(
        text,
        style: context.typeRoles.labelMicro.copyWith(
          color: color,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }

  String _getDiscountValueText() {
    switch (discount.type) {
      case DiscountType.percentage:
        return '${discount.value.toInt()}% OFF';
      case DiscountType.flatAmount:
        return 'Rp ${formatGroupedAmount(discount.value.round())} OFF';
    }
  }

  Widget _buildMoreButton(BuildContext context) {
    final canDelete = discount.currentUsageCount == 0;

    return PopupMenuButton<String>(
      icon: const Icon(Icons.more_vert),
      itemBuilder: (context) => [
        // Edit
        PopupMenuItem(
          value: 'edit',
          child: Row(
            children: [
              Icon(
                Icons.edit_outlined,
                size: AppIconSize.action,
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
              const SizedBox(width: 12),
              const Text('Edit'),
            ],
          ),
        ),

        // Toggle Active/Inactive
        PopupMenuItem(
          value: 'toggle',
          child: Row(
            children: [
              Icon(
                discount.isActive
                    ? Icons.visibility_off_outlined
                    : Icons.visibility_outlined,
                size: AppIconSize.action,
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
              const SizedBox(width: 12),
              Text(discount.isActive ? 'Deactivate' : 'Activate'),
            ],
          ),
        ),

        // Delete (conditional)
        if (canDelete)
          PopupMenuItem(
            value: 'delete',
            child: Row(
              children: [
                Icon(
                  Icons.delete_outline,
                  size: AppIconSize.action,
                  color: context.statusColors.error,
                ),
                const SizedBox(width: 12),
                Text(
                  'Hapus',
                  style: TextStyle(color: context.statusColors.error),
                ),
              ],
            ),
          )
        else
          PopupMenuItem(
            enabled: false,
            child: Row(
              children: [
                Icon(
                  Icons.delete_outline,
                  size: AppIconSize.action,
                  color: Theme.of(context).colorScheme.outline,
                ),
                const SizedBox(width: 12),
                Text(
                  'Hapus',
                  style: TextStyle(
                    color: Theme.of(context).colorScheme.outline,
                  ),
                ),
              ],
            ),
          ),
      ],
      onSelected: (value) {
        switch (value) {
          case 'edit':
            onEdit?.call();
            break;
          case 'toggle':
            onToggleActive?.call(!discount.isActive);
            break;
          case 'delete':
            onDelete?.call();
            break;
        }
      },
    );
  }
}
