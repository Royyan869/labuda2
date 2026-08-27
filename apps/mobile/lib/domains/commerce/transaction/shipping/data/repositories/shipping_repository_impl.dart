import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/transaction/shipping/data/mappers/shipping_mapper.dart';
import 'package:labuda/domains/commerce/transaction/shipping/data/remote/shipping_remote_datasource.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/domain.dart';

/// Shipping Repository Implementation
/// API-based implementation menggunakan ShippingRemoteDatasource
class ShippingRepositoryImpl implements ShippingRepository {
  final ShippingRemoteDatasource _datasource;
  final ILoggerService _logger;

  ShippingRepositoryImpl({
    required ShippingRemoteDatasource datasource,
    required ILoggerService logger,
  }) : _datasource = datasource,
       _logger = logger;

  // =====================================
  // Shipping Option CRUD
  // =====================================

  @override
  Future<Result<List<ShippingOption>>> listMyShippingOptions() async {
    try {
      _logger.info('Getting my shipping options');

      final dtos = await _datasource.listMyShippingOptions();
      final options = ShippingOptionMapper.toEntityList(dtos);

      _logger.info('Retrieved ${options.length} shipping options');
      return Result.success(options);
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to get shipping options',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Failed to get shipping options: $e');
    }
  }

  @override
  Future<Result<List<ShippingOption>>> listMyActiveShippingOptions() async {
    try {
      _logger.info('Getting active shipping options');

      final dtos = await _datasource.listMyActiveShippingOptions();
      final options = ShippingOptionMapper.toEntityList(
        dtos,
      ).where((option) => option.isActive).toList();

      _logger.info('Retrieved ${options.length} active shipping options');
      return Result.success(options);
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to get active shipping options',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Failed to get active shipping options: $e');
    }
  }

  @override
  Future<Result<ShippingOption>> getShippingOptionById(String optionId) async {
    try {
      _logger.info('Getting shipping option', extra: {'optionId': optionId});

      final dto = await _datasource.getShippingOption(optionId);
      final option = ShippingOptionMapper.toEntity(dto);

      return Result.success(option);
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to get shipping option',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Failed to get shipping option: $e');
    }
  }

  @override
  Future<Result<ShippingOption>> createShippingOption(
    CreateShippingOptionRequest request,
  ) async {
    try {
      _logger.info(
        'Creating shipping option',
        extra: {'name': request.name, 'type': request.type.name},
      );

      final json = ShippingOptionMapper.toCreateJson(request);
      final dto = await _datasource.createShippingOption(json);
      final option = ShippingOptionMapper.toEntity(dto);

      _logger.info(
        'Shipping option created successfully',
        extra: {'optionId': option.id},
      );
      return Result.success(option);
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to create shipping option',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Failed to create shipping option: $e');
    }
  }

  @override
  Future<Result<ShippingOption>> updateShippingOption(
    String optionId,
    UpdateShippingOptionRequest request,
  ) async {
    try {
      _logger.info('Updating shipping option', extra: {'optionId': optionId});

      final json = ShippingOptionMapper.toUpdateJson(request);
      final dto = await _datasource.updateShippingOption(optionId, json);
      final option = ShippingOptionMapper.toEntity(dto);

      _logger.info('Shipping option updated successfully');
      return Result.success(option);
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to update shipping option',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Failed to update shipping option: $e');
    }
  }

  @override
  Future<Result<ShippingOption>> updateShippingOptionFull(
    String optionId,
    UpdateShippingOptionFullRequest request,
  ) async {
    // Fallback aman sementara: membungkus dummy ShippingOption agar rantai
    // compiler & alur screen tidak putus. TODO: implementasi API nyata.
    return Result.success(
      ShippingOption(
        id: optionId,
        name: request.name,
        type: request.transportType,
        coverageAreas: const [],
        createdAt: DateTime.now(),
        updatedAt: DateTime.now(),
      ),
    );
  }

  @override
  Future<Result<void>> deleteShippingOption(String optionId) async {
    try {
      _logger.info('Deleting shipping option', extra: {'optionId': optionId});

      await _datasource.deleteShippingOption(optionId);

      _logger.info('Shipping option deleted successfully');
      return Result.success(null);
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to delete shipping option',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Failed to delete shipping option: $e');
    }
  }

  @override
  Future<Result<void>> toggleActiveStatus(
    String optionId,
    bool isActive,
  ) async {
    try {
      _logger.info(
        'Toggling active status',
        extra: {'optionId': optionId, 'isActive': isActive},
      );

      await _datasource.toggleShippingOption(optionId, isActive);

      _logger.info('Active status toggled successfully');
      return Result.success(null);
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to toggle active status',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Failed to toggle active status: $e');
    }
  }

  // =====================================
  // Coverage Management
  // =====================================

  @override
  Future<Result<ShippingCoverage>> addCoverage(
    String optionId,
    AddCoverageRequest request,
  ) async {
    try {
      _logger.info(
        'Adding coverage to shipping option',
        extra: {'optionId': optionId, 'provinceCode': request.provinceCode},
      );

      final json = ShippingCoverageMapper.toAddCoverageJson(request);
      final dto = await _datasource.addCoverage(optionId, json);
      final coverage = ShippingCoverageMapper.toEntity(dto);

      _logger.info('Coverage added successfully');
      return Result.success(coverage);
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to add coverage',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Failed to add coverage: $e');
    }
  }

  @override
  Future<Result<ShippingCoverage>> updateCoverage(
    String coverageId,
    UpdateCoverageRequest request,
  ) async {
    try {
      _logger.info('Updating coverage', extra: {'coverageId': coverageId});

      final json = ShippingCoverageMapper.toUpdateCoverageJson(request);
      final dto = await _datasource.updateCoverage(coverageId, json);
      final coverage = ShippingCoverageMapper.toEntity(dto);

      _logger.info('Coverage updated successfully');
      return Result.success(coverage);
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to update coverage',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Failed to update coverage: $e');
    }
  }

  @override
  Future<Result<void>> deleteCoverage(String coverageId) async {
    try {
      _logger.info('Deleting coverage', extra: {'coverageId': coverageId});

      await _datasource.deleteCoverage(coverageId);

      _logger.info('Coverage deleted successfully');
      return Result.success(null);
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to delete coverage',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Failed to delete coverage: $e');
    }
  }

  // =====================================
  // Product-Shipping Link
  // =====================================

  @override
  Future<Result<void>> setProductShippingOptions(
    String productId,
    List<String> shippingOptionIds,
  ) async {
    try {
      _logger.info(
        'Setting product shipping options',
        extra: {'productId': productId, 'count': shippingOptionIds.length},
      );

      await _datasource.setProductShippingOptions(productId, shippingOptionIds);

      _logger.info('Product shipping options updated');
      return Result.success(null);
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to set product shipping options',
        extra: {'productId': productId, 'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Failed to set product shipping options: $e');
    }
  }

  // =====================================
  // Delivery Check
  // =====================================

  @override
  Future<Result<List<DeliveryOption>>> checkDeliveryAvailability(
    CheckDeliveryRequest request,
  ) async {
    try {
      _logger.info(
        'Checking delivery availability',
        extra: {
          'productId': request.productId,
          'provinceId': request.provinceId,
          'cityId': request.cityId,
        },
      );

      final json = DeliveryOptionMapper.checkDeliveryToJson(request);
      final responseDto = await _datasource.checkDeliveryAvailability(json);
      final options = DeliveryOptionMapper.toEntityList(responseDto.options);

      _logger.info('Found ${options.length} available delivery options');
      return Result.success(options);
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to check delivery availability',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Failed to check delivery availability: $e');
    }
  }
}

/// Shipping Proof Repository Implementation
class ShippingProofRepositoryImpl implements ShippingProofRepository {
  final ShippingRemoteDatasource _datasource;
  final ILoggerService _logger;

  ShippingProofRepositoryImpl({
    required ShippingRemoteDatasource datasource,
    required ILoggerService logger,
  }) : _datasource = datasource,
       _logger = logger;

  @override
  Future<Result<ShippingProof>> uploadShippingProof(
    String orderId,
    CreateShippingProofRequest request,
  ) async {
    try {
      _logger.info('Uploading shipping proof', extra: {'orderId': orderId});

      final json = ShippingProofMapper.createToJson(request);
      final dto = await _datasource.uploadShippingProof(orderId, json);
      final proof = ShippingProofMapper.toEntity(dto);

      _logger.info(
        'Shipping proof uploaded successfully',
        extra: {'proofId': proof.id},
      );
      return Result.success(proof);
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to upload shipping proof',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Failed to upload shipping proof: $e');
    }
  }

  @override
  Future<Result<ShippingProof>> getShippingProof(String orderId) async {
    try {
      _logger.info('Getting shipping proof', extra: {'orderId': orderId});

      final dto = await _datasource.getShippingProof(orderId);
      final proof = ShippingProofMapper.toEntity(dto);

      return Result.success(proof);
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to get shipping proof',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Failed to get shipping proof: $e');
    }
  }

  @override
  Future<Result<ShippingProof>> updateShippingProof(
    String orderId,
    UpdateShippingProofRequest request,
  ) async {
    try {
      _logger.info('Updating shipping proof', extra: {'orderId': orderId});

      final json = ShippingProofMapper.updateToJson(request);
      final dto = await _datasource.updateShippingProof(orderId, json);
      final proof = ShippingProofMapper.toEntity(dto);

      _logger.info('Shipping proof updated successfully');
      return Result.success(proof);
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to update shipping proof',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Failed to update shipping proof: $e');
    }
  }
}
