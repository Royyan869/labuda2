import 'package:dio/dio.dart';
import 'package:firebase_performance/firebase_performance.dart';
import 'package:labuda/core/observability/performance_monitor.dart';

/// Canonical network performance instrumentation for the Dart HTTP client.
///
/// Firebase Performance automatically instruments native networking, but the
/// Dart `dio` client does not pass through the native stack — so a single
/// canonical interceptor reports API latency. This is the ONE place API
/// latency is measured; do not add per-request timers elsewhere.
///
/// Dynamic path segments are normalized to `:id` so metric cardinality stays
/// bounded, and query strings are dropped so no user input reaches the metric.
class PerformanceInterceptor extends Interceptor {
  static const String _metricKey = 'labuda_perf_metric';

  static final RegExp _uuid = RegExp(
    r'^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-'
    r'[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$',
  );
  static final RegExp _numeric = RegExp(r'^\d+$');
  static final RegExp _hex = RegExp(r'^[0-9a-fA-F]{16,}$');

  @override
  void onRequest(
    RequestOptions options,
    RequestInterceptorHandler handler,
  ) {
    final performance = PerformanceMonitoring.instance;
    if (performance != null) {
      try {
        final metric = performance.newHttpMetric(
          _metricUrl(options),
          _httpMethod(options.method),
        );
        options.extra[_metricKey] = metric;
        metric.start();
      } catch (_) {
        // Instrumentation must never break a request.
      }
    }
    handler.next(options);
  }

  @override
  void onResponse(
    Response<dynamic> response,
    ResponseInterceptorHandler handler,
  ) {
    _stop(response.requestOptions, response.statusCode);
    handler.next(response);
  }

  @override
  void onError(DioException err, ErrorInterceptorHandler handler) {
    _stop(err.requestOptions, err.response?.statusCode);
    handler.next(err);
  }

  void _stop(RequestOptions options, int? statusCode) {
    final metric = options.extra[_metricKey];
    if (metric is! HttpMetric) return;
    options.extra.remove(_metricKey);
    try {
      if (statusCode != null) {
        metric.httpResponseCode = statusCode;
      }
      metric.stop();
    } catch (_) {
      // Ignore metric teardown failures.
    }
  }

  String _metricUrl(RequestOptions options) {
    final uri = options.uri;
    final normalizedPath = uri.path
        .split('/')
        .map(_normalizeSegment)
        .join('/');
    if (uri.hasScheme && uri.host.isNotEmpty) {
      return Uri(
        scheme: uri.scheme,
        host: uri.host,
        path: normalizedPath,
      ).toString();
    }
    return normalizedPath.isEmpty ? '/' : normalizedPath;
  }

  String _normalizeSegment(String segment) {
    if (segment.isEmpty) return segment;
    if (_uuid.hasMatch(segment) ||
        _numeric.hasMatch(segment) ||
        _hex.hasMatch(segment)) {
      return ':id';
    }
    return segment;
  }

  HttpMethod _httpMethod(String method) {
    switch (method.toUpperCase()) {
      case 'POST':
        return HttpMethod.Post;
      case 'PUT':
        return HttpMethod.Put;
      case 'PATCH':
        return HttpMethod.Patch;
      case 'DELETE':
        return HttpMethod.Delete;
      default:
        return HttpMethod.Get;
    }
  }
}
