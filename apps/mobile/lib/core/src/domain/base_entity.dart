import 'package:equatable/equatable.dart';

abstract class BaseEntity extends Equatable {
  final String id;
  final DateTime createdAt;
  final DateTime updatedAt;

  const BaseEntity({
    required this.id,
    required this.createdAt,
    required this.updatedAt,
  });

  @override
  List<Object?> get props => [id, createdAt, updatedAt];

  // Abstract copyWith - each entity implements its own version
  // BaseEntity copyWith({
  //   String? id,
  //   DateTime? createdAt,
  //   DateTime? updatedAt,
  // });
}

abstract class BaseModel {
  const BaseModel();

  Map<String, dynamic> toJson();

  static T fromJson<T extends BaseModel>(
    Map<String, dynamic> json,
    T Function(Map<String, dynamic>) fromJsonFactory,
  ) {
    return fromJsonFactory(json);
  }
}
