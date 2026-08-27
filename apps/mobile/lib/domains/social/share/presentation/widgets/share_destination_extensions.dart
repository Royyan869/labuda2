import 'package:flutter/material.dart';
import '../../domain/entities/share_destination.dart';

/// Extension to convert ShareDestination to IconData
extension ShareDestinationIconExtension on ShareDestination {
  IconData get iconData {
    switch (iconName) {
      case 'chat':
        return Icons.chat;
      case 'camera_alt':
        return Icons.camera_alt;
      case 'send':
        return Icons.send;
      case 'link':
        return Icons.link;
      case 'email':
        return Icons.email;
      case 'home':
        return Icons.home;
      case 'message':
        return Icons.message;
      case 'bookmark':
        return Icons.bookmark;
      case 'more_horiz':
        return Icons.more_horiz;
      case 'repeat':
        return Icons.repeat;
      case 'gavel':
        return Icons.gavel;
      case 'article':
        return Icons.article;
      case 'shopping_bag':
        return Icons.shopping_bag;
      case 'help_outline':
        return Icons.help_outline;
      case 'person':
        return Icons.person;
      case 'emoji_events':
        return Icons.emoji_events;
      default:
        return Icons.share;
    }
  }

  /// Get color from hex string
  Color? get color {
    if (colorHex == null) return null;
    try {
      final colorValue = int.parse(colorHex!.replaceFirst('#', '0xFF'));
      return Color(colorValue);
    } catch (_) {
      return null;
    }
  }
}
