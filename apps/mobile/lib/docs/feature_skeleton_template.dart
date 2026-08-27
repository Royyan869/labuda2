// =============================================================================
// FEATURE SKELETON TEMPLATE - Minimal Feature Structure (R5)
// =============================================================================
//
// This is a minimal template showing the correct structure for a new feature.
// Use this as a starting point when creating new features.
//
// **STRUCTURE:**
// - domain/entities/     - Pure domain models
// - domain/repositories/ - Abstract interfaces
// - domain/use_cases/    - Business logic
// - data/                - Implementation details (NOT exported)
// - presentation/        - UI layer (screens, widgets, providers)
//
// **DEPENDENCY RULES:**
// - Presentation → Data via Providers (ref.watch/ref.read)
// - Data → Core Services via Providers (sl<T>() ONLY in data providers)
// - NO direct sl<T>() in presentation layer
//
// =============================================================================
// NOTE: This is a DOCUMENTATION/TEMPLATE file, not executable code.
// The examples below show the canonical patterns to follow.
// =============================================================================

// =============================================================================
// EXAMPLE 1: DOMAIN ENTITY (Pure Domain Model)
// =============================================================================
//
// File: domain/entities/example_entity.dart
//
// ```dart
// class ExampleEntity {
//   final String id;
//   final String name;
//   final DateTime createdAt;
//
//   ExampleEntity({
//     required this.id,
//     required this.name,
//     required this.createdAt,
//   });
// }
// ```

// =============================================================================
// EXAMPLE 2: REPOSITORY INTERFACE (Contract)
// =============================================================================
//
// File: domain/repositories/i_example_repository.dart
//
// ```dart
// import 'package:labuda/core/src/api/result.dart';
// import 'package:labuda/features/example/domain/entities/example_entity.dart';
//
// abstract class IExampleRepository {
//   Future<Result<List<ExampleEntity>>> getExamples(String userId);
//   Future<Result<ExampleEntity>> getExampleById(String id);
//   Future<Result<void>> createExample(ExampleEntity example);
// }
// ```

// =============================================================================
// EXAMPLE 3: USE CASE (Business Logic)
// =============================================================================
//
// File: domain/use_cases/get_examples_use_case.dart
//
// ```dart
// import 'package:labuda/core/src/api/result.dart';
// import 'package:labuda/features/example/domain/entities/example_entity.dart';
// import 'package:labuda/features/example/domain/repositories/i_example_repository.dart';
//
// class GetExamplesUseCase {
//   final IExampleRepository repository;
//
//   GetExamplesUseCase(this.repository);
//
//   Future<Result<List<ExampleEntity>>> call(String userId) async {
//     // Business logic here
//     if (userId.isEmpty) {
//       return Result.error('User ID cannot be empty');
//     }
//     return await repository.getExamples(userId);
//   }
// }
// ```

// =============================================================================
// EXAMPLE 4: DATA PROVIDERS (Riverpod - Data Layer)
// =============================================================================
//
// File: data/example_providers.dart
//
// ```dart
// import 'package:flutter_riverpod/flutter_riverpod.dart';
// import 'package:labuda/core/core.dart'; // For apiClientProvider, etc.
// import 'package:labuda/features/example/domain/repositories/i_example_repository.dart';
//
// // Import datasource (internal)
// import 'datasources/example_api_datasource.dart';
// import 'repositories/example_repository_impl.dart';
//
// /// Repository Provider - Exposed to presentation layer
// final exampleRepositoryProvider = Provider<IExampleRepository>((ref) {
//   final apiClient = ref.watch(apiClientProvider);
//   final datasource = ExampleApiDatasource(apiClient: apiClient);
//   return ExampleRepositoryImpl(datasource: datasource);
// });
// ```

// =============================================================================
// EXAMPLE 5: PRESENTATION PROVIDERS (Riverpod - UI Layer)
// =============================================================================
//
// File: presentation/providers/example_providers.dart
//
// ```dart
// // =============================================================================
// // ARCHITECTURE GUARDRAIL (R5) - FEATURE PROVIDER PATTERN
// // =============================================================================
// // 1. Import data layer providers using `show` to expose only what's needed
// // 2. Import core services from core/providers/core_providers.dart
// // 3. Use ref.read() for dependencies in Provider constructors
// // 4. DO NOT use sl<T>() or ServiceLocator.getService<T>()
// // =============================================================================
//
// import 'package:flutter_riverpod/flutter_riverpod.dart';
// import 'package:labuda/features/example/domain/entities/example_entity.dart';
// import 'package:labuda/features/example/domain/use_cases/get_examples_use_case.dart';
//
// // Import ONLY what you need from data layer
// import 'package:labuda/features/example/data/example_providers.dart'
//     show exampleRepositoryProvider;
//
// // Use Case Provider
// final getExamplesUseCaseProvider = Provider<GetExamplesUseCase>((ref) {
//   final repository = ref.read(exampleRepositoryProvider);
//   return GetExamplesUseCase(repository);
// });
//
// // UI Provider - Exposes data to widgets
// final examplesProvider = FutureProvider.family<List<ExampleEntity>, String>((
//   ref,
//   userId,
// ) async {
//   final useCase = ref.read(getExamplesUseCaseProvider);
//   final result = await useCase(userId);
//
//   return result.fold(
//     (error) => throw Exception(error),
//     (examples) => examples,
//   );
// });
// ```

// =============================================================================
// EXAMPLE 6: BARREL FILE (Public API)
// =============================================================================
//
// File: example_feature.dart
//
// ```dart
// library;
//
// // Domain - Public API
// export 'domain/entities/example_entity.dart';
// export 'domain/repositories/i_example_repository.dart';
// export 'domain/use_cases/get_examples_use_case.dart';
//
// // Providers - Public API
// export 'data/example_providers.dart';
// export 'presentation/providers/example_providers.dart';
//
// // Presentation - Public API
// export 'presentation/screens/example_screen.dart';
//
// // =============================================================================
// // ❌ DILARANG: Data layer (dto, mappers, repositories_impl, datasources)
// // =============================================================================
// // These are implementation details and should NOT be exported:
// // - data/models/*
// // - data/datasources/*
// // - data/mappers/*
// // - data/repositories/*_impl.dart
// //
// // External code should only depend on:
// // - Domain entities
// // - Repository interfaces
// // - Use cases
// // - Presentation providers
// // - UI screens/widgets
// // =============================================================================
// ```

// =============================================================================
// SUMMARY: CANONICAL PATTERN
// =============================================================================
//
// ✅ CORRECT:
// ```dart
// // In presentation/providers
// import 'package:labuda/features/example/data/example_providers.dart'
//     show exampleRepositoryProvider;
//
// final provider = Provider<UseCase>((ref) {
//   final repository = ref.read(exampleRepositoryProvider);  // ✅ Via Riverpod
//   return UseCase(repository);
// });
// ```
//
// ❌ WRONG:
// ```dart
// final provider = Provider<UseCase>((ref) {
//   final repo = sl<IExampleRepository>();  // ❌ Don't use sl<T>()!
//   return UseCase(repo);
// });
// ```
