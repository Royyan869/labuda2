import 'package:flutter/material.dart';
import 'package:labuda/shared/ui/base/base_component.dart';

/// Atomic component untuk visibility settings (public/private/friends)
/// Single responsibility: Handle post visibility selection
/// MAKSIMAL 100 LINES - ENFORCED BY GUIDELINES
class VisibilitySettingsComponent extends BaseComponent
    implements
        ValidatableComponent,
        DataComponent<String>,
        ResettableComponent {
  final String initialVisibility;
  final String label;
  final List<String> availableOptions;
  final bool showDescription;
  final void Function(String)? onVisibilityChanged;
  final String? Function(String?)? validator;

  const VisibilitySettingsComponent({
    super.key,
    required this.initialVisibility,
    required this.label,
    this.availableOptions = const ['Public', 'Private', 'Friends'],
    this.showDescription = true,
    this.onVisibilityChanged,
    this.validator,
    super.componentId,
    super.isRequired,
    super.errorMessage,
    super.isLoading,
    super.isDisabled,
  });

  @override
  Widget buildContent(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          isRequired ? '$label *' : label,
          style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w500),
        ),
        const SizedBox(height: 8),
        RadioGroup<String>(
          groupValue: initialVisibility,
          onChanged: isDisabled
              ? (_) {}
              : (String? value) {
                  if (value != null) {
                    onVisibilityChanged?.call(value);
                  }
                },
          child: Column(
            children: availableOptions
                .map((option) => _buildOptionTile(context, option))
                .toList(),
          ),
        ),
      ],
    );
  }

  Widget _buildOptionTile(BuildContext context, String option) {
    final isSelected = option == initialVisibility;

    return RadioListTile<String>(
      title: Text(option),
      subtitle: showDescription ? Text(_getDescription(option)) : null,
      value: option,
      selected: isSelected,
      dense: true,
      contentPadding: EdgeInsets.zero,
    );
  }

  String _getDescription(String option) {
    switch (option.toLowerCase()) {
      case 'public':
        return 'Everyone can see this post';
      case 'private':
        return 'Only you can see this post';
      case 'friends':
        return 'Only your friends can see this post';
      case 'followers':
        return 'Only your followers can see this post';
      default:
        return 'Custom visibility setting';
    }
  }

  @override
  String? validate() {
    return validator?.call(getData()) ?? _defaultValidator(getData());
  }

  @override
  String? getData() {
    return initialVisibility;
  }

  @override
  void reset() {
    onVisibilityChanged?.call(availableOptions.first);
  }

  String? _defaultValidator(String? value) {
    if (isRequired && (value?.isEmpty ?? true)) {
      return 'Visibility setting is required';
    }
    if (value != null && !availableOptions.contains(value)) {
      return 'Invalid visibility option';
    }
    return null;
  }
}
