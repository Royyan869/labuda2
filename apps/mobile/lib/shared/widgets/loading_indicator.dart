import 'package:flutter/material.dart';

import 'package:labuda/core/core.dart';

enum LoadingSize { small, medium, large }

class LoadingIndicator extends StatelessWidget {
  final LoadingSize size;
  final Color? color;
  final String? message;

  const LoadingIndicator({
    super.key,
    this.size = LoadingSize.medium,
    this.color,
    this.message,
  });

  const LoadingIndicator.small({super.key, this.color, this.message})
    : size = LoadingSize.small;

  const LoadingIndicator.large({super.key, this.color, this.message})
    : size = LoadingSize.large;

  @override
  Widget build(BuildContext context) {
    final indicatorColor = color ?? context.colorScheme.primary;

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        SizedBox(
          width: _getSize(),
          height: _getSize(),
          child: CircularProgressIndicator(
            valueColor: AlwaysStoppedAnimation<Color>(indicatorColor),
            strokeWidth: _getStrokeWidth(),
          ),
        ),
        if (message != null) ...[
          const SizedBox(height: 16),
          Text(
            message!,
            style: AppTypography.bodyMedium.copyWith(
              color: context.colorScheme.onSurface.withValues(alpha: 0.7),
            ),
            textAlign: TextAlign.center,
          ),
        ],
      ],
    );
  }

  double _getSize() {
    return switch (size) {
      LoadingSize.small => 20,
      LoadingSize.medium => 32,
      LoadingSize.large => 48,
    };
  }

  double _getStrokeWidth() {
    return switch (size) {
      LoadingSize.small => 2,
      LoadingSize.medium => 3,
      LoadingSize.large => 4,
    };
  }
}

class FullScreenLoading extends StatelessWidget {
  final String? message;
  final bool barrierDismissible;

  const FullScreenLoading({
    super.key,
    this.message,
    this.barrierDismissible = false,
  });

  @override
  Widget build(BuildContext context) {
    return PopScope(
      canPop: barrierDismissible,
      child: Container(
        color: AppColors.dark.withValues(alpha: 0.5),
        child: Center(
          child: Card(
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: LoadingIndicator.large(message: message ?? 'Loading...'),
            ),
          ),
        ),
      ),
    );
  }

  static void show(BuildContext context, {String? message}) {
    showDialog(
      context: context,
      barrierDismissible: false,
      builder: (context) => FullScreenLoading(message: message),
    );
  }

  static void hide(BuildContext context) {
    if (Navigator.of(context).canPop()) {
      Navigator.of(context).pop();
    }
  }
}
