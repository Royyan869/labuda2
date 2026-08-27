/// For Sale Feature Module
///
/// The fixed-price sale channel — a sibling of Auction over Product, never
/// its parent. Direct backend API integration, no collection dependency.
///
/// Architecture:
/// - ForSaleRemoteDatasource: Direct /api/v1/for-sale API calls
/// - ForSaleRepositoryImpl: Uses datasource (no collection dependency)
/// - ForSale entity: Domain model for fixed-price sales
library;

// Domain
export 'domain/domain.dart';

// Data
export 'data/data.dart';

// Presentation
export 'presentation/presentation.dart';
