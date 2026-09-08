import 'package:equatable/equatable.dart';

/// Type of searchable content in the platform
enum SearchResultType { user, listing, externalProduct, auction, content }

/// Polymorphic search result that can represent any searchable entity
class SearchResult extends Equatable {
  final String id;
  final SearchResultType type;
  final String title;
  final String? subtitle;
  final String? imageUrl;
  final String? description;
  final Map<String, dynamic> metadata;
  final double relevanceScore;
  final DateTime createdAt;

  /// PROMOTION PHASE 4: Whether this result is promoted
  /// When true, this item appears in results due to active promotion
  final bool isPromoted;

  /// PROMOTION CANONICAL: contract ID of the promotion (if promoted).
  /// Carries promotion_contracts.id — the canonical identity. The legacy
  /// instance-id vocabulary is purged.
  final String? contractId;

  const SearchResult({
    required this.id,
    required this.type,
    required this.title,
    this.subtitle,
    this.imageUrl,
    this.description,
    this.metadata = const {},
    this.relevanceScore = 0.0,
    required this.createdAt,
    this.isPromoted = false,
    this.contractId,
  });

  @override
  List<Object?> get props => [
    id,
    type,
    title,
    subtitle,
    imageUrl,
    description,
    metadata,
    relevanceScore,
    createdAt,
    isPromoted,
    contractId,
  ];

  SearchResult copyWith({
    String? id,
    SearchResultType? type,
    String? title,
    String? subtitle,
    String? imageUrl,
    String? description,
    Map<String, dynamic>? metadata,
    double? relevanceScore,
    DateTime? createdAt,
    bool? isPromoted,
    String? contractId,
  }) {
    return SearchResult(
      id: id ?? this.id,
      type: type ?? this.type,
      title: title ?? this.title,
      subtitle: subtitle ?? this.subtitle,
      imageUrl: imageUrl ?? this.imageUrl,
      description: description ?? this.description,
      metadata: metadata ?? this.metadata,
      relevanceScore: relevanceScore ?? this.relevanceScore,
      createdAt: createdAt ?? this.createdAt,
      isPromoted: isPromoted ?? this.isPromoted,
      contractId: contractId ?? this.contractId,
    );
  }
}

/// Unified search results containing results from all types
class UnifiedSearchResults extends Equatable {
  final List<SearchResult> allResults;
  final List<SearchResult> users;
  final List<SearchResult> listings;
  final List<SearchResult> auctions;
  final List<SearchResult> contents;
  final int totalCount;
  final String query;
  final Duration searchDuration;

  const UnifiedSearchResults({
    required this.allResults,
    this.users = const [],
    this.listings = const [],
    this.auctions = const [],
    this.contents = const [],
    required this.totalCount,
    required this.query,
    this.searchDuration = Duration.zero,
  });

  @override
  List<Object?> get props => [
    allResults,
    users,
    listings,
    auctions,
    contents,
    totalCount,
    query,
    searchDuration,
  ];

  /// Get results by type
  List<SearchResult> getByType(SearchResultType type) {
    switch (type) {
      case SearchResultType.user:
        return users;
      case SearchResultType.listing:
        return listings;
      case SearchResultType.externalProduct:
        return allResults
            .where((result) => result.type == SearchResultType.externalProduct)
            .toList();
      case SearchResultType.auction:
        return auctions;
      case SearchResultType.content:
        return contents;
    }
  }

  /// Check if there are no results
  bool get isEmpty => allResults.isEmpty;

  /// Check if there are results
  bool get isNotEmpty => allResults.isNotEmpty;
}
