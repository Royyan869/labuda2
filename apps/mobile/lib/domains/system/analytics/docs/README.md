# Analytics Module

## Overview

Product analytics for Labuda mobile. This module is the single implementation of
the canonical `IAnalyticsRepository` contract, backed by Firebase Analytics.

## Boundary

This module measures **what users do** (product/behaviour/business events). It is
deliberately NOT:

- crash/error monitoring — see `core/observability/crash_reporting.dart`
- performance monitoring — see `core/observability/performance_monitor.dart`
- diagnostic logging — see `ILoggerService`

It is never the authority for business/financial state. Payment/order/seller
truth stays with the owning domain.

## Architecture

```
Feature / domain
      │  ref.read(coreAnalyticsRepositoryProvider)
      ▼
IAnalyticsRepository            (core/src/interfaces/services)
      ▼
FirebaseAnalyticsRepositoryImpl (data/repositories)
      ▼
FirebaseAnalyticsService        (data/services)
      ▼
FirebaseAnalytics.instance
```

- **Event taxonomy:** `core/observability/analytics_events.dart` (`AnalyticsEvents`,
  `AnalyticsParams`, `AnalyticsAuthMethods`). Never use raw event-name literals.
- **Screen taxonomy:** `core/observability/screen_names.dart` (`AnalyticsScreen`).
  Screen names describe the KIND of screen, never a resource identifier.
- **Wiring:** `main.dart` constructs the service/repository and enables collection.

## Files

- `data/services/firebase_analytics_service.dart` — SDK wrapper.
- `data/repositories/firebase_analytics_repository_impl.dart` — canonical impl.

## Active capability

Analytics collection is enabled explicitly at startup. Delivery to GA4 depends on
the Firebase project having Google Analytics for Firebase provisioned and a
measurement id present in the platform config (`google-services.json` /
`GoogleService-Info.plist` / `firebase_options.dart`). See the convergence report
for the current provisioning status.
