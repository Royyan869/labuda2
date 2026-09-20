/// Seller Dashboard Entities
///
/// Pure Dart entities for seller dashboard - no Firebase/Flutter dependencies.
library;

/// Seller Dashboard Statistics
///
/// Aggregated statistics for seller dashboard display.
class SellerDashboardStats {
  final int totalOrders;
  final int pendingOrders;
  final int processingOrders;
  final int completedOrders;
  final int cancelledOrders;
  final int refundedOrders;
  final int problematicOrders;
  final double totalRevenue;
  final double pendingRevenue;
  final double refundedRevenue;
  final int totalForSales;
  final int activeForSales;
  final int soldForSales;
  final int totalAuctions;
  final int activeAuctions;

  const SellerDashboardStats({
    this.totalOrders = 0,
    this.pendingOrders = 0,
    this.processingOrders = 0,
    this.completedOrders = 0,
    this.cancelledOrders = 0,
    this.refundedOrders = 0,
    this.problematicOrders = 0,
    this.totalRevenue = 0,
    this.pendingRevenue = 0,
    this.refundedRevenue = 0,
    this.totalForSales = 0,
    this.activeForSales = 0,
    this.soldForSales = 0,
    this.totalAuctions = 0,
    this.activeAuctions = 0,
  });

  /// Calculate net revenue (total - refunded)
  double get netRevenue => totalRevenue - refundedRevenue;

  /// Calculate active orders (processing + shipped + delivered)
  int get activeOrders => processingOrders;

  /// Create empty stats
  factory SellerDashboardStats.empty() {
    return const SellerDashboardStats();
  }

  /// Copy with
  SellerDashboardStats copyWith({
    int? totalOrders,
    int? pendingOrders,
    int? processingOrders,
    int? completedOrders,
    int? cancelledOrders,
    int? refundedOrders,
    int? problematicOrders,
    double? totalRevenue,
    double? pendingRevenue,
    double? refundedRevenue,
    int? totalForSales,
    int? activeForSales,
    int? soldForSales,
    int? totalAuctions,
    int? activeAuctions,
  }) {
    return SellerDashboardStats(
      totalOrders: totalOrders ?? this.totalOrders,
      pendingOrders: pendingOrders ?? this.pendingOrders,
      processingOrders: processingOrders ?? this.processingOrders,
      completedOrders: completedOrders ?? this.completedOrders,
      cancelledOrders: cancelledOrders ?? this.cancelledOrders,
      refundedOrders: refundedOrders ?? this.refundedOrders,
      problematicOrders: problematicOrders ?? this.problematicOrders,
      totalRevenue: totalRevenue ?? this.totalRevenue,
      pendingRevenue: pendingRevenue ?? this.pendingRevenue,
      refundedRevenue: refundedRevenue ?? this.refundedRevenue,
      totalForSales: totalForSales ?? this.totalForSales,
      activeForSales: activeForSales ?? this.activeForSales,
      soldForSales: soldForSales ?? this.soldForSales,
      totalAuctions: totalAuctions ?? this.totalAuctions,
      activeAuctions: activeAuctions ?? this.activeAuctions,
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is SellerDashboardStats &&
          totalOrders == other.totalOrders &&
          pendingOrders == other.pendingOrders &&
          processingOrders == other.processingOrders &&
          completedOrders == other.completedOrders &&
          cancelledOrders == other.cancelledOrders &&
          refundedOrders == other.refundedOrders &&
          problematicOrders == other.problematicOrders &&
          totalRevenue == other.totalRevenue &&
          pendingRevenue == other.pendingRevenue &&
          refundedRevenue == other.refundedRevenue &&
          totalForSales == other.totalForSales &&
          activeForSales == other.activeForSales &&
          soldForSales == other.soldForSales &&
          totalAuctions == other.totalAuctions &&
          activeAuctions == other.activeAuctions;

  @override
  int get hashCode => Object.hash(
    totalOrders,
    pendingOrders,
    processingOrders,
    completedOrders,
    cancelledOrders,
    refundedOrders,
    problematicOrders,
    totalRevenue,
    pendingRevenue,
    refundedRevenue,
    totalForSales,
    activeForSales,
    soldForSales,
    totalAuctions,
    activeAuctions,
  );
}
