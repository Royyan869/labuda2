/// Report DI Helper
library;

import 'package:labuda/domains/system/report/presentation/providers/report_providers.dart';

/// Report DI
class ReportDI {
  static List overrides({String? currentUserId}) {
    return [
      if (currentUserId != null)
        reportCurrentUserIdProvider.overrideWithValue(currentUserId),
    ];
  }
}
