import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/features/search/search/presentation/providers/providers.dart';
import 'package:hishumi/features/search/search/domain/usecases/search_usecase.dart';

/// Search UseCase Providers
///
/// Provides usecase instances with repository injection.

/// Search UseCase Provider
final searchUseCaseProvider = Provider<SearchUseCase>((ref) {
  final repository = ref.watch(searchRepositoryProvider);
  return SearchUseCase(repository);
});
