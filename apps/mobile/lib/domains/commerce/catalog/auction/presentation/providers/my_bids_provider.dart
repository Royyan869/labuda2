import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/commerce/catalog/auction/data/auction_providers.dart';
import 'package:hishumi/domains/commerce/catalog/auction/data/dto/bidding_item_dto.dart';
import 'package:hishumi/domains/commerce/catalog/auction/data/repositories/my_bids_repository.dart';

final myBidsProvider = FutureProvider.autoDispose<List<BiddingItemDto>>((
  ref,
) async {
  final repository = MyBidsRepository(
    ref.watch(auctionRemoteDatasourceProvider),
    ref.watch(loggerServiceProvider),
  );
  final result = await repository.getMyBids();
  return result.fold((error) => throw Exception(error), (items) => items);
});
