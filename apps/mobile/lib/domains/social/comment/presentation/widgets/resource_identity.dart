library;

import 'package:equatable/equatable.dart';

/// Canonical typed identity for a commerce or content resource.
///
/// This is identity only. It does not carry preview or snapshot data.
enum ResourceType {
  content('content'),
  forSale('for_sale'),
  auction('auction'),
  profile('profile');

  const ResourceType(this.wireValue);

  final String wireValue;

  static ResourceType fromWire(String value) {
    return ResourceType.values.firstWhere(
      (type) => type.wireValue == value,
      orElse: () => throw FormatException('Invalid resource type: $value'),
    );
  }
}

class ResourceIdentity extends Equatable {
  final ResourceType resourceType;
  final String resourceId;

  const ResourceIdentity({
    required this.resourceType,
    required this.resourceId,
  });

  Map<String, dynamic> toJson() => {
    'resource_type': resourceType.wireValue,
    'resource_id': resourceId,
  };

  @override
  List<Object?> get props => [resourceType, resourceId];
}
