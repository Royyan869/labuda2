library;

/// Presentation Layer Barrel File for Support Module
/// Exports all presentation layer components

import 'package:flutter/material.dart';

// Providers
export 'providers/support_providers.dart';

// Screens
export 'screens/help_center_screen.dart';
export 'screens/support_ticket_thread_screen.dart';
export 'screens/support_tickets_list_screen.dart';

// Widgets
export 'widgets/pre_chat_form_sheet.dart';
export 'widgets/support_ticket_card.dart';
export 'widgets/suggested_messages_widget.dart';

// Import for internal use in the alias function below
import 'widgets/pre_chat_form_sheet.dart' show showPreChatFormRefactored;

// Export alias for backward compatibility
export 'widgets/pre_chat_form_sheet.dart' show showPreChatFormRefactored;

// Helper function alias
void showPreChatForm(
  BuildContext context, {
  required String userId,
  required String userName,
  String? userAvatar,
  String? linkedOrderId,
  VoidCallback? onChatCreated,
}) => showPreChatFormRefactored(
  context,
  userId: userId,
  userName: userName,
  userAvatar: userAvatar,
  linkedOrderId: linkedOrderId,
  onChatCreated: onChatCreated,
);
