import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository_provider.dart';
import 'package:labuda/domains/user/preference/saved_item/models/saved_item_model.dart';

final savedItemsProvider = FutureProvider<List<SavedItemModel>>((ref) async {
  final repository = ref.watch(savedItemRepositoryProvider);
  return repository.getSavedItems();
});

final savedItemsCountProvider = FutureProvider<int>((ref) async {
  final repository = ref.watch(savedItemRepositoryProvider);
  return repository.getSavedItemsCount();
});
