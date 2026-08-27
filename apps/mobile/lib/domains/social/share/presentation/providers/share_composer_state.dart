import '../../domain/domain.dart';

/// Canonical state for the unified share composer entry.
///
/// The composer owns the selected target and the currently chosen destination.
/// Optional text is owned by the shared TextEditingController in the share
/// sheet so chat/feed subflows can consume the same text authority.
class ShareComposerState {
  final ShareTarget target;
  final ShareDestinationType? selectedDestination;

  const ShareComposerState({required this.target, this.selectedDestination});

  ShareComposerState copyWith({
    ShareTarget? target,
    ShareDestinationType? selectedDestination,
  }) {
    return ShareComposerState(
      target: target ?? this.target,
      selectedDestination: selectedDestination ?? this.selectedDestination,
    );
  }
}
