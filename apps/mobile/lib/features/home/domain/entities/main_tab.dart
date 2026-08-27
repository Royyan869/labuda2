import 'package:flutter/material.dart';

/// Domain entity untuk MainTab
/// Bebas dari implementation details (Riverpod, Firebase, dll)
///
/// MainTab merepresentasikan sebuah tab di bottom navigation
class MainTabEntity {
  final String id;
  final String label;
  final String iconName;
  final String selectedIconName;

  const MainTabEntity({
    required this.id,
    required this.label,
    required this.iconName,
    required this.selectedIconName,
  });

  /// Convert IconData ke string name untuk serialisasi
  /// Data akan di-convert ke IconData di presentation layer
  factory MainTabEntity.fromIconData({
    required String id,
    required String label,
    required IconData icon,
    required IconData selectedIcon,
  }) {
    return MainTabEntity(
      id: id,
      label: label,
      iconName: icon.codePoint.toString(),
      selectedIconName: selectedIcon.codePoint.toString(),
    );
  }

  MainTabEntity copyWith({
    String? id,
    String? label,
    String? iconName,
    String? selectedIconName,
  }) {
    return MainTabEntity(
      id: id ?? this.id,
      label: label ?? this.label,
      iconName: iconName ?? this.iconName,
      selectedIconName: selectedIconName ?? this.selectedIconName,
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) || other is MainTabEntity && other.id == id;

  @override
  int get hashCode => id.hashCode;
}
