import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/domains/system/notification/data/datasources/notification_api_datasource.dart';
import 'package:labuda/domains/system/notification/data/models/api/notification_api_models.dart';

class _RecordingApiClient implements ApiClient {
  String? lastGetPath;
  String? lastPostPath;
  String? lastDeletePath;
  Map<String, dynamic>? lastGetQuery;
  dynamic lastPostData;
  Map<String, dynamic>? lastDeleteQuery;

  dynamic getPayload = <String, dynamic>{'data': <String, dynamic>{}};
  dynamic postPayload = <String, dynamic>{'data': <String, dynamic>{}};

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    lastGetPath = path;
    lastGetQuery = queryParameters;
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      data: getPayload as T,
      statusCode: 200,
    );
  }

  @override
  Future<Response<T>> post<T>(
    String path, {
    data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    lastPostPath = path;
    lastPostData = data;
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      data: postPayload as T,
      statusCode: 200,
    );
  }

  @override
  Future<Response<T>> delete<T>(
    String path, {
    data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    lastDeletePath = path;
    lastDeleteQuery = queryParameters;
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      data: postPayload as T,
      statusCode: 200,
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  group('NotificationApiDatasource contract', () {
    test('listNotifications uses canonical limit/offset', () async {
      final client = _RecordingApiClient()
        ..getPayload = {
          'data': {
            'notifications': <Map<String, dynamic>>[],
            'total_count': 0,
            'unread_count': 0,
            'limit': 20,
            'offset': 20,
          },
        };
      final ds = NotificationApiDatasource(client);

      await ds.listNotifications(page: 2, perPage: 20, unreadOnly: false);

      expect(client.lastGetPath, '/notifications');
      expect(client.lastGetQuery?['limit'], 20);
      expect(client.lastGetQuery?['offset'], 20);
    });

    test('snake_case notification fields parse correctly', () async {
      final client = _RecordingApiClient()
        ..getPayload = {
          'data': {
            'notifications': [
              {
                'id': 'n1',
                'user_id': 'u1',
                'type': 'chat_message',
                'title': 'T',
                'body': 'B',
                'is_read': true,
                'created_at': '2026-05-31T00:00:00Z',
                'data': {'actor_id': 'a1', 'entity_id': 'e1'},
              },
            ],
            'total_count': 1,
            'unread_count': 0,
            'limit': 20,
            'offset': 0,
          },
        };
      final ds = NotificationApiDatasource(client);

      final res = await ds.listNotifications(page: 1, perPage: 20);
      expect(res.notifications.single.userId, 'u1');
      expect(res.notifications.single.isRead, isTrue);
      expect(res.notifications.single.data?['actor_id'], 'a1');
    });

    test('unread-count parses canonical {count}', () async {
      final client = _RecordingApiClient()
        ..getPayload = {
          'data': {'count': 7},
        };
      final ds = NotificationApiDatasource(client);

      final res = await ds.getUnreadCount();
      expect(client.lastGetPath, '/notifications/unread-count');
      expect(res.count, 7);
    });

    test('markAsRead uses /notifications/{id}/read', () async {
      final client = _RecordingApiClient();
      final ds = NotificationApiDatasource(client);

      await ds.markNotificationsAsRead(['notif-1']);
      expect(client.lastPostPath, '/notifications/notif-1/read');
    });

    test(
      'fcm register/delete use canonical /notifications/fcm-token',
      () async {
        final client = _RecordingApiClient()
          ..postPayload = {
            'data': {'token_id': 'token-1'},
          };
        final ds = NotificationApiDatasource(client);

        await ds.registerFCMToken(
          const RegisterFCMTokenRequest(
            token: 'abc',
            platform: 'android',
            deviceId: 'dev1',
          ),
        );
        expect(client.lastPostPath, '/notifications/fcm-token');

        await ds.removeFCMToken('dev1');
        expect(client.lastDeletePath, '/notifications/fcm-token');
        expect(client.lastDeleteQuery?['device_id'], 'dev1');
      },
    );
  });
}
