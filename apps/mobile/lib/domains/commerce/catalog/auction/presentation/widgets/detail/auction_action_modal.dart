/// Auction Action Modal
///
/// Modal for placing bid or using Buy Now feature
/// Buy Now is an auction feature that ends the auction immediately
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/utils/money_input_formatter.dart';
import 'package:hishumi/shared/widgets/app_bottom_sheet_base.dart';
import 'package:hishumi/shared/widgets/app_dialog.dart';
import 'package:hishumi/shared/widgets/app_snackbar.dart';
import 'package:hishumi/shared/domain/entities/resource_projection.dart';
import 'package:hishumi/domains/commerce/catalog/auction/domain/entities/auction.dart';

/// Canonical Place Bid amount representation is integer (backend binds
/// `amount` to int64 and persists to PostgreSQL bigint; a JSON literal like
/// `1000000.0` is rejected at the binding). The whole live write chain emits
/// int only — no double alias, no silent coercion.
typedef BidCallback = void Function(int amount);
typedef BuyNowCallback = void Function();

/// Canonical parse for the Place Bid amount input.
///
/// Integer is the single numeric representation of the live Place Bid write
/// chain (backend binds `amount` to int64, persists to PostgreSQL bigint).
/// Fractional or malformed input parses to null and MUST be rejected
/// explicitly by the caller — no round/floor/ceil/truncation ever happens
/// here: "1000000.9" never becomes 1000000 silently.
int? parseCanonicalBidAmount(String rawInput) {
  final input = rawInput.trim();
  if (input.isEmpty) return null;
  // Thousands-grouped form: strip separators only for strictly valid
  // grouping — the canonical mask emits `1.000.000`; the legacy comma form
  // (`1,000,000`) stays accepted so already-typed values keep parsing. Any
  // other placement (e.g. `12,5`, which in id-ID locale means 12.5) is
  // REJECTED explicitly — never silently reinterpreted. Fractional and
  // malformed input likewise return null so the caller rejects the bid
  // instead of coercing the nominal.
  final grouped = RegExp(r'^\d{1,3}([.,]\d{3})+$');
  final normalized = grouped.hasMatch(input)
      ? input.replaceAll(RegExp(r'[.,]'), '')
      : input;
  if (normalized.contains(',') || normalized.contains('.')) return null;
  // Strict digit shape (an optional sign only): "12a" must stay rejected
  // instead of being silently coerced by a lenient digit stripper.
  final negative = normalized.startsWith('-');
  final unsigned = negative ? normalized.substring(1) : normalized;
  if (!RegExp(r'^\d+$').hasMatch(unsigned)) return null;
  // The digit-to-int step is the canonical money parse (no second parse
  // engine; the strict separator/shape rejection above is the bid rule).
  final parsed = MoneyInputFormatter.parseAmount(unsigned);
  if (parsed == null) return null;
  return negative ? -parsed : parsed;
}

/// Action modal for auction detail
class AuctionActionModal extends ConsumerStatefulWidget {
  final Auction auction;
  final BidCallback onPlaceBid;
  final BuyNowCallback onBuyNow;

  const AuctionActionModal({
    super.key,
    required this.auction,
    required this.onPlaceBid,
    required this.onBuyNow,
  });

  /// Show the action modal
  static Future<void> show(
    BuildContext context, {
    required Auction auction,
    required BidCallback onPlaceBid,
    required BuyNowCallback onBuyNow,
  }) {
    return AppBottomSheetBase.show<void>(
      context: context,
      title: 'Tawar Lelang',
      content: AuctionActionModal(
        auction: auction,
        onPlaceBid: onPlaceBid,
        onBuyNow: onBuyNow,
      ),
    );
  }

  @override
  ConsumerState<AuctionActionModal> createState() => _AuctionActionModalState();
}

class _AuctionActionModalState extends ConsumerState<AuctionActionModal> {
  late TextEditingController _bidController;
  late int _minimumBid;

  @override
  void initState() {
    super.initState();
    // Read entity is canonical int (PASS 1 numeric read convergence) — the
    // minimum is computed int + int with no conversion bridge of any kind.
    _minimumBid = widget.auction.currentBid + widget.auction.bidIncrement;
    // Seeded in the canonical display form: the input mask groups while
    // editing, so the prefilled value must read the same way (and the parse
    // below reads both forms).
    _bidController = TextEditingController(
      text: MoneyInputFormatter.display(_minimumBid),
    );
  }

  @override
  void dispose() {
    _bidController.dispose();
    super.dispose();
  }

  Future<void> _handlePlaceBid() async {
    // Canonical integer parsing — fractional or malformed input is rejected
    // explicitly at this boundary. "1000000.9" never reaches the chain as a
    // coerced 1000000; there is no round/floor/ceil and no double detour
    // anywhere below this parse.
    final amount = parseCanonicalBidAmount(_bidController.text);
    if (amount == null) {
      AppSnackBar.showError(
        context,
        'Nominal bid harus bilangan bulat rupiah (tanpa desimal).',
      );
      return;
    }
    if (amount < _minimumBid) {
      AppSnackBar.showError(
        context,
        'Bid minimum: Rp ${formatGroupedAmount(_minimumBid)}',
      );
      return;
    }

    // Show confirmation dialog before placing bid.
    // F9(a) convergence: pure place-bid yes/no decision consumes the
    // canonical AppDialog.confirm grammar (same order and meaning). The
    // side effects stay caller-side: confirm only resolves the decision.
    final confirmed = await AppDialog.confirm(
      context: context,
      title: 'Konfirmasi Penawaran',
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text('Kamu akan menawar sebesar'),
          const SizedBox(height: 12),
          Text(
            'Rp ${formatGroupedAmount(amount)}',
            style: context.typeRoles.titleProminent.copyWith(
              fontWeight: FontWeight.bold,
              // Money reads as the brand price role — same authority the
              // ForSale detail price and the checkout totals use.
              color: Theme.of(context).colorScheme.primary,
            ),
          ),
          const SizedBox(height: 16),
          // TRANSACTION CLARITY: Consequence warning for auction inaction
          Container(
            padding: const EdgeInsets.all(AppMetrics.p12),
            decoration: BoxDecoration(
              color: context.statusColors.warning.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(AppShape.r8),
              border: Border.all(
                color: context.statusColors.warning.withValues(alpha: 0.3),
              ),
            ),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Icon(
                  Icons.info_outline,
                  size: AppIconSize.inlineGlyph,
                  color: context.statusColors.warning,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    'Jika Anda menang dan tidak membayar, akun Anda dapat dibatasi',
                    style: context.typeRoles.labelMicro.copyWith(
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
      confirmLabel: 'Konfirmasi',
      cancelLabel: 'Batal',
    );
    if (!confirmed) return;
    if (!mounted) return;
    Navigator.of(context).pop();
    widget.onPlaceBid(amount);
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final currentBid = widget.auction.currentBid;
    final caps = widget.auction.viewerCapabilities;
    final canBid = caps?.canBid ?? false;
    final showBuyNow = caps?.canBuyNow ?? false;

    return Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // Current bid info
          Container(
            padding: const EdgeInsets.all(AppMetrics.p12),
            decoration: BoxDecoration(
              color: scheme.surfaceContainerHighest,
              borderRadius: BorderRadius.circular(AppShape.r8),
            ),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                const Text('Bid Saat Ini'),
                Text(
                  'Rp ${formatGroupedAmount(currentBid)}',
                  style: TextStyle(
                    fontWeight: FontWeight.bold,
                    color: scheme.primary,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 8),
          // Next bid info
          Container(
            padding: const EdgeInsets.all(AppMetrics.p12),
            decoration: BoxDecoration(
              color: scheme.secondary.withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(AppShape.r8),
            ),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                const Text('Bid Minimum'),
                Text(
                  'Rp ${formatGroupedAmount(_minimumBid)}',
                  style: TextStyle(
                    fontWeight: FontWeight.bold,
                    color: scheme.secondary,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),

          // Bid amount input
          const Text('Masukkan Jumlah Bid'),
          const SizedBox(height: 8),
          TextField(
            controller: _bidController,
            // Canonical integer amount: digits-only, grouped with the one
            // money-input mask while typing (`1000000` → `1.000.000`). The
            // business value stays an int — fractions cannot be typed,
            // pasted, or silently coerced anywhere on the Place Bid path.
            keyboardType: const TextInputType.numberWithOptions(decimal: false),
            inputFormatters: const [MoneyInputFormatter()],
            // Border/fill come from `inputDecorationTheme` (AppTheme) — the
            // one form-field authority.
            decoration: InputDecoration(
              hintText: 'Rp $_minimumBid',
              prefixText: 'Rp ',
            ),
          ),
          const SizedBox(height: 16),
          // Expired-seller badge — render above CTAs when seller-trust is
          // degraded so the user understands why the buttons are disabled.
          // Place bid button
          ElevatedButton(
            onPressed: canBid ? _handlePlaceBid : null,
            // CTA fill comes from the button theme (scheme.primary).
            style: ElevatedButton.styleFrom(
              padding: const EdgeInsets.symmetric(vertical: AppMetrics.p16),
            ),
            child: const Text(
              'Pasang Bid',
              style: TextStyle(fontWeight: FontWeight.bold),
            ),
          ),
          // Buy now button — canonical capability (can_buy_now) when present.
          if (showBuyNow) ...[
            const SizedBox(height: 12),
            OutlinedButton(
              onPressed: () {
                Navigator.of(context).pop();
                widget.onBuyNow();
              },
              style: OutlinedButton.styleFrom(
                padding: const EdgeInsets.symmetric(vertical: AppMetrics.p16),
              ),
              child: Text(
                'Buy Now - Rp ${formatGroupedAmount(widget.auction.buyNowPrice!.round())}',
                style: const TextStyle(fontWeight: FontWeight.bold),
              ),
            ),
          ],
        ],
    );
  }
}
