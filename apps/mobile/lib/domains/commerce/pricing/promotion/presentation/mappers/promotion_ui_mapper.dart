library;

import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/instance_status.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/promotion_instance.dart';

class PromotionUiMapper {
  const PromotionUiMapper._();

  static String mapStatus(InstanceStatus status) {
    switch (status) {
      case InstanceStatus.active:
        return 'Active';
      case InstanceStatus.paused:
        return 'Paused';
      case InstanceStatus.cancelled:
        return 'Completed';
      case InstanceStatus.expired:
        return 'Expired';
      case InstanceStatus.inactive:
        return 'Inactive';
    }
  }

  static String mapReason(String? reason) {
    switch (reason) {
      case 'seller_inactive':
        return 'Subscription expired';
      case 'seller_verification_suspended':
        return 'Verification suspended';
      case 'fixed_price_sale_hidden':
        return 'Fixed-price sale hidden';
      case 'fixed_price_sale_moderated':
        return 'Fixed-price sale under moderation';
      case 'fixed_price_sale_sold':
        return 'Fixed-price sale sold';
      case 'fixed_price_sale_deleted':
        return 'Fixed-price sale deleted';
      case 'fixed_price_sale_expired':
        return 'Fixed-price sale expired';
      case 'auction_ended':
        return 'Auction ended';
      case 'duration_exhausted':
        return 'Duration exhausted';
      case 'validity_expired':
        return 'Promotion expired';
      default:
        return 'Unknown';
    }
  }

  static String mapTargetName(PromotionInstance instance) {
    final targetId = instance.targetId;
    if (targetId == null || targetId.isEmpty) {
      return 'Unknown target';
    }
    final short = targetId.length > 8 ? targetId.substring(0, 8) : targetId;
    final type = instance.targetType.value;
    return '$type:$short';
  }
}
