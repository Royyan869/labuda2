import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/src/router/route_paths.dart';

void main() {
  test('for sale route paths stay fixed-price-sale specific', () {
    expect(RoutePaths.forSaleDetail, '/for-sale/:forSaleId');
    expect(RoutePaths.editForSale, '/for-sale/:forSaleId/edit');
    expect(RoutePaths.checkout, '/checkout/:forSaleId');
    expect(RoutePaths.forSaleDetail.contains('productId'), isFalse);
    expect(RoutePaths.editForSale.contains('productId'), isFalse);
  });
}
