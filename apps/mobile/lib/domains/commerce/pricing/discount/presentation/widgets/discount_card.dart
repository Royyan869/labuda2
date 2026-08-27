import 'package:flutter/material.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';
import 'package:intl/intl.dart';

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
      margin: const EdgeInsets.only(bottom: 12),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Header: Code + Status + More Button
              Row(
                children: [
                  Expanded(
                    child: Text(
                      discount.code,
                      style: const TextStyle(
                        fontSize: 18,
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                  ),
                  _buildStatusBadge(isActive, isExpired),
                  const SizedBox(width: 8),
                  _buildMoreButton(context),
                ],
              ),
              const SizedBox(height: 8),

              // Description
              Text(
                discount.description,
                style: TextStyle(color: Colors.grey[600], fontSize: 14),
              ),
              const SizedBox(height: 12),

              // Discount value
              Text(
                _getDiscountValueText(),
                style: const TextStyle(
                  fontSize: 16,
                  fontWeight: FontWeight.w600,
                  color: Colors.green,
                ),
              ),
              const SizedBox(height: 8),

              // Valid period
              Text(
                'Valid: ${_formatDate(discount.validFrom)} - ${_formatDate(discount.validUntil)}',
                style: TextStyle(fontSize: 12, color: Colors.grey[600]),
              ),

              // Usage stats
              if (discount.totalUsageLimit != null) ...[
                const SizedBox(height: 8),
                Text(
                  'Used: ${discount.currentUsageCount}/${discount.totalUsageLimit}',
                  style: TextStyle(fontSize: 12, color: Colors.grey[600]),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildStatusBadge(bool isActive, bool isExpired) {
    Color color;
    String text;

    if (isExpired) {
      color = Colors.grey;
      text = 'Expired';
    } else if (!isActive) {
      color = Colors.orange;
      text = 'Inactive';
    } else {
      color = Colors.green;
      text = 'Active';
    }

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: color),
      ),
      child: Text(
        text,
        style: TextStyle(
          color: color,
          fontSize: 12,
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
        return 'Rp ${NumberFormat('#,###', 'id_ID').format(discount.value)} OFF';
      case DiscountType.freeShipping:
        return 'FREE SHIPPING';
    }
  }

  String _formatDate(DateTime date) {
    return DateFormat('dd MMM yyyy', 'id_ID').format(date);
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
              Icon(Icons.edit_outlined, size: 20, color: Colors.grey[700]),
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
                size: 20,
                color: Colors.grey[700],
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
                const Icon(Icons.delete_outline, size: 20, color: Colors.red),
                const SizedBox(width: 12),
                const Text('Hapus', style: TextStyle(color: Colors.red)),
              ],
            ),
          )
        else
          PopupMenuItem(
            enabled: false,
            child: Row(
              children: [
                Icon(Icons.delete_outline, size: 20, color: Colors.grey[400]),
                const SizedBox(width: 12),
                Text('Hapus', style: TextStyle(color: Colors.grey[400])),
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
