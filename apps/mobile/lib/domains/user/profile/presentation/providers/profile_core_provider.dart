// =============================================================================
// ARCHITECTURE GUARDRAIL (R5) - FEATURE PROVIDER PATTERN EXAMPLE
// =============================================================================
//
// **CANONICAL PATTERN for Feature Providers:**
// 1. Import data layer providers via show (hide internal details)
// 2. Import core services from core/providers/core_providers.dart
// 3. Use ref.read() for dependencies in Provider constructors
// 4. DO NOT use sl<T>() or ServiceLocator.getService<T>()
// 5. Return domain entities or use cases, not data sources
//
// **CORRECT DEPENDENCY INJECTION:**
// ```dart
// final repository = ref.read(profileRepositoryProvider);  // ✅ From data layer
// final logger = ref.read(loggerServiceProvider);          // ✅ From core
// ```
//
// **WRONG PATTERNS:**
// ```dart
// final api = sl<ApiClient>();                    // ❌ Don't use sl<T>()
// final service = ServiceLocator.getService();      // ❌ Don't use ServiceLocator
// final repo = ProfileRepository();                // ❌ Don't instantiate directly
// ```
// =============================================================================

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:labuda/domains/user/profile/domain/use_cases/get_profile_use_case.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart';

// Use case providers
final getProfileUseCaseProvider = Provider<GetProfileUseCase>((ref) {
  final repository = ref.read(profileRepositoryProvider);
  return GetProfileUseCase(repository);
});

// Profile state provider - simple FutureProvider approach
final profileProvider = FutureProvider.family<ProfileEntity?, String>((
  ref,
  userId,
) async {
  final useCase = ref.read(getProfileUseCaseProvider);
  final result = await useCase(userId);

  if (result.isSuccess) {
    return result.data;
  } else {
    throw Exception(result.error);
  }
});


