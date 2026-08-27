import 'package:flutter/material.dart';

/// Base class untuk semua reusable components
/// Enforce consistent API dan behavior across components
abstract class BaseComponent extends StatelessWidget {
  /// Unique identifier untuk component (untuk testing dan debugging)
  final String? componentId;

  /// Whether component ini required atau optional
  final bool isRequired;

  /// Error message kalau validation fail
  final String? errorMessage;

  /// Whether component dalam loading state
  final bool isLoading;

  /// Whether component disabled
  final bool isDisabled;

  const BaseComponent({
    super.key,
    this.componentId,
    this.isRequired = false,
    this.errorMessage,
    this.isLoading = false,
    this.isDisabled = false,
  });

  /// Build component content - implemented by subclasses
  Widget buildContent(BuildContext context);

  /// Validate component data - override jika butuh validation
  String? validateData() => null;

  /// Get component data - override untuk return data
  dynamic getData() => null;

  /// Standard build method dengan wrapper
  @override
  Widget build(BuildContext context) {
    return ComponentWrapper(
      componentId: componentId,
      isRequired: isRequired,
      errorMessage: errorMessage,
      isLoading: isLoading,
      isDisabled: isDisabled,
      child: buildContent(context),
    );
  }
}

/// Wrapper untuk semua components dengan consistent styling
class ComponentWrapper extends StatelessWidget {
  final String? componentId;
  final bool isRequired;
  final String? errorMessage;
  final bool isLoading;
  final bool isDisabled;
  final Widget child;

  const ComponentWrapper({
    super.key,
    required this.child,
    this.componentId,
    this.isRequired = false,
    this.errorMessage,
    this.isLoading = false,
    this.isDisabled = false,
  });

  @override
  Widget build(BuildContext context) {
    Widget wrappedChild = child;

    // Add loading overlay
    if (isLoading) {
      wrappedChild = Stack(
        children: [
          Opacity(opacity: 0.5, child: wrappedChild),
          const Center(child: CircularProgressIndicator()),
        ],
      );
    }

    // Add disabled styling
    if (isDisabled) {
      wrappedChild = Opacity(
        opacity: 0.6,
        child: AbsorbPointer(child: wrappedChild),
      );
    }

    // Add error styling
    if (errorMessage != null) {
      wrappedChild = Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          wrappedChild,
          const SizedBox(height: 4),
          Text(
            errorMessage!,
            style: TextStyle(
              color: Theme.of(context).colorScheme.error,
              fontSize: 12,
            ),
          ),
        ],
      );
    }

    return wrappedChild;
  }
}

/// Interface untuk components yang bisa di-validate
abstract class ValidatableComponent {
  String? validate();
}

/// Interface untuk components yang return data
abstract class DataComponent<T> {
  T? getData();
}

/// Interface untuk components yang bisa di-reset
abstract class ResettableComponent {
  void reset();
}

/// Component size constraints
enum ComponentSize {
  small, // height: 40
  medium, // height: 56
  large, // height: 72
  auto, // fit content
}

/// Component spacing enum
enum ComponentSpacing {
  xxs, // 2.0
  xs, // 4.0
  sm, // 8.0
  md, // 12.0
  lg, // 16.0
  xl, // 20.0
  xxl, // 24.0
}

/// Component spacing helpers
class ComponentSpacingValues {
  static const double XXS = 2.0;
  static const double XS = 4.0;
  static const double SM = 8.0;
  static const double MD = 12.0;
  static const double LG = 16.0;
  static const double XL = 20.0;
  static const double XXL = 24.0;
}
