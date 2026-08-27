import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';
import 'package:labuda/features/search/search/presentation/utils/search_result_type_helper.dart';
import 'package:labuda/features/search/search/presentation/widgets/global_search_bar.dart';

void main() {
  test('content label is rendered as Content', () {
    expect(SearchResultTypeHelper.getLabel(SearchResultType.content), 'Content');
  });

  testWidgets('global search bar exposes a Content filter chip', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: GlobalSearchBar(showCategoryChips: true),
        ),
      ),
    );

    expect(find.widgetWithText(FilterChip, 'Content'), findsOneWidget);
  });
}
