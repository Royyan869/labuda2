import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository.dart';
import 'package:labuda/domains/user/preference/saved_item/models/saved_item_model.dart';

class SavedItemService {
  final SavedItemRepository repository;

  SavedItemService({required this.repository});

  Future<List<SavedItemModel>> getSavedItems({String? type}) async {
    return await repository.getSavedItems(type: type);
  }

  Future<SavedItemModel> addSavedItem({
    required String targetType,
    required String targetId,
  }) async {
    return await repository.addSavedItem(
      targetType: targetType,
      targetId: targetId,
    );
  }

  Future<void> removeSavedItem({
    required String targetType,
    required String targetId,
  }) async {
    await repository.removeSavedItem(
      targetType: targetType,
      targetId: targetId,
    );
  }

  Future<void> clearSavedItems({String? type}) async {
    await repository.clearSavedItems(type: type);
  }

  Future<int> getSavedItemsCount({String? type}) async {
    return await repository.getSavedItemsCount(type: type);
  }

  Future<bool> isSaved({
    required String targetType,
    required String targetId,
  }) async {
    return await repository.isSaved(targetType: targetType, targetId: targetId);
  }
}
