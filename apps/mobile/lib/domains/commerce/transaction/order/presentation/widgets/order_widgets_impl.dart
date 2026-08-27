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
import 'package:labuda/domains/chat/chat/data/chat_providers.dart';
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
