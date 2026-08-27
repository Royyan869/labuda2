/// Auction Repository Implementation
/// API-based implementation using AuctionRemoteDatasource
library;

import 'dart:async';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/utils/polling_monitor.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/dto/auction_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/mappers/auction_mapper.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/remote/auction_remote_datasource.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/repositories/repository_result.dart';

/// Auction Repository Implementation
///
/// API implementation of AuctionRepository interface.
/// Uses AuctionRemoteDatasource for HTTP operations and AuctionMapper for conversions.
class AuctionRepositoryImpl implements AuctionRepository {
  final AuctionRemoteDatasource _datasource;
  final ILoggerService _logger;

  // Stream controllers for real-time auction updates (polling-based for API)
  final Map<String, StreamController<Auction?>> _auctionStreamControllers = {};
  final Map<String, StreamController<List<AuctionBid>>> _bidStreamControllers =
      {};
  final Map<String, Timer?> _pollingTimers = {};

  // Polling monitors for tracking auction polling status
  final Map<String, PollingMonitor> _auctionMonitors = {};
  final Map<String, PollingMonitor> _bidMonitors = {};

  AuctionRepositoryImpl({
    required AuctionRemoteDatasource datasource,
    required ILoggerService logger,
  }) : _datasource = datasource,
       _logger = logger;

  // ========== Auction CRUD Operations ==========

  @override
  Future<RepositoryResult<Auction>> createAuction({
    required String sellerId,
    String? sellerUsername,
    String? sellerFarmName,
    String? sellerAvatar,
    required String title,
    required String description,
    required List<String> mediaUrls,
    required List<AuctionMediaType> mediaTypes,
    required KoiDetails koiDetails,
    required double openingBid,
    required double bidIncrement,
    double? buyNowPrice,
    required String startMode,
    DateTime? scheduledStartAt,
    required int durationHours,
    String? farmAddressId,
    AuctionLocation? location,
    required List<String> shippingOptionIds,
    String? preparationNote,
  }) async {
    try {
      final params = CreateAuctionParams(
        sellerId: sellerId,
        sellerUsername: sellerUsername,
        sellerFarmName: sellerFarmName,
        sellerAvatar: sellerAvatar,
        title: title,
        description: description,
        mediaUrls: mediaUrls,
        mediaTypes: mediaTypes,
        koiDetails: koiDetails,
        openingBid: openingBid,
        bidIncrement: bidIncrement,
        buyNowPrice: buyNowPrice,
        startMode: startMode,
        scheduledStartAt: scheduledStartAt,
        durationHours: durationHours,
        farmAddressId: farmAddressId,
        location: location,
        shippingOptionIds: shippingOptionIds,
        preparationNote: preparationNote,
      );

      final dto = AuctionMapper.toCreateDto(params);
      final result = await _datasource.createAuction(dto);
      final entity = AuctionMapper.toEntity(result);

      return RepositoryResult.success(entity);
    } catch (e) {
      _logger.error('Failed to create auction: $e');
      return RepositoryResult.error(e.toString());
    }
  }

  @override
  Future<RepositoryResult<Auction>> getAuctionById(String auctionId) async {
    try {
      final dto = await _datasource.getAuctionById(auctionId);
      final entity = AuctionMapper.toEntity(dto);
      return RepositoryResult.success(entity);
    } catch (e) {
      _logger.error('Failed to get auction: $e');
      return RepositoryResult.error(e.toString());
    }
  }

  @override
  Future<RepositoryResult<List<Auction>>> getAuctionsByIds(
    List<String> auctionIds,
  ) async {
    try {
      if (auctionIds.isEmpty) {
        return RepositoryResult.success([]);
      }

      final dtos = await _datasource.getAuctionsByIds(auctionIds);
      final entities = dtos.map(AuctionMapper.toEntity).toList();
      return RepositoryResult.success(entities);
    } catch (e) {
      _logger.error('Failed to get auctions by IDs: $e');
      return RepositoryResult.error(e.toString());
    }
  }

  @override
  Future<RepositoryResult<List<Auction>>> getActiveAuctions({
    String? variety,
    double? minSize,
    double? maxSize,
    double? maxBid,
    int limit = 20,
    String? lastAuctionId,
  }) async {
    try {
      final dtos = await _datasource.getAuctions(
        status: 'active',
        limit: limit,
        cursor: lastAuctionId,
      );

      final entities = dtos.map(AuctionMapper.toEntity).toList();
      return RepositoryResult.success(entities);
    } catch (e) {
      _logger.error('Failed to get active auctions: $e');
      return RepositoryResult.error(e.toString());
    }
  }

  @override
  Future<RepositoryResult<List<Auction>>> getUserAuctions({
    required String sellerId,
    AuctionStatus? status,
    int limit = 20,
    String? lastAuctionId,
  }) async {
    try {
      final dtos = await _datasource.getAuctions(
        sellerId: sellerId,
        status: status != null ? AuctionMapper.mapStatusToApi(status) : null,
        limit: limit,
        cursor: lastAuctionId,
      );

      final entities = dtos.map(AuctionMapper.toEntity).toList();
      return RepositoryResult.success(entities);
    } catch (e) {
      _logger.error('Failed to get user auctions: $e');
      return RepositoryResult.error(e.toString());
    }
  }

  @override
  Future<RepositoryResult<Auction>> updateAuction(
    String auctionId,
    Map<String, dynamic> updates,
  ) async {
    try {
      final dto = AuctionMapper.toUpdateDto(updates);
      final result = await _datasource.updateAuction(auctionId, dto);
      final entity = AuctionMapper.toEntity(result);
      return RepositoryResult.success(entity);
    } catch (e) {
      _logger.error('Failed to update auction: $e');
      return RepositoryResult.error(e.toString());
    }
  }

  @override
  Future<RepositoryResult<Auction>> updateAuctionStatus({
    required String auctionId,
    required AuctionStatus status,
  }) async {
    try {
      if (status == AuctionStatus.active) {
        await _datasource.scheduleAuction(auctionId);
        final refreshed = await _datasource.getAuctionById(auctionId);
        return RepositoryResult.success(AuctionMapper.toEntity(refreshed));
      }

      // For other status changes, use update endpoint
      final dto = UpdateAuctionDto();
      final result = await _datasource.updateAuction(auctionId, dto);
      final entity = AuctionMapper.toEntity(result);
      return RepositoryResult.success(entity);
    } catch (e) {
      _logger.error('Failed to update auction status: $e');
      return RepositoryResult.error(e.toString());
    }
  }

  @override
  Future<RepositoryResult<void>> cancelAuction({
    required String auctionId,
    required String sellerId,
    required String reason,
  }) async {
    try {
      final dto = CancelAuctionDto(reason: reason);
      await _datasource.cancelAuction(auctionId, dto);
      return RepositoryResult.success(null);
    } catch (e) {
      _logger.error('Failed to cancel auction: $e');
      return RepositoryResult.error(e.toString());
    }
  }

  // Note: View tracking is handled by backend automatically
  // No explicit incrementViewCount needed

  // ========== Bidding Operations ==========

  @override
  Future<RepositoryResult<AuctionBid>> placeBid({
    required String auctionId,
    required String bidderId,
    required double amount,
  }) async {
    try {
      final dto = PlaceBidDto(amount: amount);
      final result = await _datasource.placeBid(auctionId, dto);
      if (result.isError) {
        // Propagate the API code (e.g. EMAIL_VERIFICATION_REQUIRED,
        // BNR_AUCTION_RESTRICTED) so the notifier/screen can react via
        // state.errorCode and state.errorDetails.
        return RepositoryResult.error(
          result.error ?? 'Unknown error',
          code: result.errorCode,
          details: result.errorDetails,
        );
      }
      final entity = AuctionMapper.toBidEntity(result.data!);
      return RepositoryResult.success(entity);
    } catch (e) {
      _logger.error('Failed to place bid: $e');
      return RepositoryResult.error(e.toString());
    }
  }

  @override
  Future<RepositoryResult<List<AuctionBid>>> getAuctionBids({
    required String auctionId,
    int limit = 50,
  }) async {
    try {
      final dtos = await _datasource.getBidHistory(auctionId, pageSize: limit);
      final entities = dtos.map(AuctionMapper.toBidEntity).toList();
      return RepositoryResult.success(entities);
    } catch (e) {
      _logger.error('Failed to get auction bids: $e');
      return RepositoryResult.error(e.toString());
    }
  }

  @override
  Future<RepositoryResult<String>> claimAuction({
    required String auctionId,
    required String addressId,
    required String shippingOptionId,
    String? discountCode,
    bool useCoins = false,
  }) async {
    try {
      final orderId = await _datasource.claimAuction(
        auctionId,
        addressId: addressId,
        shippingOptionId: shippingOptionId,
        discountCode: discountCode,
        useCoins: useCoins,
      );
      return RepositoryResult.success(orderId);
    } catch (e) {
      _logger.error('Failed to claim auction: $e');
      return RepositoryResult.error(e.toString());
    }
  }

  // ========== Real-time Streams (Polling-based for API) ==========

  @override
  Stream<List<Auction>> watchUserAuctions({
    required String sellerId,
    AuctionStatus? status,
    int limit = 100,
  }) {
    // API implementation uses polling instead of Firestore streams
    return Stream.periodic(
      const Duration(seconds: 15),
      (_) => sellerId,
    ).asyncMap((_) async {
      final result = await getUserAuctions(
        sellerId: sellerId,
        status: status,
        limit: limit,
      );
      return result.fold((auctions) => auctions, (_) => <Auction>[]);
    });
  }

  @override
  Stream<List<Auction>> watchActiveAuctions({int limit = 50}) {
    // Create a stream controller that emits immediately on listen
    final controller = StreamController<List<Auction>>.broadcast();

    // Fetch and emit initial data immediately
    void fetchData() async {
      final result = await getActiveAuctions(limit: limit);
      final auctions = result.fold((auctions) => auctions, (_) => <Auction>[]);
      if (!controller.isClosed) {
        controller.add(auctions);
      }
    }

    // Start polling when someone listens
    Timer? pollingTimer;

    controller.onListen = () {
      // Fetch immediately
      fetchData();

      // Then poll every 30 seconds
      pollingTimer = Timer.periodic(const Duration(seconds: 30), (_) {
        fetchData();
      });
    };

    controller.onCancel = () {
      pollingTimer?.cancel();
    };

    return controller.stream;
  }

  @override
  Stream<Auction?> watchAuction(String auctionId) {
    // Create stream controller if not exists
    if (!_auctionStreamControllers.containsKey(auctionId)) {
      _auctionStreamControllers[auctionId] =
          StreamController<Auction?>.broadcast(
            onListen: () => _startAuctionPolling(auctionId),
            onCancel: () => _stopAuctionPolling(auctionId),
          );

      // Fetch initial data
      getAuctionById(auctionId).then((result) {
        result.fold((auction) {
          final controller = _auctionStreamControllers[auctionId];
          if (controller != null && !controller.isClosed) {
            controller.add(auction);
          }
        }, (_) => null);
      });
    }

    return _auctionStreamControllers[auctionId]!.stream;
  }

  @override
  Stream<List<AuctionBid>> watchAuctionBids(
    String auctionId, {
    int limit = 50,
  }) {
    // Create stream controller if not exists
    if (!_bidStreamControllers.containsKey(auctionId)) {
      _bidStreamControllers[auctionId] =
          StreamController<List<AuctionBid>>.broadcast(
            onListen: () => _startBidPolling(auctionId, limit),
            onCancel: () => _stopBidPolling(auctionId),
          );

      // Fetch initial data
      getAuctionBids(auctionId: auctionId, limit: limit).then((result) {
        result.fold((bids) {
          final controller = _bidStreamControllers[auctionId];
          if (controller != null && !controller.isClosed) {
            controller.add(bids);
          }
        }, (_) => null);
      });
    }

    return _bidStreamControllers[auctionId]!.stream;
  }

  // ========== Private Polling Methods ==========

  void _startAuctionPolling(String auctionId) {
    if (_pollingTimers.containsKey(auctionId)) return;

    // Create monitor for this auction if not exists
    if (!_auctionMonitors.containsKey(auctionId)) {
      _auctionMonitors[auctionId] = PollingMonitor(
        logger: _logger,
        domain: PollingDomain.auction,
        operationId: auctionId,
        config: PollingBackoffConfig.auction,
      );
    }

    final monitor = _auctionMonitors[auctionId]!;

    // Schedule first poll with dynamic interval
    _scheduleAuctionPoll(auctionId, monitor);
  }

  void _scheduleAuctionPoll(String auctionId, PollingMonitor monitor) {
    if (_auctionStreamControllers[auctionId]?.isClosed ?? true) {
      return;
    }

    final interval = monitor.getCurrentInterval();

    _pollingTimers[auctionId] = Timer(interval, () async {
      await _pollAuction(auctionId);
      // Schedule next poll
      if (_pollingTimers.containsKey(auctionId)) {
        _scheduleAuctionPoll(auctionId, monitor);
      }
    });
  }

  void _stopAuctionPolling(String auctionId) {
    _pollingTimers[auctionId]?.cancel();
    _pollingTimers.remove(auctionId);

    // Clean up monitor
    _auctionMonitors.remove(auctionId);

    // Close stream controller if no more listeners
    final controller = _auctionStreamControllers[auctionId];
    if (controller != null && !controller.hasListener) {
      controller.close();
      _auctionStreamControllers.remove(auctionId);
    }
  }

  Future<void> _pollAuction(String auctionId) async {
    final monitor = _auctionMonitors[auctionId];
    final result = await getAuctionById(auctionId);

    if (monitor != null) {
      // Log with monitoring
      try {
        if (result.isSuccess) {
          monitor.logSuccess();
          final auction = result.data;
          final controller = _auctionStreamControllers[auctionId];
          if (controller != null && !controller.isClosed && auction != null) {
            controller.add(auction);
          }
        } else {
          monitor.logError(result.error ?? 'Unknown error');
        }
      } catch (e) {
        monitor.logError(e.toString());
      }
    } else {
      // Fallback without monitoring
      result.fold((auction) {
        final controller = _auctionStreamControllers[auctionId];
        if (controller != null && !controller.isClosed) {
          controller.add(auction);
        }
      }, (_) => null);
    }
  }

  void _startBidPolling(String auctionId, int limit) {
    // Share auction polling timer
    if (!_pollingTimers.containsKey(auctionId)) {
      _startAuctionPolling(auctionId);
    }
  }

  void _stopBidPolling(String auctionId) {
    // Bids share the auction polling timer, only stop if auction also not listening
    final auctionController = _auctionStreamControllers[auctionId];
    final bidController = _bidStreamControllers[auctionId];

    if ((auctionController == null || !auctionController.hasListener) &&
        (bidController == null || !bidController.hasListener)) {
      _stopAuctionPolling(auctionId);
    }

    // Close bid stream controller if no more listeners
    if (bidController != null && !bidController.hasListener) {
      bidController.close();
      _bidStreamControllers.remove(auctionId);
    }
  }

  /// Dispose all resources
  void dispose() {
    for (final timer in _pollingTimers.values) {
      timer?.cancel();
    }
    _pollingTimers.clear();

    for (final controller in _auctionStreamControllers.values) {
      controller.close();
    }
    _auctionStreamControllers.clear();

    for (final controller in _bidStreamControllers.values) {
      controller.close();
    }
    _bidStreamControllers.clear();

    // Clean up monitors
    _auctionMonitors.clear();
    _bidMonitors.clear();
  }

  /// Get polling status for an auction (for UI/debugging)
  Map<String, dynamic>? getAuctionPollingStatus(String auctionId) {
    final monitor = _auctionMonitors[auctionId];
    return monitor?.getStatusSummary();
  }
}
