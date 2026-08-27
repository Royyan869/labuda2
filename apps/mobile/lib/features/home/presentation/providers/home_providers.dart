/// Providers for Home Refactor Module
/// Export semua Riverpod providers untuk use di UI layer
library;

// Feed providers - export everything from feed_notifier
export 'package:labuda/features/home/presentation/providers/feed/feed_notifier.dart';

// Tab switch providers
export 'package:labuda/features/home/presentation/providers/tab/tab_switch_notifier.dart';

// Feed renderers - canonical feed card and navigation state (CONTRACT HYGIENE PASS V1)
// Previously named "tab_switch_provider_stubs.dart" - renamed for honesty
export 'feed_renderers.dart'
    show
        pendingTabSwitchProvider,
        FeedCardFactory,
        setGlobalFeedRefreshCallback,
        refreshFeedGlobally;
