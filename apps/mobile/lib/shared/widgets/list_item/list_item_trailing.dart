import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'list_item_types.dart';

/// Trailing widget configuration interface
abstract class ListItemTrailing {
  TrailingType get type;
  bool? get toggleValue;
  ValueChanged<bool>? get onToggleChanged;
  String? get text;
  String? get badgeText;
  Color? get badgeColor;
  Color? get textColor;
  String? get buttonText;
  VoidCallback? get onButtonPressed;
  Widget? get custom;
}

/// Factory constructor for ListItemTrailing
extension ListItemTrailingFactory on ListItemTrailing {
  static ListItemTrailing chevron() => const _ChevronTrailing();
  static ListItemTrailing arrow() => const _ArrowTrailing();
  static ListItemTrailing toggle({
    required bool value,
    required ValueChanged<bool> onChanged,
  }) => _ToggleTrailing(value: value, onChanged: onChanged);
  static ListItemTrailing badge({required String text, Color? color}) =>
      _BadgeTrailing(text: text, color: color);
  static ListItemTrailing text({required String text, Color? color}) =>
      _TextTrailing(text: text, color: color);
  static ListItemTrailing button({
    required String text,
    required VoidCallback onPressed,
  }) => _ButtonTrailing(text: text, onPressed: onPressed);
  static ListItemTrailing custom(Widget widget) => _CustomTrailing(widget);
  static ListItemTrailing none() => const _NoneTrailing();
}

class _ChevronTrailing implements ListItemTrailing {
  const _ChevronTrailing();

  @override
  TrailingType get type => TrailingType.chevron;
  @override
  bool? get toggleValue => null;
  @override
  ValueChanged<bool>? get onToggleChanged => null;
  @override
  String? get text => null;
  @override
  String? get badgeText => null;
  @override
  Color? get badgeColor => null;
  @override
  Color? get textColor => null;
  @override
  String? get buttonText => null;
  @override
  VoidCallback? get onButtonPressed => null;
  @override
  Widget? get custom => null;
}

class _ArrowTrailing implements ListItemTrailing {
  const _ArrowTrailing();

  @override
  TrailingType get type => TrailingType.arrow;
  @override
  bool? get toggleValue => null;
  @override
  ValueChanged<bool>? get onToggleChanged => null;
  @override
  String? get text => null;
  @override
  String? get badgeText => null;
  @override
  Color? get badgeColor => null;
  @override
  Color? get textColor => null;
  @override
  String? get buttonText => null;
  @override
  VoidCallback? get onButtonPressed => null;
  @override
  Widget? get custom => null;
}

class _ToggleTrailing implements ListItemTrailing {
  final bool value;
  final ValueChanged<bool> onChanged;

  const _ToggleTrailing({required this.value, required this.onChanged});

  @override
  TrailingType get type => TrailingType.toggle;
  @override
  bool get toggleValue => value;
  @override
  ValueChanged<bool> get onToggleChanged => onChanged;
  @override
  String? get text => null;
  @override
  String? get badgeText => null;
  @override
  Color? get badgeColor => null;
  @override
  Color? get textColor => null;
  @override
  String? get buttonText => null;
  @override
  VoidCallback? get onButtonPressed => null;
  @override
  Widget? get custom => null;
}

class _BadgeTrailing implements ListItemTrailing {
  final String _badgeTextValue;
  final Color? color;

  const _BadgeTrailing({required String text, this.color})
    : _badgeTextValue = text;

  @override
  TrailingType get type => TrailingType.badge;
  @override
  bool? get toggleValue => null;
  @override
  ValueChanged<bool>? get onToggleChanged => null;
  @override
  String? get text => null;
  @override
  String get badgeText => _badgeTextValue;
  @override
  Color? get badgeColor => color;
  @override
  Color? get textColor => null;
  @override
  String? get buttonText => null;
  @override
  VoidCallback? get onButtonPressed => null;
  @override
  Widget? get custom => null;
}

class _TextTrailing implements ListItemTrailing {
  final String _textValue;
  final Color? color;

  const _TextTrailing({required String text, this.color}) : _textValue = text;

  @override
  TrailingType get type => TrailingType.text;
  @override
  bool? get toggleValue => null;
  @override
  ValueChanged<bool>? get onToggleChanged => null;
  @override
  String get text => _textValue;
  @override
  String? get badgeText => null;
  @override
  Color? get badgeColor => null;
  @override
  Color? get textColor => color;
  @override
  String? get buttonText => null;
  @override
  VoidCallback? get onButtonPressed => null;
  @override
  Widget? get custom => null;
}

class _ButtonTrailing implements ListItemTrailing {
  final String _buttonTextValue;
  final VoidCallback onPressed;

  const _ButtonTrailing({required String text, required this.onPressed})
    : _buttonTextValue = text;

  @override
  TrailingType get type => TrailingType.button;
  @override
  bool? get toggleValue => null;
  @override
  ValueChanged<bool>? get onToggleChanged => null;
  @override
  String? get text => null;
  @override
  String? get badgeText => null;
  @override
  Color? get badgeColor => null;
  @override
  Color? get textColor => null;
  @override
  String get buttonText => _buttonTextValue;
  @override
  VoidCallback get onButtonPressed => onPressed;
  @override
  Widget? get custom => null;
}

class _CustomTrailing implements ListItemTrailing {
  final Widget widget;

  _CustomTrailing(this.widget);

  @override
  TrailingType get type => TrailingType.custom;
  @override
  bool? get toggleValue => null;
  @override
  ValueChanged<bool>? get onToggleChanged => null;
  @override
  String? get text => null;
  @override
  String? get badgeText => null;
  @override
  Color? get badgeColor => null;
  @override
  Color? get textColor => null;
  @override
  String? get buttonText => null;
  @override
  VoidCallback? get onButtonPressed => null;
  @override
  Widget get custom => widget;
}

class _NoneTrailing implements ListItemTrailing {
  const _NoneTrailing();

  @override
  TrailingType get type => TrailingType.none;
  @override
  bool? get toggleValue => null;
  @override
  ValueChanged<bool>? get onToggleChanged => null;
  @override
  String? get text => null;
  @override
  String? get badgeText => null;
  @override
  Color? get badgeColor => null;
  @override
  Color? get textColor => null;
  @override
  String? get buttonText => null;
  @override
  VoidCallback? get onButtonPressed => null;
  @override
  Widget? get custom => null;
}

/// Build widget from ListItemTrailing configuration
Widget? buildListItemTrailing(ListItemTrailing config, bool isDark) {
  switch (config.type) {
    case TrailingType.chevron:
      return Icon(
        Icons.chevron_right,
        color: isDark ? AppColors.neutralGray500 : AppColors.neutralGray400,
      );

    case TrailingType.arrow:
      return Icon(
        Icons.arrow_forward_ios,
        size: 16,
        color: isDark ? AppColors.neutralGray500 : AppColors.neutralGray400,
      );

    case TrailingType.toggle:
      return Switch(
        value: config.toggleValue ?? false,
        onChanged: config.onToggleChanged,
        materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
      );

    case TrailingType.badge:
      return Container(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
        decoration: BoxDecoration(
          color: config.badgeColor ?? AppColors.primaryRed,
          borderRadius: BorderRadius.circular(12),
        ),
        child: Text(
          config.badgeText ?? '',
          style: AppTypography.labelSmall.copyWith(
            color: Colors.white,
            fontWeight: FontWeight.w600,
          ),
        ),
      );

    case TrailingType.text:
      return Text(
        config.text ?? '',
        style: AppTypography.bodyMedium.copyWith(
          color:
              config.textColor ??
              (isDark ? AppColors.neutralWhite : AppColors.neutralGray900),
          fontWeight: FontWeight.w600,
        ),
      );

    case TrailingType.button:
      return SizedBox(
        height: 32,
        child: TextButton(
          onPressed: config.onButtonPressed,
          style: TextButton.styleFrom(
            padding: const EdgeInsets.symmetric(horizontal: 12),
            backgroundColor: AppColors.primaryRed,
            foregroundColor: Colors.white,
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(6),
            ),
          ),
          child: Text(
            config.buttonText ?? '',
            style: AppTypography.labelSmall.copyWith(
              fontWeight: FontWeight.w600,
            ),
          ),
        ),
      );

    case TrailingType.custom:
      return config.custom;

    case TrailingType.none:
      return null;
  }
}
