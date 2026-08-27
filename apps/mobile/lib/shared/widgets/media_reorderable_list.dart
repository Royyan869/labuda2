import 'dart:io';
import 'package:flutter/material.dart';
import 'package:labuda/shared/widgets/media_image_item.dart';

class MediaReorderableList extends StatelessWidget {
  final List<File> images;
  final void Function(int oldIndex, int newIndex)? onReorder;
  final void Function(int index)? onRemove;
  final bool showCoverBadge;
  final double height;
  final double width;

  const MediaReorderableList({
    super.key,
    required this.images,
    this.onReorder,
    this.onRemove,
    required this.showCoverBadge,
    required this.height,
    required this.width,
  });

  @override
  Widget build(BuildContext context) {
    return ReorderableListView.builder(
      scrollDirection: Axis.horizontal,
      clipBehavior: Clip.none, // Agar shadow/border tidak terpotong
      itemCount: images.length,
      onReorder: onReorder ?? (oldIndex, newIndex) {},
      itemBuilder: (context, index) {
        return MediaImageItem(
          key: ValueKey('image_$index'),
          image: images[index],
          index: index,
          onRemove: onRemove != null ? () => onRemove!(index) : null,
          showCoverBadge: showCoverBadge,
          height: height,
          width: width,
        );
      },
    );
  }
}
