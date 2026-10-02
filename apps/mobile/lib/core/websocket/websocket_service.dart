import 'dart:async';
import 'dart:convert';
import 'dart:developer' as developer;

import 'package:web_socket_channel/io.dart';
import 'package:web_socket_channel/web_socket_channel.dart';
import 'package:web_socket_channel/status.dart' as status;

import 'websocket_message.dart';

enum ConnectionState { disconnected, connecting, connected, reconnecting }

class WebSocketService {
  WebSocketChannel? _channel;
  StreamController<WebSocketMessage>? _messageController;
  StreamController<ConnectionState>? _stateController;
  Timer? _reconnectTimer;
  Timer? _pingTimer;

  final String baseUrl;
  bool _isConnecting = false;
  int _reconnectAttempts = 0;
  ConnectionState _state = ConnectionState.disconnected;

  // Phase 5: Labuda canonical — token provider and connect generation for race safety.
  Future<String?> Function()? _labudaTokenProvider;
  int _connectGeneration = 0;

  // Room subscriptions
  final Set<String> _subscribedRooms = {};

  // Message acknowledgment
  final Map<String, Completer<void>> _pendingAcks = {};

  // REALTIME RESUME CONTRACT: reconnect NEVER gives up while the session is
  // alive. A backgrounded app exhausts any finite attempt budget (timers
  // suspended, network flapping), the socket then stays dead until the next
  // login — exactly the backend's "No connections for user" when a
  // chat.room.updated event has nowhere to land. Backoff stays capped so
  // recovery remains prompt.
  static const Duration reconnectDelay = Duration(seconds: 5);
  static const Duration maxReconnectDelay = Duration(seconds: 30);
  static const Duration ackTimeout = Duration(seconds: 10);
  static const Duration pingInterval = Duration(seconds: 30);

  WebSocketService({required this.baseUrl});

  Stream<WebSocketMessage> get messages =>
      _messageController?.stream ?? const Stream.empty();

  Stream<ConnectionState> get connectionState =>
      _stateController?.stream ?? const Stream.empty();

  bool get isConnected => _state == ConnectionState.connected;
  ConnectionState get state => _state;

  void setLabudaTokenProvider(Future<String?> Function()? provider) {
    _labudaTokenProvider = provider;
  }

  // Test-only accessor for provider
  Future<String?> Function()? get labudaTokenProviderForTest => _labudaTokenProvider;
  int get connectGenerationForTest => _connectGeneration;

  Future<void> connect(String authToken) async {
    if (_isConnecting || isConnected) return;

    final int generation = ++_connectGeneration;
    _isConnecting = true;
    _updateState(ConnectionState.connecting);

    try {
      final wsUri = Uri.parse(baseUrl);
      developer.log(
        'Connecting to WebSocket: $wsUri',
        name: 'WebSocketService',
      );

      // Auth via Authorization header — never embed token in the URL.
      // Backend LabudaAuthMiddleware reads only the Authorization header;
      // a ?token= query param would be silently ignored → 401.
      _channel = IOWebSocketChannel.connect(
        wsUri,
        headers: {'Authorization': 'Bearer $authToken'},
      );

      _messageController = StreamController<WebSocketMessage>.broadcast();
      _stateController = StreamController<ConnectionState>.broadcast();

      _channel!.stream.listen(
        _handleMessage,
        onError: _handleError,
        onDone: _handleDisconnect,
      );

      // Await the ready future so handshake failures (e.g. 401) are caught
      // here rather than leaking as uncaught async errors.
      await _channel!.ready;

      // Phase 5 race: if disconnect/logout happened while handshake was in flight,
      // the generation will have changed — abort stale handshake before marking connected.
      if (generation != _connectGeneration) {
        developer.log('WebSocket handshake stale (generation $generation != $_connectGeneration) — aborting', name: 'WebSocketService');
        try { await _channel?.sink.close(status.goingAway); } catch (_) {}
        _channel = null;
        _isConnecting = false;
        return;
      }

      _isConnecting = false;
      _reconnectAttempts = 0;

      // Tier 4 (Runtime Honesty): flip the INTERNAL state to connected
      // so `send()` can execute during the resubscribe pass (the send
      // path guards on `isConnected`), but DO NOT broadcast `connected`
      // to outside listeners yet. Subscribers must not observe
      // "connected" until the channel is actually usable (i.e. all
      // pre-existing room subscriptions have been re-established with
      // the server). Broadcast moves to AFTER `_resubscribeRooms`.
      _state = ConnectionState.connected;

      developer.log(
        'WebSocket handshake complete — running resubscribe before '
        'broadcasting connected state',
        name: 'WebSocketService',
      );

      // Start ping timer
      _startPingTimer();

      // Re-subscribe to rooms after reconnect. Failures inside this
      // call DO NOT throw out (each room is independently try/catched
      // and on failure removed from _subscribedRooms — see method).
      await _resubscribeRooms();

      // Only NOW announce "connected" to the world. Listeners that
      // resubscribe on this transition (e.g. ChatRepositoryImpl) act
      // as a second-pass safety net for rooms that were removed by
      // resubscribe failures above.
      _stateController?.add(ConnectionState.connected);

      developer.log(
        'WebSocket fully ready (broadcast: connected)',
        name: 'WebSocketService',
      );
    } catch (e) {
      developer.log(
        'Failed to connect WebSocket: $e',
        name: 'WebSocketService',
        error: e,
      );
      _isConnecting = false;
      _updateState(ConnectionState.disconnected);
      _scheduleReconnect();
    }
  }

  void _handleMessage(dynamic message) {
    try {
      final data = jsonDecode(message as String) as Map<String, dynamic>;
      final wsMessage = WebSocketMessage.fromJson(data);

      // Handle acknowledgments
      if (wsMessage.type == MessageType.ack) {
        _handleAck(wsMessage);
        return;
      }

      // Handle errors
      if (wsMessage.type == MessageType.error) {
        developer.log(
          'Server error: ${wsMessage.data}',
          name: 'WebSocketService',
        );
      }

      // Handle pong
      if (wsMessage.type == MessageType.pong) {
        // Just keep connection alive
        return;
      }

      _messageController?.add(wsMessage);
      _reconnectAttempts = 0;
    } catch (e, stackTrace) {
      // Tier 4 (Runtime Honesty): parse errors used to be `developer.log`
      // only — listeners had no way to know an incoming frame was
      // malformed. Emit the error on the message stream so subscribers'
      // onError handlers can react. The stream itself stays open (we
      // do NOT close the controller), so subsequent valid frames still
      // flow through.
      developer.log(
        'Error parsing WebSocket message: $e',
        name: 'WebSocketService',
        error: e,
      );
      _messageController?.addError(
        FormatException('WebSocket frame parse failure: $e'),
        stackTrace,
      );
    }
  }

  void _handleError(Object error) {
    developer.log(
      'WebSocket error: $error',
      name: 'WebSocketService',
      error: error,
    );
    _handleDisconnect();
  }

  void _handleDisconnect() {
    developer.log('WebSocket connection closed', name: 'WebSocketService');
    _channel = null;
    _pingTimer?.cancel();
    _updateState(ConnectionState.disconnected);
    _scheduleReconnect();
  }

  void _updateState(ConnectionState newState) {
    _state = newState;
    _stateController?.add(newState);
  }

  void _scheduleReconnect() {
    // Phase 5: do not reconnect if Labuda credential has been cleared (logout).
    // Canonical reconnect uses Labuda token provider (Labuda storage). No _authToken fallback,
    // no Firebase fallback, no stale pre-logout reuse.
    final provider = _labudaTokenProvider;
    if (provider == null) {
      developer.log(
        'WebSocket reconnect stopped — no Labuda token provider (no _authToken fallback)',
        name: 'WebSocketService',
      );
      _updateState(ConnectionState.disconnected);
      return;
    }

    _updateState(ConnectionState.reconnecting);
    _reconnectTimer?.cancel();
    _reconnectAttempts++;
    final delay = _reconnectDelayFor(_reconnectAttempts);
    _reconnectTimer = Timer(delay, () async {
      developer.log(
        'Reconnect attempt $_reconnectAttempts (delay ${delay.inSeconds}s)',
        name: 'WebSocketService',
      );
      String? fresh;
      try {
        fresh = await provider();
        fresh = fresh?.trim();
      } catch (e) {
        developer.log(
          'WebSocket token provider error: $e',
          name: 'WebSocketService',
        );
        _scheduleReconnect();
        return;
      }
      if (fresh == null || fresh.isEmpty) {
        // TRANSIENT (token refresh in flight) — never a terminal state.
        // Returning silently here is what used to leave the socket dead.
        developer.log(
          'WebSocket reconnect deferred — Labuda access missing (retrying)',
          name: 'WebSocketService',
        );
        _scheduleReconnect();
        return;
      }
      // Use fresh Labuda JWT
      connect(fresh);
    });
  }

  /// 5s → 10s → 20s → capped 30s. Never terminal: retries stop only on a
  /// successful connect (attempts reset) or logout (no token provider).
  Duration _reconnectDelayFor(int attempt) {
    if (attempt <= 1) return reconnectDelay;
    final exponent = attempt - 1 > 2 ? 2 : attempt - 1;
    final backoff = reconnectDelay * (1 << exponent);
    return backoff > maxReconnectDelay ? maxReconnectDelay : backoff;
  }

  /// Foreground/resume entry point: drop any pending backoff and reconnect
  /// NOW using the installed Labuda token provider. Wired to the
  /// app-lifecycle observer so a socket killed while backgrounded recovers
  /// on resume instead of waiting for the next login.
  void reconnectNow() {
    if (_isConnecting || isConnected) return;
    final provider = _labudaTokenProvider;
    if (provider == null) return;
    _reconnectTimer?.cancel();
    _reconnectAttempts = 0;
    _updateState(ConnectionState.reconnecting);
    Future<void>(() async {
      try {
        final token = (await provider())?.trim();
        if (token == null || token.isEmpty) {
          _scheduleReconnect();
          return;
        }
        await connect(token);
      } catch (e) {
        developer.log(
          'WebSocket resume reconnect failed: $e',
          name: 'WebSocketService',
        );
        _scheduleReconnect();
      }
    });
  }

  void _startPingTimer() {
    _pingTimer?.cancel();
    _pingTimer = Timer.periodic(pingInterval, (timer) {
      if (isConnected) {
        final pingMessage = WebSocketMessage(
          type: MessageType.ping,
          from: '',
          data: {},
        );
        _sendMessage(pingMessage);
      }
    });
  }

  Future<void> send(WebSocketMessage message, {bool requireAck = false}) async {
    if (!isConnected) {
      throw Exception('Cannot send message: WebSocket not connected');
    }

    if (requireAck) {
      message.requireAck = true;
      final completer = Completer<void>();
      _pendingAcks[message.id] = completer;

      // Timeout for ACK
      Timer(ackTimeout, () {
        if (!completer.isCompleted) {
          completer.completeError('ACK timeout');
          _pendingAcks.remove(message.id);
        }
      });

      _sendMessage(message);
      return completer.future;
    } else {
      _sendMessage(message);
    }
  }

  void _sendMessage(WebSocketMessage message) {
    _channel?.sink.add(jsonEncode(message.toJson()));
  }

  void _handleAck(WebSocketMessage ackMessage) {
    final messageId = ackMessage.data['message_id'] as String?;
    if (messageId != null && _pendingAcks.containsKey(messageId)) {
      _pendingAcks[messageId]?.complete();
      _pendingAcks.remove(messageId);
    }
  }

  Future<void> subscribeToRoom(String roomId) async {
    if (_subscribedRooms.contains(roomId)) return;

    final message = WebSocketMessage(
      type: MessageType.subscribe,
      from: '',
      data: {'room_id': roomId},
    );

    await send(message, requireAck: true);
    _subscribedRooms.add(roomId);
    developer.log('Subscribed to room: $roomId', name: 'WebSocketService');
  }

  Future<void> unsubscribeFromRoom(String roomId) async {
    if (!_subscribedRooms.contains(roomId)) return;

    final message = WebSocketMessage(
      type: MessageType.unsubscribe,
      from: '',
      data: {'room_id': roomId},
    );

    await send(message, requireAck: true);
    _subscribedRooms.remove(roomId);
    developer.log('Unsubscribed from room: $roomId', name: 'WebSocketService');
  }

  Future<void> _resubscribeRooms() async {
    // Tier 4 (Runtime Honesty): iterate a snapshot so we can mutate
    // `_subscribedRooms` in the catch branch without ConcurrentModification.
    for (final roomId in _subscribedRooms.toList()) {
      try {
        final message = WebSocketMessage(
          type: MessageType.subscribe,
          from: '',
          data: {'room_id': roomId},
        );
        // Require ACK so a server-side failure to honor the subscribe
        // (or a 10s timeout) surfaces as an exception instead of a
        // silent "send succeeded but server didn't actually subscribe".
        await send(message, requireAck: true);
      } catch (e, stackTrace) {
        // Honest failure: REMOVE the room from `_subscribedRooms` so
        // subscription state no longer lies about being subscribed.
        // The chat-repo layer keeps the high-level intent in its own
        // `_subscribedChatRooms` set and re-issues `subscribeToRoom`
        // on the next `connected` broadcast — which acts as a second
        // pass that will re-attempt the failed rooms.
        _subscribedRooms.remove(roomId);
        developer.log(
          'Failed to resubscribe to room $roomId — removed from '
          'subscribed set, will re-attempt on next connected broadcast: $e',
          name: 'WebSocketService',
          error: e,
          stackTrace: stackTrace,
        );
      }
    }
  }

  /// Test-only accessor for asserting subscription-set state after
  /// driving the connect / resubscribe path in unit tests.
  Set<String> get subscribedRoomsForTest =>
      Set<String>.unmodifiable(_subscribedRooms);

  /// Test-only seam to populate the subscription set before driving
  /// the resubscribe path in unit tests.
  void primeSubscribedRoomsForTest(Iterable<String> rooms) {
    _subscribedRooms
      ..clear()
      ..addAll(rooms);
  }

  /// Test-only seam to drive the resubscribe pass in isolation. The
  /// caller must have already populated `_subscribedRooms` (e.g. via
  /// [primeSubscribedRoomsForTest]) and set `_state` to connected.
  Future<void> resubscribeRoomsForTest() => _resubscribeRooms();

  /// Test-only seam to mark the channel as connected (without opening
  /// a real WebSocket). Use [setChannelForTest] in conjunction so
  /// `send()` has a sink to write to.
  void markConnectedForTest() {
    _state = ConnectionState.connected;
  }

  /// Test-only seam to inject a fake sink so `send()` does not throw
  /// on a null channel when the resubscribe pass runs.
  void setChannelForTest(WebSocketChannel channel) {
    _channel = channel;
  }

  /// Test-only seam to construct the message controller without
  /// opening a real WebSocket. Required before driving
  /// [handleMessageForTest] so the controller can `addError` on parse
  /// failures.
  void primeMessageControllerForTest() {
    _messageController?.close();
    _messageController = StreamController<WebSocketMessage>.broadcast();
  }

  /// Test-only seam to drive [_handleMessage] directly with a raw
  /// frame payload. Used to verify that parse failures emit a stream
  /// error rather than being silently swallowed.
  void handleMessageForTest(dynamic frame) => _handleMessage(frame);

  Future<void> disconnect() async {
    // Phase 5: bump generation to invalidate any in-flight handshake and
    // prevent stale reconnect (reconnect resolves fresh tokens via the
    // Labuda token provider — there is no stored-token fallback).
    _connectGeneration++;
    _isConnecting = false;
    _reconnectTimer?.cancel();
    _pingTimer?.cancel();
    _subscribedRooms.clear();
    _pendingAcks.clear();

    await _channel?.sink.close(status.goingAway);
    _channel = null;

    await _messageController?.close();
    _messageController = null;

    await _stateController?.close();
    _stateController = null;

    _reconnectAttempts = 0;
    // _state already disconnected but ensure controller handles closed state gracefully.
    try {
      _updateState(ConnectionState.disconnected);
    } catch (_) {}
    developer.log('WebSocket disconnected', name: 'WebSocketService');
  }

}
