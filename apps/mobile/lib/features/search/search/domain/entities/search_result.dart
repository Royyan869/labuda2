import 'package:equatable/equatable.dart';

/// Type of searchable content in the platform
enum SearchResultType { user, forSale, externalProduct, auction, content }

/// Polymorphic search result that can represent any searchable entity.
///
/// SECTION-BASED ALL (canonical): each search domain keeps its own
/// backend-ordered result list. There is deliberately NO cross-domain
/// relevance score — no producer ever filled one and no consumer may
/// rank domains against each other — so [SearchResult] carries no
/// relevance field.
class SearchResult extends Equatable {
  final String id;
  final SearchResultType type;
  final String title;
  final String? subtitle;
  final String? imageUrl;
  final String? description;
  final Map<String, dynamic> metadata;
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
      createdAt: createdAt ?? this.createdAt,
      isPromoted: isPromoted ?? this.isPromoted,
      contractId: contractId ?? this.contractId,
    );
  }
}

/// Results of one canonical search execution, held as separate domain
/// collections.
///
/// The All tab is a SECTION-BASED multi-domain overview projected from
/// these collections — it never flattens them into one cross-domain list
/// and never applies a unified ranking:
///
/// ```text
/// SEARCH RESULTS
/// └── ALL
///     ├── Users    → max 3 preview (canonical User ordering)
///     ├── For Sale → max 5 preview (canonical For Sale ordering)
///     ├── Auctions → max 5 preview (canonical Auction ordering)
///     └── Content  → max 5 preview (canonical Content ordering)
/// ```
///
/// Each per-type tab reads the same domain collection; the All preview
/// caps are a rendering projection only and never truncate the collection
/// that backs a per-type tab.
class UnifiedSearchResults extends Equatable {
  final List<SearchResult> users;
  final List<SearchResult> forSales;
  final List<SearchResult> auctions;
  final List<SearchResult> contents;
  final int totalCount;
  final String query;
  final Duration searchDuration;

  const UnifiedSearchResults({
    this.users = const [],
    this.forSales = const [],
    this.auctions = const [],
    this.contents = const [],
    required this.totalCount,
    required this.query,
    this.searchDuration = Duration.zero,
  });

  @override
  List<Object?> get props => [
    users,
    forSales,
    auctions,
    contents,
    totalCount,
    query,
    searchDuration,
  ];

  /// Check if there are no results in any domain
  bool get isEmpty =>
      users.isEmpty &&
      forSales.isEmpty &&
      auctions.isEmpty &&
      contents.isEmpty;

  /// Check if any domain has results
  bool get isNotEmpty => !isEmpty;
}
