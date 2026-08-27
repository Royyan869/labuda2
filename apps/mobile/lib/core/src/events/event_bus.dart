import 'dart:async';
import 'base_event.dart';

class EventBus {
  static final EventBus _instance = EventBus._internal();
  factory EventBus() => _instance;
  EventBus._internal();

  final StreamController<BaseEvent> _controller =
      StreamController<BaseEvent>.broadcast();

  Stream<BaseEvent> get events => _controller.stream;

  void emit(BaseEvent event) {
    if (!_controller.isClosed) {
      _controller.add(event);
    }
  }

  Stream<T> on<T extends BaseEvent>() {
    return _controller.stream.where((event) => event is T).cast<T>();
  }

  StreamSubscription<T> listen<T extends BaseEvent>(
    void Function(T event) onEvent, {
    Function? onError,
    void Function()? onDone,
    bool? cancelOnError,
  }) {
    return on<T>().listen(
      onEvent,
      onError: onError,
      onDone: onDone,
      cancelOnError: cancelOnError,
    );
  }

  void dispose() {
    _controller.close();
  }
}

class EventSubscription {
  final List<StreamSubscription> _subscriptions = [];

  void add(StreamSubscription subscription) {
    _subscriptions.add(subscription);
  }

  void cancel() {
    for (final subscription in _subscriptions) {
      subscription.cancel();
    }
    _subscriptions.clear();
  }
}

extension EventBusExtension on EventBus {
  EventSubscription subscribeToMultiple(
    List<StreamSubscription> subscriptions,
  ) {
    final eventSubscription = EventSubscription();
    for (final subscription in subscriptions) {
      eventSubscription.add(subscription);
    }
    return eventSubscription;
  }
}
