import 'package:equatable/equatable.dart';

class CommerceShippingSetupSummary extends Equatable {
  final String id;
  final String name;
  final String? transportType;

  const CommerceShippingSetupSummary({
    required this.id,
    required this.name,
    this.transportType,
  });

  factory CommerceShippingSetupSummary.fromJson(Map<String, dynamic> json) {
    final transportType = json['transport_type'];
    return CommerceShippingSetupSummary(
      id: json['id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      transportType: transportType is String && transportType.isNotEmpty
          ? transportType
          : null,
    );
  }

  @override
  List<Object?> get props => [id, name, transportType];
}
