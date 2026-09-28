import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';

/// Helper for search result type UI elements
class SearchResultTypeHelper {
  /// Get icon for result type
  static IconData getIcon(SearchResultType type) {
    switch (type) {
      case SearchResultType.user:
        return Icons.person;
      case SearchResultType.forSale:
      case SearchResultType.externalProduct:
        return Icons.shopping_bag;
      case SearchResultType.auction:
        return Icons.gavel;
      case SearchResultType.content:
        return Icons.article;
    }
  }

  /// Get color for result type.
  ///
  /// Brand/semantic tokens stay constant across modes; the content tint
  /// follows the scheme secondary text role so it adapts to light/dark.
  static Color getColor(SearchResultType type, BuildContext context) {
    switch (type) {
      case SearchResultType.user:
        return Theme.of(context).colorScheme.secondary;
      case SearchResultType.forSale:
      case SearchResultType.externalProduct:
        return AppColors.primary;
      case SearchResultType.auction:
        return AppColors.statusWarning;
      case SearchResultType.content:
        return Theme.of(context).colorScheme.onSurfaceVariant;
    }
  }

  /// Get label for result type
  static String getLabel(SearchResultType type) {
    switch (type) {
      case SearchResultType.user:
        return 'User';
      case SearchResultType.forSale:
      case SearchResultType.externalProduct:
        return 'For Sale';
      case SearchResultType.auction:
        return 'Lelang';
      case SearchResultType.content:
        return 'Content';
    }
  }

  /// Get tab index for result type
  static int getTabIndex(SearchResultType? type) {
    switch (type) {
      case SearchResultType.forSale:
      case SearchResultType.externalProduct:
        return 1;
      case SearchResultType.auction:
        return 2;
      case SearchResultType.user:
        return 3;
      case SearchResultType.content:
        return 4;
      default:
        return 0;
    }
  }

  /// Get result type from tab index
  static SearchResultType? getTypeFromIndex(int index) {
    switch (index) {
      case 1:
        return SearchResultType.forSale;
      case 2:
        return SearchResultType.auction;
      case 3:
        return SearchResultType.user;
      case 4:
        return SearchResultType.content;
      default:
        return null;
    }
  }
}
