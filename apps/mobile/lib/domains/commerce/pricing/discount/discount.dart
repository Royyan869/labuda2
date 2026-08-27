/// Discount Feature - Public API
///
/// Barrel file for discount module.
/// Following RESTRUCTURE.md rules.
library;

// Domain Layer (Entities)
export 'domain/entities/discount_entity.dart';
export 'domain/entities/discount_validation_result.dart';

// Domain Layer (Repository Interface)
export 'domain/repositories/i_discount_repository.dart';

// Domain Layer (Use Cases)
export 'domain/use_cases/create_discount_use_case.dart';
export 'domain/use_cases/delete_discount_use_case.dart';
export 'domain/use_cases/get_seller_discounts_use_case.dart';
export 'domain/use_cases/update_discount_use_case.dart';
export 'domain/use_cases/validate_discount_use_case.dart';

// Data Layer - Providers (Riverpod)
export 'data/discount_providers.dart';

// Presentation Layer (Screens & Providers)
export 'presentation/screens/create_discount_screen.dart';
export 'presentation/screens/edit_discount_screen.dart';
export 'presentation/screens/seller_discount_list_screen.dart';
export 'presentation/providers/discount_provider.dart';

// Presentation Layer (Widgets)
export 'presentation/widgets/discount_card.dart';
export 'presentation/widgets/discount_input_field.dart';

// ❌ DILARANG EXPORT (sesuai RESTRUCTURE.md):
// - data/ (datasources, dto, mappers, repositories_impl)
