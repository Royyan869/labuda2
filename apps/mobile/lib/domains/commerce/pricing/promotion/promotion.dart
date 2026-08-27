/// Promotion feature barrel file.
///
/// Provides duration-based promotion entitlement for listings,
/// auctions, and external products.
///
/// This is NOT an ad platform - it's a simple promotion system
/// where users purchase packages and activate them on targets.
library;

export 'data/dto/external_product_dto.dart';
export 'data/dto/promotion_dto.dart';
export 'domain/entities/external_product.dart';
export 'domain/entities/external_product_media.dart';
export 'domain/entities/external_product_review_status.dart';
export 'domain/entities/instance_status.dart';
export 'domain/entities/ownership_status.dart';
export 'domain/entities/promotion_instance.dart';
export 'domain/entities/promotion_ownership.dart';
export 'domain/entities/promotion_package.dart';
export 'domain/entities/stop_reason.dart';
export 'domain/entities/target_type.dart';
export 'domain/repositories/promotion_repository.dart';
