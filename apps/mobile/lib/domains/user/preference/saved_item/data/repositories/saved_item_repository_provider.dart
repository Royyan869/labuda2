import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/providers/core_providers.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository.dart';

final savedItemRepositoryProvider = Provider<SavedItemRepository>((ref) {
  return SavedItemRepository(apiClient: ref.watch(apiClientProvider));
});
