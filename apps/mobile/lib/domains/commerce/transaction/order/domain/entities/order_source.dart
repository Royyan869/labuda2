import 'package:flutter/material.dart';

/// Order Source - aligned with backend OrderSourceType
/// Backend values: "for_sale", "negotiation", "auction"
enum OrderSource {
  /// Direct fixed-price sale purchase (backend: "for_sale")
  forSale,

  /// From accepted negotiation (backend: "negotiation")
  negotiation,

  /// From auction winning bid (backend: "auction")
  auction,
}

extension OrderSourceExtension on OrderSource {
  /// Get display label for UI
  String get label {
    switch (this) {
      case OrderSource.forSale:
        return 'For Sale';
      case OrderSource.negotiation:
        return 'Negotiation';
      case OrderSource.auction:
        return 'Auction';
    }
  }

  Color get badgeColor {
    switch (this) {
      case OrderSource.forSale:
        return const Color(0xFF2196F3); // Blue
      case OrderSource.negotiation:
        return const Color(0xFF9C27B0); // Purple
      case OrderSource.auction:
        return const Color(0xFFF44336); // Red
    }
  }

  /// Convert to backend JSON value
  /// IMPORTANT: Backend uses "for_sale"
  String toJson() {
    switch (this) {
      case OrderSource.forSale:
        return 'for_sale';
      case OrderSource.negotiation:
        return 'negotiation';
      case OrderSource.auction:
        return 'auction';
    }
  }

  /// Parse from backend JSON value
  static OrderSource fromJson(String value) {
    switch (value.toLowerCase()) {
      case 'for_sale':
        return OrderSource.forSale;
      case 'negotiation':
      case 'negotiationoffer':
        return OrderSource.negotiation;
      case 'auction':
        return OrderSource.auction;
      default:
        // Default to fixed-price sale for unknown values.
        return OrderSource.forSale;
    }
  }
}
