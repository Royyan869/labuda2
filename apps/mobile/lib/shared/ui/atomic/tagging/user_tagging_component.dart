import 'package:labuda/core/src/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:labuda/shared/ui/base/base_component.dart';

/// Atomic component untuk user tagging dengan autocomplete
/// Single responsibility: Handle user tagging dan mentions
/// MAKSIMAL 100 LINES - ENFORCED BY GUIDELINES
class UserTaggingComponent extends BaseComponent
    implements
        ValidatableComponent,
        DataComponent<List<String>>,
        ResettableComponent {
  final List<String>? initialTaggedUsers;
  final String label;
  final String hint;
  final int maxTags;
  final void Function(List<String>)? onTagsChanged;
  final String? Function(List<String>)? validator;

  const UserTaggingComponent({
    super.key,
    this.initialTaggedUsers,
    required this.label,
    required this.hint,
    this.maxTags = 10,
    this.onTagsChanged,
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
        _buildTagInput(context),
        const SizedBox(height: 8),
        _buildTaggedUsers(context),
      ],
    );
  }

  Widget _buildTagInput(BuildContext context) {
    return TextFormField(
      onChanged: (value) => _handleInputChange(value),
      validator: (value) =>
          validator?.call(getData() ?? []) ??
          _defaultValidator(getData() ?? []),
      enabled: !isDisabled,
      decoration: InputDecoration(
        labelText: isRequired ? '$label *' : label,
        hintText: hint,
        border: const OutlineInputBorder(),
        prefixIcon: const Icon(Icons.alternate_email),
        suffixIcon: isRequired
            ? const Icon(Icons.star, size: 12, color: AppColors.error)
            : null,
      ),
    );
  }

  Widget _buildTaggedUsers(BuildContext context) {
    final taggedUsers = getData() ?? [];
    if (taggedUsers.isEmpty) return const SizedBox.shrink();

    return Wrap(
      spacing: 8,
      runSpacing: 4,
      children: taggedUsers.asMap().entries.map((entry) {
        final index = entry.key;
        final user = entry.value;

        return Chip(
          label: Text('@$user'),
          onDeleted: isDisabled ? null : () => _removeUser(index),
          deleteIcon: const Icon(Icons.close, size: 16),
          backgroundColor: AppColors.neutralGray100,
        );
      }).toList(),
    );
  }

  void _handleInputChange(String value) {
    // Handle @ mention detection
    if (value.endsWith('@')) {
      _showUserSuggestions();
    }

    // Handle space or enter to add user
    if (value.endsWith(' ') && value.contains('@')) {
      final username = value.trim().replaceAll('@', '');
      if (username.isNotEmpty) {
        _addUser(username);
      }
    }
  }

  void _showUserSuggestions() {
    // TODO: Implement user suggestion dropdown
    // This would integrate dengan user search API
  }

  void _addUser(String username) {
    final currentUsers = List<String>.from(getData() ?? []);
    if (!currentUsers.contains(username) && currentUsers.length < maxTags) {
      currentUsers.add(username);
      onTagsChanged?.call(currentUsers);
    }
  }

  void _removeUser(int index) {
    final currentUsers = List<String>.from(getData() ?? []);
    currentUsers.removeAt(index);
    onTagsChanged?.call(currentUsers);
  }

  @override
  String? validate() {
    return validator?.call(getData() ?? []) ??
        _defaultValidator(getData() ?? []);
  }

  @override
  List<String>? getData() {
    return initialTaggedUsers; // In real implementation, this would track current state
  }

  @override
  void reset() {
    onTagsChanged?.call([]);
  }

  String? _defaultValidator(List<String> taggedUsers) {
    if (isRequired && taggedUsers.isEmpty) {
      return 'At least one user tag is required';
    }
    if (taggedUsers.length > maxTags) {
      return 'Maximum $maxTags users can be tagged';
    }
    return null;
  }
}
