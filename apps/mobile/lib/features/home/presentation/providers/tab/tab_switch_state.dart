/// Tab switch state untuk navigation
class TabSwitchState {
  final String? targetTab;
  final int? subTabIndex;
  final bool isPending;

  const TabSwitchState({
    this.targetTab,
    this.subTabIndex,
    this.isPending = false,
  });

  TabSwitchState copyWith({
    String? targetTab,
    int? subTabIndex,
    bool? isPending,
  }) {
    return TabSwitchState(
      targetTab: targetTab ?? this.targetTab,
      subTabIndex: subTabIndex ?? this.subTabIndex,
      isPending: isPending ?? this.isPending,
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is TabSwitchState &&
          other.targetTab == targetTab &&
          other.subTabIndex == subTabIndex &&
          other.isPending == isPending;

  @override
  int get hashCode => Object.hash(targetTab, subTabIndex, isPending);
}
