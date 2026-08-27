/// Seller Shipping Options Selector
///
/// Reusable multi-select section embedded in create_listing_screen,
/// edit_listing_screen, and create_auction_screen. Loads the seller's
/// **active** global shipping options via the canonical shipping repository
/// and lets the seller tick which ones apply to this sale surface.
///
/// The selector does NOT call the persistence endpoint itself - the owning
/// screen decides when to persist. The selector exposes its current selection
/// through [onSelectionChanged] and renders three honest states:
///   - loading (showing a spinner)
///   - empty (seller has zero active options -> CTA to /seller/shipping)
///   - populated (chips for each active option, multi-select)
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart' hide ConnectionState;
import 'package:labuda/domains/commerce/transaction/shipping/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart';

class SellerShippingOptionsSelector extends ConsumerStatefulWidget {
  /// Initial selection (used by edit flow when the screen knows the prior IDs;
  /// the create flow passes `const []`).
  final List<String> initialSelectedIds;

  /// Called whenever the selection changes. The full list of selected
  /// option IDs is passed each time.
  final ValueChanged<List<String>> onSelectionChanged;

  /// Optional hint text rendered under the section title.
  final String? helperText;

  const SellerShippingOptionsSelector({
    super.key,
    this.initialSelectedIds = const [],
    required this.onSelectionChanged,
    this.helperText,
  });

  @override
  ConsumerState<SellerShippingOptionsSelector> createState() =>
      _SellerShippingOptionsSelectorState();
}

class _SellerShippingOptionsSelectorState
    extends ConsumerState<SellerShippingOptionsSelector> {
  late Set<String> _selected;
  late Future<List<ShippingOption>> _future;

  @override
  void initState() {
    super.initState();
    _selected = widget.initialSelectedIds.toSet();
    _future = _loadOptions();
  }

  Future<List<ShippingOption>> _loadOptions() async {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) {
      return const [];
    }
    final repo = ref.read(shippingRepositoryProvider);
    final result = await repo.listMyActiveShippingOptions();
    if (result.isError) {
      throw Exception(result.error ?? 'Failed to load shipping options');
    }
    return result.data ?? const [];
  }

  void _toggle(String id) {
    setState(() {
      if (_selected.contains(id)) {
        _selected.remove(id);
      } else {
        _selected.add(id);
      }
    });
    widget.onSelectionChanged(_selected.toList(growable: false));
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<List<ShippingOption>>(
      future: _future,
      builder: (context, snap) {
        if (snap.connectionState != ConnectionState.done) {
          return const _LoadingPlaceholder();
        }
        if (snap.hasError) {
          return _ErrorPlaceholder(
            message: snap.error.toString(),
            onRetry: () {
              setState(() => _future = _loadOptions());
            },
          );
        }
        final options = snap.data ?? const [];
        if (options.isEmpty) {
          return const _EmptyOptionsBanner();
        }
        return _populated(options);
      },
    );
  }

  Widget _populated(List<ShippingOption> options) {
    final hasSelection = _selected.isNotEmpty;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (widget.helperText != null)
          Padding(
            padding: const EdgeInsets.only(bottom: 12),
            child: Text(
              widget.helperText!,
              style: TextStyle(fontSize: 13, color: AppColors.neutralGray600),
            ),
          ),
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: options.map((opt) {
            final selected = _selected.contains(opt.id);
            return FilterChip(
              label: Text('${opt.emoji}  ${opt.shortName}'),
              selected: selected,
              onSelected: (_) => _toggle(opt.id),
              selectedColor: AppColors.primaryRed.withValues(alpha: 0.15),
              checkmarkColor: AppColors.primaryRed,
            );
          }).toList(),
        ),
        if (!hasSelection) ...[
          const SizedBox(height: 8),
          Text(
            'Pilih minimal 1 opsi pengiriman agar listing bisa dipublish.',
            style: TextStyle(
              fontSize: 12,
              color: AppColors.statusWarning,
              fontStyle: FontStyle.italic,
            ),
          ),
        ],
      ],
    );
  }
}

// =============================================================================
// PLACEHOLDERS
// =============================================================================

class _LoadingPlaceholder extends StatelessWidget {
  const _LoadingPlaceholder();

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(vertical: 16, horizontal: 12),
      decoration: BoxDecoration(
        color: AppColors.neutralGray100,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          const SizedBox(
            width: 18,
            height: 18,
            child: CircularProgressIndicator(strokeWidth: 2),
          ),
          const SizedBox(width: 12),
          Text(
            'Memuat opsi pengiriman...',
            style: TextStyle(fontSize: 13, color: AppColors.neutralGray700),
          ),
        ],
      ),
    );
  }
}

class _ErrorPlaceholder extends StatelessWidget {
  final String message;
  final VoidCallback onRetry;
  const _ErrorPlaceholder({required this.message, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: AppColors.error.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: AppColors.error.withValues(alpha: 0.25)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Gagal memuat opsi pengiriman.',
            style: TextStyle(
              fontSize: 13,
              fontWeight: FontWeight.w600,
              color: AppColors.error,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            message,
            style: TextStyle(fontSize: 12, color: AppColors.neutralGray700),
          ),
          const SizedBox(height: 8),
          Align(
            alignment: Alignment.centerRight,
            child: OutlinedButton(
              onPressed: onRetry,
              child: const Text('Coba Lagi'),
            ),
          ),
        ],
      ),
    );
  }
}

class _EmptyOptionsBanner extends StatelessWidget {
  const _EmptyOptionsBanner();

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: AppColors.statusWarning.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: AppColors.statusWarning.withValues(alpha: 0.4),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Icon(
                Icons.local_shipping_outlined,
                size: 18,
                color: AppColors.statusWarning,
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  'Belum Ada Opsi Pengiriman Aktif',
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: AppColors.statusWarning,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 6),
          Text(
            'Belum ada opsi pengiriman aktif. Buat opsi pengiriman dulu sebelum publish listing.',
            style: TextStyle(fontSize: 12, color: AppColors.neutralGray700),
          ),
          const SizedBox(height: 10),
          Align(
            alignment: Alignment.centerLeft,
            child: ElevatedButton.icon(
              onPressed: () => context.push(RoutePaths.sellerShipping),
              style: ElevatedButton.styleFrom(
                backgroundColor: AppColors.primaryRed,
                foregroundColor: Colors.white,
                visualDensity: VisualDensity.compact,
              ),
              icon: const Icon(Icons.add, size: 16),
              label: const Text('Atur Pengiriman'),
            ),
          ),
        ],
      ),
    );
  }
}
