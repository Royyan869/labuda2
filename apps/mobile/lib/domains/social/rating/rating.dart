// Barrel file for rating module

// Domain layer exports
export 'domain/entities/rating_entity.dart';
export 'domain/repositories/i_rating_repository.dart';

// Data layer - Providers (Riverpod)
export 'data/rating_providers.dart';

// Presentation layer exports
export 'presentation/providers/rating_provider.dart';
export 'presentation/widgets/rating_card.dart';
export 'presentation/pages/rating_list_screen.dart';
