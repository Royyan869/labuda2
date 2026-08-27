import 'package:flutter/material.dart';
import 'package:labuda/domains/user/preference/saved_item/models/saved_item_model.dart';
import 'package:labuda/domains/user/preference/saved_item/data/services/saved_item_service.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository.dart';

class SavedItemScreen extends StatefulWidget {
  const SavedItemScreen({super.key});

  @override
  State<SavedItemScreen> createState() => _SavedItemScreenState();
}

class _SavedItemScreenState extends State<SavedItemScreen> {
  final SavedItemService _savedItemService = SavedItemService(
    repository: SavedItemRepository(),
  );

  List<SavedItemModel> _savedItems = [];
  bool _isLoading = true;
  String? _selectedType;

  @override
  void initState() {
    super.initState();
    _loadSavedItems();
  }

  Future<void> _loadSavedItems() async {
    setState(() => _isLoading = true);

    try {
      final items = await _savedItemService.getSavedItems(type: _selectedType);
      setState(() => _savedItems = items);
    } catch (e) {
      // Handle error
    } finally {
      setState(() => _isLoading = false);
    }
  }

  Future<void> _removeItem(SavedItemModel item) async {
    try {
      await _savedItemService.removeSavedItem(
        targetType: item.targetType == TargetType.listing
            ? 'listing'
            : 'auction',
        targetId: item.targetId,
      );
      await _loadSavedItems();
    } catch (e) {
      // Handle error
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Disimpan'),
        actions: [
          PopupMenuButton<String?>(
            initialValue: _selectedType,
            onSelected: (value) {
              setState(() => _selectedType = value);
              _loadSavedItems();
            },
            itemBuilder: (context) => [
              const PopupMenuItem(value: null, child: Text('Semua')),
              const PopupMenuItem(value: 'listing', child: Text('Listing')),
              const PopupMenuItem(value: 'auction', child: Text('Auction')),
            ],
          ),
        ],
      ),
      body: _isLoading
          ? const Center(child: CircularProgressIndicator())
          : _savedItems.isEmpty
          ? const Center(child: Text('Belum ada item yang disimpan'))
          : ListView.builder(
              itemCount: _savedItems.length,
              itemBuilder: (context, index) {
                final item = _savedItems[index];
                return _buildSavedItemCard(item);
              },
            ),
    );
  }

  Widget _buildSavedItemCard(SavedItemModel item) {
    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      child: ListTile(
        leading: Icon(
          item.isListing ? Icons.list_alt : Icons.gavel,
          color: Theme.of(context).primaryColor,
        ),
        title: Text(
          item.isListing
              ? item.listingTitle ?? 'Untitled'
              : item.auctionTitle ?? 'Untitled',
        ),
        subtitle: Text(
          item.isListing
              ? 'Rp ${item.listingPrice ?? 0}'
              : 'Rp ${item.currentBid ?? item.startPrice ?? 0}',
        ),
        trailing: IconButton(
          icon: const Icon(Icons.bookmark_remove),
          onPressed: () => _removeItem(item),
        ),
        onTap: () {
          // Navigate to detail
        },
      ),
    );
  }
}
