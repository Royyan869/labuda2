import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/system/report/report.dart';

void main() {
  test('forSale serializes to for_sale for backend requests', () {
    expect(ReportTargetType.forSale.backendValue, 'for_sale');
    expect(ReportTargetType.forSale.displayName, 'For Sale');
  });

  test('canonical targets only — chat_message is not a target', () {
    expect(ReportTargetType.values.map((e) => e.backendValue),
        ['content', 'comment', 'user', 'for_sale', 'auction']);
  });
}
