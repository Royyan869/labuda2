import 'package:flutter/material.dart';
import 'package:labuda/shared/ui/base/base_component.dart';

/// Reusable widget component
/// Generated from: lib\shared\widgets\media_viewer_widget_container.dart
class MediaViewerWidgetContainer extends BaseComponent {
  final String title;
  final VoidCallback? onTap;

  const MediaViewerWidgetContainer({
    super.key,
    required this.title,
    this.onTap,
  });

  @override
  Widget buildContent(BuildContext context) {
    // TODO: Implement repeated widget pattern
    // Container used 7 times

    return ListTile(title: Text(title), onTap: onTap);
  }
}
