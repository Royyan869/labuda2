import 'package:equatable/equatable.dart';

/// Canonical commerce detail authority for a single viewer.
///
/// This is viewer-scoped, not entity-scoped: the backend detail response can
/// emit a different role/action set depending on the authenticated viewer.
class CommerceViewerCapabilities extends Equatable {
  final String role;
  final bool canManage;
  final bool canEdit;
  final bool canPromote;
  final bool canChat;
  final bool canNegotiate;
  final bool canBuy;
  final bool canBid;
  final bool canBuyNow;

  const CommerceViewerCapabilities({
    required this.role,
    required this.canManage,
    required this.canEdit,
    required this.canPromote,
    required this.canChat,
    required this.canNegotiate,
    required this.canBuy,
    required this.canBid,
    required this.canBuyNow,
  });

  const CommerceViewerCapabilities.guest()
    : role = 'guest',
      canManage = false,
      canEdit = false,
      canPromote = false,
      canChat = false,
      canNegotiate = false,
      canBuy = false,
      canBid = false,
      canBuyNow = false;

  bool get isOwner => role == 'owner';
  bool get isBuyer => role == 'buyer';
  bool get isGuest => role == 'guest';

  factory CommerceViewerCapabilities.fromJson(Map<String, dynamic>? json) {
    if (json == null) {
      return const CommerceViewerCapabilities.guest();
    }
    return CommerceViewerCapabilities(
      role: json['role'] as String? ?? 'guest',
      canManage: json['can_manage'] as bool? ?? false,
      canEdit: json['can_edit'] as bool? ?? false,
      canPromote: json['can_promote'] as bool? ?? false,
      canChat: json['can_chat'] as bool? ?? false,
      canNegotiate: json['can_negotiate'] as bool? ?? false,
      canBuy: json['can_buy'] as bool? ?? false,
      canBid: json['can_bid'] as bool? ?? false,
      canBuyNow: json['can_buy_now'] as bool? ?? false,
    );
  }

  Map<String, dynamic> toJson() => {
    'role': role,
    'can_manage': canManage,
    'can_edit': canEdit,
    'can_promote': canPromote,
    'can_chat': canChat,
    'can_negotiate': canNegotiate,
    'can_buy': canBuy,
    'can_bid': canBid,
    'can_buy_now': canBuyNow,
  };

  @override
  List<Object?> get props => [
    role,
    canManage,
    canEdit,
    canPromote,
    canChat,
    canNegotiate,
    canBuy,
    canBid,
    canBuyNow,
  ];
}
