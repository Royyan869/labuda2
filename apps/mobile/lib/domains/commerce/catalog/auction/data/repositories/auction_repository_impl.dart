/// Auction Repository Implementation
/// API-based implementation using AuctionRemoteDatasource
library;

import 'dart:async';
import 'dart:collection';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/core/common/types/preparation_time.dart';
import 'package:hishumi/core/utils/polling_monitor.dart';
import 'package:hishumi/domains/commerce/catalog/auction/data/dto/auction_dto.dart';
import 'package:hishumi/domains/commerce/catalog/auction/data/mappers/auction_mapper.dart';
import 'package:hishumi/domains/commerce/catalog/auction/data/remote/auction_remote_datasource.dart';
import 'package:hishumi/domains/commerce/catalog/auction/domain/domain.dart';

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

  // Last emitted snapshots per auctionId — polling dedup so identical
  // responses never re-emit (no widget churn) while real changes always do.
  final Map<String, Auction?> _lastAuctionSnapshot = {};
  final Map<String, SplayTreeMap<String, AuctionBid>> _lastBidSnapshots = {};

  // Polling monitor for tracking auction polling status
  final Map<String, PollingMonitor> _auctionMonitors = {};

  AuctionRepositoryImpl({
    required AuctionRemoteDatasource datasource,
    required ILoggerService logger,
  }) : _datasource = datasource,
       _logger = logger;

  // ========== Auction CRUD Operations ==========

  @override
  Future<Result<Auction>> createAuction({
    required String sellerId,
    String? sellerUsername,
    String? sellerFarmName,
    String? sellerAvatar,
    required String title,
    required String description,
    required List<String> mediaUrls,
    required List<AuctionMediaType> mediaTypes,
    required KoiDetails koiDetails,
    required int openingBid,
    required int bidIncrement,
    int? buyNowPrice,
    required String startMode,
    DateTime? scheduledStartAt,
    required int durationHours,
    required PreparationTime preparationTime,
    required List<String> shippingSetupIds,
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
        preparationTime: preparationTime,
        shippingSetupIds: shippingSetupIds,
      );

      final dto = await AuctionMapper.toCreateDto(params);
      final result = await _datasource.createAuction(dto);
      final entity = AuctionMapper.toEntity(result);

      return Result.success(entity);
    } on StructuredApiException catch (e) {
      _logger.error('Failed to create auction: ${e.message}');
      return Result.error(e.message, code: e.code, details: e.details);
    } catch (e) {
      _logger.error('Failed to create auction: $e');
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<Auction>> getAuctionById(String auctionId) async {
    try {
      final dto = await _datasource.getAuctionById(auctionId);
      final entity = AuctionMapper.toEntity(dto);
      return Result.success(entity);
    } catch (e) {
      _logger.error('Failed to get auction: $e');
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<List<Auction>>> getAuctionsByIds(
    List<String> auctionIds,
  ) async {
    try {
      if (auctionIds.isEmpty) {
        return Result.success([]);
      }

      final dtos = await _datasource.getAuctionsByIds(auctionIds);
      final entities = dtos.map(AuctionMapper.toEntity).toList();
      return Result.success(entities);
    } catch (e) {
      _logger.error('Failed to get auctions by IDs: $e');
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<List<Auction>>> getActiveAuctions({
    String? variety,
    double? minSize,
    double? maxSize,
    double? maxBid,
    int limit = 20,
    String? lastAuctionId,
  }) async {
    try {
      // Canonical browse = scheduled + active (backend default IN ('scheduled','active')).
      // Previous strict 'active' hid scheduled upcoming auctions from marketplace & tests.
      // Fetch default (no status filter) so scheduled upcoming is discoverable.
      final dtos = await _datasource.getAuctions(
        limit: limit,
        cursor: lastAuctionId,
      );

      final entities = dtos.map(AuctionMapper.toEntity).toList();
      // Keep discoverable only (scheduled + active) – defensive filter for any future status drift.
      final discoverable = entities
          .where(
            (a) =>
                a.status == AuctionStatus.scheduled ||
                a.status == AuctionStatus.active,
          )
          .toList();
      return Result.success(discoverable);
    } catch (e) {
      _logger.error('Failed to get active auctions: $e');
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<List<Auction>>> getUserAuctions({
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
      return Result.success(entities);
    } catch (e) {
      _logger.error('Failed to get user auctions: $e');
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<Auction>> updateAuction(
    String auctionId,
    Map<String, dynamic> updates,
  ) async {
    try {
      final dto = AuctionMapper.toUpdateDto(updates);
      final result = await _datasource.updateAuction(auctionId, dto);
      final entity = AuctionMapper.toEntity(result);
      return Result.success(entity);
    } catch (e) {
      _logger.error('Failed to update auction: $e');
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<void>> cancelAuction({
    required String auctionId,
    required String sellerId,
    required String reason,
  }) async {
    try {
      final dto = CancelAuctionDto(reason: reason);
      await _datasource.cancelAuction(auctionId, dto);
      return Result.success(null);
    } catch (e) {
      _logger.error('Failed to cancel auction: $e');
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<void>> relistAuction({
    required String auctionId,
    required String title,
    required String description,
    required int openingBid,
    required int bidIncrement,
    int? buyNowPrice,
    required String startMode,
    DateTime? scheduledStartAt,
    required int durationHours,
  }) async {
    try {
      await _datasource.relistAuction(auctionId, {
        'title': title,
        'description': description,
        'start_price': openingBid,
        'bid_increment': bidIncrement,
        if (buyNowPrice != null) 'buy_now_price': buyNowPrice,
        'start_mode': startMode,
        if (scheduledStartAt != null)
          'scheduled_start_at': scheduledStartAt.toUtc().toIso8601String(),
        'duration_hours': durationHours,
      });
      return Result.success(null);
    } catch (e) {
      _logger.error('Failed to relist auction: $e');
      return Result.error(e.toString());
    }
  }

  // ========== Bidding Operations ==========

  @override
  Future<Result<AuctionBid>> placeBid({
    required String auctionId,
    required String bidderId,
    required int amount,
  }) async {
    try {
      final dto = PlaceBidDto(amount: amount);
      final result = await _datasource.placeBid(auctionId, dto);
      if (result.isError) {
        // Propagate the API code (e.g. EMAIL_VERIFICATION_REQUIRED,
        // BNR_AUCTION_RESTRICTED) so the notifier/screen can react via
        // state.errorCode and state.errorDetails.
        return Result.error(
          result.error ?? 'Unknown error',
          code: result.errorCode,
          details: result.errorDetails,
        );
      }
      final entity = AuctionMapper.toBidEntity(result.data!);
      return Result.success(entity);
    } catch (e) {
      _logger.error('Failed to place bid: $e');
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<List<AuctionBid>>> getAuctionBids({
    required String auctionId,
    int limit = 50,
  }) async {
    try {
      final dtos = await _datasource.getBidHistory(auctionId, pageSize: limit);
      final entities = dtos.map(AuctionMapper.toBidEntity).toList();
      return Result.success(entities);
    } catch (e) {
      _logger.error('Failed to get auction bids: $e');
      return Result.error(e.toString());
    }
  }

  // ========== Real-time Streams (Polling-based for API) ==========

  // Live polling is reserved for detail/bids (watchAuction/watchBids).
  // LIST discovery lives in the presentation Future providers — one engine
  // with ForSale, no stream path.
  @override
  Stream<Auction?> watchAuction(String auctionId) {
    // Create stream controller if not exists
    if (!_auctionStreamControllers.containsKey(auctionId)) {
      _auctionStreamControllers[auctionId] =
          StreamController<Auction?>.broadcast(
            onListen: () => _startAuctionPolling(auctionId),
            onCancel: () => _stopAuctionPolling(auctionId),
          );

      // Fetch initial data — goes through the shared dedup emitter so the
      // first poll tick cannot re-emit the same snapshot.
      getAuctionById(auctionId).then((result) {
        result.fold((_) => null, (auction) => _emitAuction(auctionId, auction));
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

      // Fetch initial data — shared dedup emitter (see _emitBids).
      getAuctionBids(auctionId: auctionId, limit: limit).then((result) {
        result.fold((_) => null, (bids) => _emitBids(auctionId, bids));
      });
    }

    final controller = _bidStreamControllers[auctionId]!;
    if (controller.hasListener) {
      getAuctionBids(auctionId: auctionId, limit: limit).then((result) {
        result.fold((_) => null, (bids) => _emitBids(auctionId, bids));
      });
    }
    return controller.stream;
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
    // Keep the shared poll loop alive while EITHER live surface (detail or
    // bids) still has a listener — bids-only screens need polling too.
    final auctionController = _auctionStreamControllers[auctionId];
    final bidController = _bidStreamControllers[auctionId];
    final anySurfaceLive =
        (auctionController != null && !auctionController.isClosed) ||
        (bidController != null && !bidController.isClosed);
    if (!anySurfaceLive) {
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

  /// Emit a detail snapshot through the polling dedup gate: identical
  /// snapshots (same public fingerprint) never re-emit; real changes always
  /// do. Shared by initial fetch and poll ticks.
  void _emitAuction(String auctionId, Auction? auction) {
    if (auction == null) return;
    final fingerprint = auctionSnapshotFingerprint(auction);
    final last = _lastAuctionSnapshot[auctionId];
    if (last != null && auctionSnapshotFingerprint(last) == fingerprint) {
      return; // identical snapshot — suppress
    }
    _lastAuctionSnapshot[auctionId] = auction;
    final controller = _auctionStreamControllers[auctionId];
    if (controller != null && !controller.isClosed) {
      controller.add(auction);
    }
  }

  /// Emit a bids snapshot through the dedup gate keyed by bid id + amount.
  void _emitBids(String auctionId, List<AuctionBid> bids) {
    final next = SplayTreeMap<String, AuctionBid>();
    for (final bid in bids) {
      next[bid.id] = bid;
    }
    final last = _lastBidSnapshots[auctionId];
    if (last != null && last.length == next.length) {
      var identicalSnapshot = true;
      for (final entry in next.entries) {
        final prev = last[entry.key];
        if (prev == null || prev.amount != entry.value.amount) {
          identicalSnapshot = false;
          break;
        }
      }
      if (identicalSnapshot) return; // duplicate snapshot — suppress
    }
    _lastBidSnapshots[auctionId] = next;
    final controller = _bidStreamControllers[auctionId];
    if (controller != null && !controller.isClosed) {
      controller.add(bids);
    }
  }

  Future<void> _pollAuction(String auctionId) async {
    // One shared poll tick drives BOTH live surfaces: detail + bids. The
    // previous refactor split these and bids silently stopped refreshing.
    // Each surface is isolated: a failure in one must not stop the other.
    final monitor = _auctionMonitors[auctionId];

    try {
      final result = await getAuctionById(auctionId);
      if (monitor != null) {
        try {
          if (result.isSuccess) {
            monitor.logSuccess();
          } else {
            monitor.logError(result.error ?? 'Unknown error');
          }
        } catch (e) {
          monitor.logError(e.toString());
        }
      }
      result.fold((_) => null, (auction) => _emitAuction(auctionId, auction));
    } catch (_) {
      // Detail surface failure must not kill the bids surface.
    }

    try {
      final bidController = _bidStreamControllers[auctionId];
      if (bidController == null || !bidController.hasListener) return;
      final bidsResult = await getAuctionBids(auctionId: auctionId);
      bidsResult.fold((_) => null, (bids) => _emitBids(auctionId, bids));
    } catch (_) {
      // Transient failure — retain last data, retry next tick.
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
  }
}
