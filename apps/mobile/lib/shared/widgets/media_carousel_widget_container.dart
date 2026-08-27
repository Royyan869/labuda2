import 'package:flutter/material.dart';
import 'package:labuda/shared/ui/base/base_component.dart';

/// Reusable widget component
/// Generated from: lib\shared\widgets\media_carousel_widget_container.dart
class MediaCarouselWidgetContainer extends BaseComponent {
  final String title;
  final VoidCallback? onTap;

  const MediaCarouselWidgetContainer({
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
