library;

/// Order Presentation Widgets
///
/// Compatibility barrel for the split order widget library.
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/core/common/types/payment_types.dart';
import 'package:labuda/domains/commerce/transaction/order/order.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/utils/commerce_seller_identity.dart';
import 'package:labuda/domains/chat/chat/chat.dart';
import 'package:labuda/domains/system/support/presentation/widgets/pre_chat_form_sheet.dart';
part 'order_status_timeline.dart';
part 'order_info_card.dart';
part 'order_user_info_card.dart';
part 'order_items_card.dart';
part 'order_shipping_info_card.dart';
part 'order_payment_info_card.dart';
part 'order_pricing_cards.dart';
part 'order_refund_status_card.dart';
part 'order_overdue_cards.dart';

/// Canonical Order → commerce chat entry point.
///
/// Resolves (or creates) the canonical direct commerce room between the two
/// participants of [order], links the order to that room (LATEST ACTIVE ORDER
/// RULE), then navigates to the canonical conversation route `/chat/<room-id>`.
///
/// Every Order surface that opens a chat MUST go through this helper so the
/// room-resolution authority for an order stays singular.
Future<void> openOrderCommerceChat({
  required BuildContext context,
  required WidgetRef ref,
  required Order order,
  required String currentUserId,
}) async {
  final String otherUserId;
  if (currentUserId == order.buyerId) {
    otherUserId = order.sellerId;
  } else if (currentUserId == order.sellerId) {
    otherUserId = order.buyerId;
  } else {
    AppSnackBar.showWarning(context, 'Anda tidak terlibat dalam pesanan ini');
    return;
  }

  try {
    final roomResult = await ref
        .read(getOrCreateCommerceChatUseCaseProvider)
        .call(
          currentUserId: currentUserId,
          otherUserId: otherUserId,
          orderId: order.id,
        );

    if (roomResult.isError) {
      if (context.mounted) {
        final error = roomResult.error ?? '';
        AppSnackBar.showError(
          context,
          error.toLowerCase().contains('blocked')
              ? 'Tidak dapat mengirim pesan. Pengguna ini telah memblokir Anda.'
              : 'Gagal membuka chat. Coba lagi.',
        );
      }
      return;
    }

    if (context.mounted) {
      context.go('/chat/${roomResult.data!.id}');
    }
  } catch (_) {
    if (context.mounted) {
      AppSnackBar.showError(context, 'Gagal membuka chat. Coba lagi.');
    }
  }
}
