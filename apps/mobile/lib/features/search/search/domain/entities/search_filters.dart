import 'package:equatable/equatable.dart';

/// Sort options for search results
enum SearchSortBy { relevance, newest, oldest, priceAsc, priceDesc, popularity }

/// Filters that can be applied to search queries
class SearchFilters extends Equatable {
  // Price filters (for collections/auctions)
  final double? minPrice;
  final double? maxPrice;

  // Location filters
  final String? cityName;
  final String? provinceName;

  // Koi-specific filters (for collections/auctions)
  final String? variety;
  final double? minSize;
  final double? maxSize;

  // User filters
  final bool? verifiedOnly;

  // Status filters
  final bool activeOnly;
  final bool forSaleOnly;

  // Date range
  final DateTime? createdAfter;
  final DateTime? createdBefore;

  const SearchFilters({
    this.minPrice,
    this.maxPrice,
    this.cityName,
    this.provinceName,
    this.variety,
    this.minSize,
    this.maxSize,
    this.verifiedOnly,
    this.activeOnly = true,
    this.forSaleOnly = false,
    this.createdAfter,
    this.createdBefore,
  });

  @override
  List<Object?> get props => [
    minPrice,
    maxPrice,
    cityName,
    provinceName,
    variety,
    minSize,
    maxSize,
    verifiedOnly,
    activeOnly,
    forSaleOnly,
    createdAfter,
    createdBefore,
  ];

  /// Check if any filters are active
  bool get hasFilters =>
      minPrice != null ||
      maxPrice != null ||
      cityName != null ||
      provinceName != null ||
      variety != null ||
      minSize != null ||
      maxSize != null ||
      verifiedOnly == true ||
      !activeOnly ||
      forSaleOnly ||
      createdAfter != null ||
      createdBefore != null;

  /// Get count of active filters
  int get activeFilterCount {
    int count = 0;
    if (minPrice != null || maxPrice != null) count++;
    if (cityName != null || provinceName != null) count++;
    if (variety != null) count++;
    if (minSize != null || maxSize != null) count++;
    if (verifiedOnly == true) count++;
    if (forSaleOnly) count++;
    if (createdAfter != null || createdBefore != null) count++;
    return count;
  }

  SearchFilters copyWith({
    double? minPrice,
    double? maxPrice,
    String? cityName,
    String? provinceName,
    String? variety,
    double? minSize,
    double? maxSize,
    bool? verifiedOnly,
    bool? activeOnly,
    bool? forSaleOnly,
    DateTime? createdAfter,
    DateTime? createdBefore,
  }) {
    return SearchFilters(
      minPrice: minPrice ?? this.minPrice,
      maxPrice: maxPrice ?? this.maxPrice,
      cityName: cityName ?? this.cityName,
      provinceName: provinceName ?? this.provinceName,
      variety: variety ?? this.variety,
      minSize: minSize ?? this.minSize,
      maxSize: maxSize ?? this.maxSize,
      verifiedOnly: verifiedOnly ?? this.verifiedOnly,
      activeOnly: activeOnly ?? this.activeOnly,
      forSaleOnly: forSaleOnly ?? this.forSaleOnly,
      createdAfter: createdAfter ?? this.createdAfter,
      createdBefore: createdBefore ?? this.createdBefore,
    );
  }

  /// Create empty filters
  static const SearchFilters empty = SearchFilters();

  /// Clear all filters
  SearchFilters clear() => const SearchFilters();
}
