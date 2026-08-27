import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_primitives.dart';
import 'package:labuda/domains/user/preference/saved_item/data/providers/saved_item_query_providers.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository_provider.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';

class CommerceSavedItemActionButton extends ConsumerStatefulWidget {
  final String targetType;
  final String targetId;
  final String label;
  final String activeLabel;
  final IconData icon;
  final IconData activeIcon;
  final Color? activeColor;
  final Color? inactiveColor;
  final SavedItemRepository? repository;
  final bool? initialSaved;
  final bool refreshOnInit;
  final VoidCallback? onChanged;
  final bool hideForGuests;

  const CommerceSavedItemActionButton({
    super.key,
    required this.targetType,
    required this.targetId,
    required this.label,
    required this.activeLabel,
    required this.icon,
    required this.activeIcon,
    this.activeColor,
    this.inactiveColor,
    this.repository,
    this.initialSaved,
    this.refreshOnInit = true,
    this.onChanged,
    this.hideForGuests = true,
  });

  @override
  ConsumerState<CommerceSavedItemActionButton> createState() =>
      _CommerceSavedItemActionButtonState();
}

class _CommerceSavedItemActionButtonState
    extends ConsumerState<CommerceSavedItemActionButton> {
  late final SavedItemRepository _repository;
  bool _isSaved = false;
  bool _isLoading = true;
  bool _isToggling = false;

  @override
  void initState() {
    super.initState();
    _repository = widget.repository ?? ref.read(savedItemRepositoryProvider);
    if (widget.initialSaved != null) {
      _isSaved = widget.initialSaved!;
      _isLoading = false;
      if (widget.refreshOnInit) {
        _loadSavedState();
      }
    } else {
      _loadSavedState();
    }
  }

  Future<void> _loadSavedState() async {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) {
      if (!mounted) return;
      setState(() => _isLoading = false);
      return;
    }

    final saved = await _repository.isSaved(
      targetType: widget.targetType,
      targetId: widget.targetId,
    );

    if (!mounted) return;
    setState(() {
      _isSaved = saved;
      _isLoading = false;
    });
  }

  Future<void> _toggleSaved() async {
    if (_isLoading || _isToggling) return;

    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) {
      AppSnackBar.showError(context, 'Silakan masuk untuk menyimpan item');
      return;
    }

    final previousSaved = _isSaved;
    setState(() {
      _isToggling = true;
    });

    try {
      if (previousSaved) {
        await _repository.removeSavedItem(
          targetType: widget.targetType,
          targetId: widget.targetId,
        );
      } else {
        await _repository.addSavedItem(
          targetType: widget.targetType,
          targetId: widget.targetId,
        );
      }
      if (!mounted) return;
      setState(() => _isSaved = !previousSaved);
      ref.invalidate(savedItemsProvider);
      ref.invalidate(savedItemsCountProvider);
      widget.onChanged?.call();
    } catch (_) {
      if (!mounted) return;
      AppSnackBar.showError(
        context,
        previousSaved
            ? 'Gagal menghapus dari shortlist'
            : 'Gagal menyimpan item',
      );
    } finally {
      if (mounted) {
        setState(() => _isToggling = false);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authControllerProvider);
    if (widget.hideForGuests && authState is! AuthStateAuthenticated) {
      return const SizedBox.shrink();
    }

    return CommerceDetailAppBarActionButton(
      icon: widget.icon,
      activeIcon: widget.activeIcon,
      isActive: _isSaved,
      isLoading: _isLoading || _isToggling,
      tooltip: _isSaved ? widget.activeLabel : widget.label,
      semanticsLabel: _isSaved ? widget.activeLabel : widget.label,
      loadingTooltip: _isSaved ? widget.activeLabel : widget.label,
      loadingSemanticsLabel: _isSaved ? widget.activeLabel : widget.label,
      onPressed: _toggleSaved,
      activeColor: widget.activeColor,
      inactiveColor: widget.inactiveColor,
    );
  }
}
