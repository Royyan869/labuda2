/// Canonical negotiation PROPOSAL CARD — a commerce-owned component hosted
/// inside the chat message stream.
///
/// AUTHORITY SPLIT (owner rule: chat never handles commerce):
/// - chat mounts this card and forwards the Beli intent to its screen
///   (checkout resolution stays with the owning screen, same pattern as the
///   resource projection card);
/// - ALL negotiation state, turn rules and execution live in the commerce
///   negotiation authority (session entity + NegotiationNotifier + offer sheet);
/// - product display data resolves through the commerce catalog authority
///   (forSaleDetailProvider).
///
/// OWNER DESIGN (canonical, 2026-09-30):
/// - product photo + needed data rendered like a for-sale card in chat;
/// - CTA lives ONLY on the OPPONENT's latest proposal of an ACTIVE session —
///   the exact offer the viewer is being asked to answer: [Terima | Counter],
///   with [Tolak] added for the SELLER only (owner Option A — the buyer
///   exits by countering or by ignoring until auto-expire);
/// - my own latest proposal is a WAITING state, superseded rounds carry NO
///   action row (disabled buttons on old rounds are a KILLED design: they
///   advertise actions that cannot be taken);
/// - buyer Terima = accept + straight to checkout (deal price binding);
/// - accepted deal (latest card) → deal price + 24h validity and, when the
///   deal came from the other side, the buyer's Beli intent;
/// - terminal → status chip, no actions.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/domain/entities/negotiation.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_providers.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/widgets/negotiation_offer_sheet.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/shared/shared.dart';

class NegotiationProposalCard extends ConsumerWidget {
  const NegotiationProposalCard({
    super.key,
    required this.attachment,
    required this.isFromCurrentUser,
    this.onDealBuy,
  });

  final NegotiationProposalAttachment attachment;

  /// TRUE when the VIEWER sent this proposal — transport truth carried by the
  /// message row itself. Never derived from proposal_sequence parity: parity
  /// is a client-side reconstruction that broke the moment one side countered
  /// twice.
  final bool isFromCurrentUser;

  /// DEAL → checkout intent. The owning screen resolves product id, the
  /// seller trust gate and the negotiation binding (Commerce stays the
  /// transaction authority — this card never navigates to checkout itself).
  final VoidCallback? onDealBuy;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final session = ref.watch(negotiationNotifierProvider).currentNegotiation;
    final viewerId = ref.watch(currentUserIdProvider);

    final inSession = session != null && session.id == attachment.sessionId;

    // CANONICAL CTA PLACEMENT (owner truth, 2026-09-30):
    //   • action row exists ONLY on the OPPONENT's latest proposal of an
    //     ACTIVE session — the exact offer the viewer must answer;
    //   • my own latest proposal is a WAITING state, never a button row;
    //   • superseded rounds carry NO action row (disabled buttons on old
    //     rounds are a KILLED design);
    //   • terminal → chip only; accepted (latest) → deal block.
    final bool latest =
        inSession && attachment.proposalSequence == session.round;
    final bool active =
        inSession && session.status == NegotiationStatus.active;
    final bool deal =
        inSession &&
        session.status == NegotiationStatus.accepted &&
        latest;
    final bool myTurn = inSession && session.canUserAct(viewerId);
    final bool showActions =
        active && latest && !isFromCurrentUser && myTurn;
    // Option A (owner): Tolak is SELLER-ONLY. The buyer exits by countering
    // or by letting the active session auto-expire.
    final bool showReject = showActions && session.isSeller(viewerId);
    final bool showTerminalChip =
        latest && inSession && session.status.isTerminal;
    final bool waiting = active && latest && isFromCurrentUser;

    // Product display data — commerce catalog authority. Absent resourceId
    // (legacy payload) or dead listing simply renders no product block.
    final forSaleId = attachment.resourceId;
    final forSale = (forSaleId == null || forSaleId.isEmpty)
        ? null
        : ref.watch(forSaleDetailProvider(forSaleId)).value;

    final isBuyer = inSession && session.isBuyer(viewerId);

    final headerLabel = attachment.proposalSequence <= 1
        ? 'Penawaran Awal'
        : 'Penawaran Balasan • Ronde ${attachment.proposalSequence}';

    return Container(
      constraints: const BoxConstraints(maxWidth: 300),
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(
          color: AppColors.coinPrimary.withValues(alpha: 0.4),
          width: 1.5,
        ),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            padding: const EdgeInsets.symmetric(
              horizontal: AppMetrics.p12,
              vertical: AppMetrics.p8,
            ),
            decoration: BoxDecoration(
              color: AppColors.coinPrimary.withValues(alpha: 0.1),
              borderRadius: const BorderRadius.vertical(
                top: Radius.circular(AppShape.r11),
              ),
            ),
            child: Row(
              children: [
                Icon(
                  Icons.handshake_outlined,
                  size: AppIconSize.action,
                  color: AppColors.coinPrimary,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    headerLabel,
                    style: TextStyle(
                      fontSize: AppType.s14,
                      fontWeight: FontWeight.w600,
                      color: AppColors.coinPrimary,
                    ),
                  ),
                ),
                if (showTerminalChip)
                  _StatusChip(
                    label: session.status == NegotiationStatus.cancelled
                        ? 'Penawaran Ditolak'
                        : 'Penawaran Kedaluwarsa',
                    color: scheme.error,
                  ),
                if (deal)
                  _StatusChip(label: 'Disetujui', color: scheme.primary),
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(AppMetrics.p12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (forSale != null) ...[
                  _ProductRow(forSale: forSale),
                  const SizedBox(height: AppMetrics.p12),
                ],
                Text(
                  'Harga Penawaran',
                  style: TextStyle(
                    fontSize: AppType.s12,
                    color: scheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  // One money formatter for the whole app.
                  'Rp ${formatGroupedAmount(attachment.price)}',
                  style: TextStyle(
                    fontSize: AppType.s16,
                    fontWeight: FontWeight.w700,
                    color: scheme.onSurface,
                  ),
                ),
                if (attachment.note != null &&
                    attachment.note!.isNotEmpty) ...[
                  const SizedBox(height: 8),
                  Text(
                    attachment.note!,
                    style: TextStyle(fontSize: AppType.s12, color: scheme.onSurface),
                    maxLines: 3,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
                if (deal) ...[
                  const SizedBox(height: 10),
                  _DealBlock(
                    agreedPrice:
                        (session.agreedPrice ?? session.currentOfferPrice)
                            .round(),
                    validityLine: _dealValidityLine(session),
                    showBuy: isBuyer,
                    onBuy: onDealBuy,
                  ),
                ],
                if (showActions) ...[
                  const SizedBox(height: 10),
                  Row(
                    children: [
                      Expanded(
                        child: ElevatedButton(
                          onPressed: () => _accept(context, ref, session),
                          child: const Text(
                            'Terima',
                            style: TextStyle(
                              fontSize: AppType.s12,
                              fontWeight: FontWeight.w700,
                            ),
                          ),
                        ),
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: OutlinedButton(
                          onPressed: () =>
                              _counter(context, ref, session, forSale),
                          child: const Text(
                            'Counter',
                            style: TextStyle(
                              fontSize: AppType.s12,
                              fontWeight: FontWeight.w700,
                            ),
                          ),
                        ),
                      ),
                      if (showReject) ...[
                        const SizedBox(width: 8),
                        Expanded(
                          child: OutlinedButton(
                            onPressed: () => _reject(context, ref, session),
                            child: const Text(
                              'Tolak',
                              style: TextStyle(
                                fontSize: AppType.s12,
                                fontWeight: FontWeight.w700,
                              ),
                            ),
                          ),
                        ),
                      ],
                    ],
                  ),
                ],
                if (waiting) ...[
                  const SizedBox(height: 8),
                  Text(
                    'Menunggu respons ${session.isSeller(viewerId) ? 'pembeli' : 'penjual'}…',
                    style: TextStyle(
                      fontSize: AppType.s12,
                      fontStyle: FontStyle.italic,
                      color: scheme.onSurfaceVariant,
                    ),
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }

  /// TERIMA — accept the current price (either participant). Execution is
  /// delegated to the commerce negotiation authority.
  Future<void> _accept(
    BuildContext context,
    WidgetRef ref,
    Negotiation session,
  ) async {
    final notifier = ref.read(negotiationNotifierProvider.notifier);
    final viewerId = ref.read(currentUserIdProvider);
    final isBuyer = session.isBuyer(viewerId);
    final result = await notifier.acceptOffer(
      chatRoomId: session.chatId,
      sessionId: session.id,
    );
    if (!context.mounted) return;
    if (result.isSuccess) {
      AppSnackBar.showSuccess(context, 'Penawaran diterima — harga disepakati');
      // BUYER TRUTH (owner, 2026-09-30): Terima = LANGSUNG BELI. The accept
      // has committed, so the owning screen's checkout intent takes over —
      // product id, seller trust gate and the deal-price binding resolve
      // there (this card navigates nowhere itself).
      if (isBuyer) onDealBuy?.call();
      return;
    }
    AppSnackBar.showError(
      context,
      result.error ?? 'Gagal menerima penawaran',
    );
  }

  /// TOLAK — cancel the active session (either participant).
  Future<void> _reject(
    BuildContext context,
    WidgetRef ref,
    Negotiation session,
  ) async {
    final notifier = ref.read(negotiationNotifierProvider.notifier);
    final result = await notifier.cancelNegotiation(
      chatRoomId: session.chatId,
      sessionId: session.id,
    );
    if (!context.mounted) return;
    if (result.isSuccess) {
      AppSnackBar.showWarning(context, 'Penawaran ditolak');
    } else {
      AppSnackBar.showError(context, result.error ?? 'Gagal menolak penawaran');
    }
  }

  /// COUNTER — nominal entered in the canonical offer sheet (commerce-owned),
  /// posted to the room-scoped counter endpoint. No navigation.
  Future<void> _counter(
    BuildContext context,
    WidgetRef ref,
    Negotiation session,
    ForSale? forSale,
  ) async {
    final notifier = ref.read(negotiationNotifierProvider.notifier);
    final productTitle = forSale != null
        ? forSale.title
        : (session.forSaleName.isNotEmpty ? session.forSaleName : 'Produk');
    final sent = await NegotiationOfferSheet.show(
      context: context,
      productTitle: productTitle,
      onSubmit: (price) async {
        final result = await notifier.counterOffer(
          chatRoomId: session.chatId,
          sessionId: session.id,
          price: price,
        );
        if (result.isSuccess && result.data != null) return null;
        return result.error ?? 'Gagal mengirim counter. Coba lagi.';
      },
    );
    if (!sent || !context.mounted) return;
    AppSnackBar.showSuccess(context, 'Counter terkirim');
  }

  /// DEAL VALIDITY (owner truth: valid 24h from the deal) — renders the
  /// remaining window from the wire's `expires_at`. Legacy NULL expiry
  /// renders no line (never fabricate a deadline).
  static String? _dealValidityLine(Negotiation session) {
    final expiresAt = session.expiresAt;
    if (expiresAt == null) return null;
    final remaining = expiresAt.difference(DateTime.now());
    if (remaining.isNegative) return 'Berlaku sudah habis';
    final hours = remaining.inHours;
    final minutes = remaining.inMinutes.remainder(60);
    return 'Berlaku sisa ${hours}j ${minutes}m';
  }
}

/// Product identity block — same facts the for-sale-in-chat card shows
/// (photo, title, availability, list price), resolved via the catalog authority.
class _ProductRow extends StatelessWidget {
  const _ProductRow({required this.forSale});

  final ForSale forSale;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final media = forSale.media;
    final thumbUrl = media.isNotEmpty
        ? (media.first.thumbnailUrl ?? media.first.originalUrl)
        : null;
    final available = forSale.isAvailable;
    return Row(
      children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(AppShape.r8),
          child: SizedBox(
            width: AppIconSize.display + AppMetrics.p8,
            height: AppIconSize.display + AppMetrics.p8,
            child: thumbUrl == null
                ? Container(
                    color: scheme.surfaceContainerHighest,
                    child: Icon(
                      Icons.storefront_outlined,
                      size: AppIconSize.header,
                      color: scheme.onSurfaceVariant,
                    ),
                  )
                : Image.network(
                    thumbUrl,
                    fit: BoxFit.cover,
                    errorBuilder: (_, _, _) => Container(
                      color: scheme.surfaceContainerHighest,
                      child: Icon(
                        Icons.storefront_outlined,
                        size: AppIconSize.header,
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                  ),
          ),
        ),
        const SizedBox(width: 10),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                forSale.title,
                style: TextStyle(
                  fontSize: AppType.s14,
                  fontWeight: FontWeight.w600,
                  color: scheme.onSurface,
                ),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
              const SizedBox(height: 2),
              Text(
                '${available ? 'Tersedia' : 'Habis'} · ${forSale.formattedPrice}',
                style: TextStyle(
                  fontSize: AppType.s12,
                  color: scheme.onSurfaceVariant,
                ),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ],
          ),
        ),
      ],
    );
  }
}

/// Accepted-deal block: agreed price + 24h validity + buyer's Beli intent.
class _DealBlock extends StatelessWidget {
  const _DealBlock({
    required this.agreedPrice,
    required this.validityLine,
    required this.showBuy,
    required this.onBuy,
  });

  final int agreedPrice;
  final String? validityLine;
  final bool showBuy;
  final VoidCallback? onBuy;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final title = 'Rp ${formatGroupedAmount(agreedPrice)}';
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: scheme.primary.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(AppShape.r8),
        border: Border.all(color: scheme.primary.withValues(alpha: 0.3)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            'Harga Disetujui!',
            style: TextStyle(
              fontSize: AppType.s12,
              fontWeight: FontWeight.w700,
              color: scheme.primary,
            ),
          ),
          const SizedBox(height: 2),
          Text(
            'Harga Deal: $title',
            style: TextStyle(
              fontSize: AppType.s14,
              fontWeight: FontWeight.w700,
              color: scheme.onSurface,
            ),
          ),
          if (validityLine != null)
            Text(
              validityLine!,
              style: TextStyle(fontSize: AppType.s12, color: scheme.onSurfaceVariant),
            ),
          if (showBuy) ...[
            const SizedBox(height: 8),
            SizedBox(
              height: AppContentSize.controlCompact,
              child: ElevatedButton(
                onPressed: onBuy,
                child: const Text(
                  'Beli dengan Harga Deal',
                  style: TextStyle(
                    fontSize: AppType.s12,
                    fontWeight: FontWeight.bold,
                  ),
                ),
              ),
            ),
          ] else ...[
            const SizedBox(height: 4),
            Text(
              'Menunggu pembeli checkout…',
              style: TextStyle(
                fontSize: AppType.s12,
                fontStyle: FontStyle.italic,
                color: scheme.onSurfaceVariant,
              ),
            ),
          ],
        ],
      ),
    );
  }
}

class _StatusChip extends StatelessWidget {
  const _StatusChip({required this.label, required this.color});

  final String label;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p8,
        vertical: AppMetrics.p4,
      ),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(AppShape.r8),
      ),
      child: Text(
        label,
        style: TextStyle(
          fontSize: AppType.s12,
          fontWeight: FontWeight.w700,
          color: color,
        ),
      ),
    );
  }
}
