/// Object Reference
///
/// Represents a canonical reference to any domain object.
/// Used by the object resolver to fetch live preview data.
library;

import 'package:equatable/equatable.dart';

/// Canonical reference to a domain object
class ObjectReference extends Equatable {
  /// Type of the object (listing, auction, content, profile)
  final String type;

  /// Unique identifier of the object
  final String id;

  const ObjectReference({required this.type, required this.id});

  /// Create a reference to an auction
  const ObjectReference.auction(this.id) : type = 'auction';

  /// Create a reference to content
  const ObjectReference.content(this.id) : type = 'content';

  /// Create a reference to a profile
  const ObjectReference.profile(this.id) : type = 'profile';

  @override
  List<Object?> get props => [type, id];

  @override
  String toString() => 'ObjectReference(type: $type, id: $id)';
}

/// Stable cache key for a list of ObjectReferences
///
/// This class provides a stable, hashable key for batch providers.
/// It sorts and deduplicates references to ensure the same list
/// always produces the same cache key.
class ObjectReferenceListKey extends Equatable {
  /// Unique, sorted list of references (no duplicates)
  final List<ObjectReference> refs;

  const ObjectReferenceListKey(this.refs);

  /// Create from a raw list of references
  ///
  /// Automatically sorts and deduplicates to ensure stability
  factory ObjectReferenceListKey.from(List<ObjectReference> references) {
    // Remove duplicates by using a Set with custom comparison
    final seen = <String>{};
    final uniqueRefs = <ObjectReference>[];

    for (final ref in references) {
      final key = '${ref.type}:${ref.id}';
      if (!seen.contains(key)) {
        seen.add(key);
        uniqueRefs.add(ref);
      }
    }

    // Sort by type then by ID for consistent ordering
    uniqueRefs.sort((a, b) {
      final typeCompare = a.type.compareTo(b.type);
      if (typeCompare != 0) return typeCompare;
      return a.id.compareTo(b.id);
    });

    return ObjectReferenceListKey(uniqueRefs);
  }

  /// Get the cache key string
  String get cacheKey => refs.map((r) => '${r.type}:${r.id}').join(',');

  @override
  List<Object?> get props => [cacheKey];

  @override
  String toString() =>
      'ObjectReferenceListKey(count: ${refs.length}, key: $cacheKey)';
}
