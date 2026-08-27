import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/system/report/report.dart';

void main() {
  test(
    'fixedPriceSale serializes to fixed_price_sale for backend requests',
    () {
      expect(ReportTargetType.forSale.backendValue, 'fixed_price_sale');
      expect(ReportTargetType.forSale.displayName, 'Fixed-Price Sale');
    },
  );
}
