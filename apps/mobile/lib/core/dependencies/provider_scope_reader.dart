import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Canonical service-locator authority for Labuda mobile core.
///
/// Current filesystem exports the canonical service locator via:
/// `package:labuda/core/core.dart` → `src/services/service_locator.dart`.
///
/// This file deliberately imports the canonical barrel, not the obsolete
/// `core/dependencies/service_locator.dart` path.
import 'package:labuda/core/core.dart';

/// Minimal static accessor for the Riverpod container currently in scope.
///
/// Helper hanya dipakai oleh kode test/integration yang harus membaca provider
/// tanpa [ConsumerWidget]/[ConsumerStatefulWidget]Warstash. Untuk kode produk
/// yang berjalan di dalam [ConsumerState] sudah cukup memanggil
/// `ref.read(s3ServiceProvider)`.
class _ProviderScopeHolder extends InheritedWidget {
  final ProviderContainer container;

  const _ProviderScopeHolder({required this.container, required super.child});

  @override
  bool updateShouldNotify(covariant InheritedWidget oldWidget) => false;
}
