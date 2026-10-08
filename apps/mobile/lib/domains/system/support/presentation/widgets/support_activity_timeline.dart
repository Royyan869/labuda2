library;

/// Support Ticket Activity Timeline
///
/// I18N-18 — the ONE read-only user-facing activity surface for a Support
/// ticket. It renders the canonical ticket event history (created, claimed,
/// status/priority/category changes, resolved, closed, reopened, escalated)
/// that the owner-only `GET /api/v1/support/tickets/:id/events` contract
/// exposes.
///
/// Boundaries:
/// - READ-ONLY: the user can never manipulate event history here.
/// - Display copy comes ONLY from `AppLocalizations` through the canonical
///   `SupportEventTypeLabel` / `SupportEventDisplay` resolver — no raw wire
///   value, no enum name, and no technical metadata is ever rendered.
/// - Bounded: the section never displaces the ticket conversation; longer
///   histories scroll inside it.

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/support/domain/domain.dart';
import 'package:labuda/domains/system/support/presentation/providers/support_providers.dart';
import 'package:labuda/domains/system/support/presentation/utils/support_event_label.dart';
import 'package:labuda/shared/shared.dart';

class SupportActivityTimeline extends ConsumerWidget {
  const SupportActivityTimeline({super.key, required this.ticketId});

  final String ticketId;

  /// Maximum body height so the section stays a bounded part of the ticket
  /// detail screen and never pushes the conversation off the viewport.
  static const double _maxBodyHeight = 200;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final eventsAsync = ref.watch(supportTicketEventsProvider(ticketId));
    final events = eventsAsync.value ?? const <SupportEvent>[];
    final scheme = Theme.of(context).colorScheme;

    return Container(
      padding: const EdgeInsets.fromLTRB(
        AppMetrics.p16,
        AppMetrics.p12,
        AppMetrics.p16,
        AppMetrics.p12,
      ),
      decoration: BoxDecoration(
        color: scheme.surfaceContainer,
        border: Border(bottom: BorderSide(color: scheme.outlineVariant)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _TimelineHeader(count: events.length),
          const SizedBox(height: AppMetrics.p8),
          _buildBody(context, ref, eventsAsync, events),
        ],
      ),
    );
  }

  /// Canonical section composition over the ONE ticket-activity authority.
  ///
  /// A successful zero-result is Empty; a failed request is Error with a
  /// retry; while the first request is in flight it is Loading. The raw
  /// backend error never reaches the screen.
  Widget _buildBody(
    BuildContext context,
    WidgetRef ref,
    AsyncValue<List<SupportEvent>> eventsAsync,
    List<SupportEvent> events,
  ) {
    if (events.isEmpty) {
      if (eventsAsync.isLoading) {
        return const Padding(
          padding: EdgeInsets.symmetric(vertical: AppMetrics.p8),
          child: LoadingIndicator.small(),
        );
      }
      if (eventsAsync.hasError) {
        return _buildSectionError(context, ref);
      }
      return Text(
        context.l10n.supportTimelineEmpty,
        style: context.typeRoles.bodyDense.copyWith(
          color: Theme.of(context).colorScheme.onSurfaceVariant,
        ),
      );
    }

    return ConstrainedBox(
      constraints: const BoxConstraints(maxHeight: _maxBodyHeight),
      child: ListView.builder(
        shrinkWrap: true,
        padding: EdgeInsets.zero,
        itemCount: events.length,
        itemBuilder: (context, index) => _TimelineEntry(event: events[index]),
      ),
    );
  }

  /// Section-bounded failure: canonical localized copy plus the canonical
  /// retry action, never a raw error string and never a full-page error.
  Widget _buildSectionError(BuildContext context, WidgetRef ref) {
    final scheme = Theme.of(context).colorScheme;
    final l10n = context.l10n;
    return Row(
      children: [
        Icon(
          Icons.error_outline,
          size: AppIconSize.inlineGlyph,
          color: scheme.error,
        ),
        const SizedBox(width: AppMetrics.p8),
        Expanded(
          child: Text(
            l10n.pageErrorMessage,
            style: context.typeRoles.bodyDense.copyWith(
              color: scheme.onSurfaceVariant,
            ),
          ),
        ),
        TextButton(
          onPressed: () =>
              ref.invalidate(supportTicketEventsProvider(ticketId)),
          child: Text(l10n.retryAction),
        ),
      ],
    );
  }
}

class _TimelineHeader extends StatelessWidget {
  const _TimelineHeader({required this.count});

  final int count;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Row(
      children: [
        Icon(
          Icons.history,
          size: AppIconSize.inlineGlyph,
          color: scheme.onSurfaceVariant,
        ),
        const SizedBox(width: AppMetrics.p8),
        // Dynamic localized title — flex-bounded so high text scale cannot
        // overflow the header Row.
        Flexible(
          child: Text(
            context.l10n.supportTimelineTitle,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: context.typeRoles.labelMicro.copyWith(
              fontWeight: FontWeight.bold,
              color: scheme.onSurface,
            ),
          ),
        ),
        if (count > 0) ...[
          const SizedBox(width: AppMetrics.p8),
          Flexible(
            child: Text(
              '($count)',
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: context.typeRoles.labelMicro.copyWith(
                color: scheme.onSurfaceVariant,
              ),
            ),
          ),
        ],
      ],
    );
  }
}

class _TimelineEntry extends StatelessWidget {
  const _TimelineEntry({required this.event});

  final SupportEvent event;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final l10n = context.l10n;
    final detail = event.detail(l10n);

    return Padding(
      padding: const EdgeInsets.symmetric(vertical: AppMetrics.p8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: AppMetrics.p24 + AppMetrics.p4,
            height: AppMetrics.p24 + AppMetrics.p4,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: scheme.primary.withAlpha(30),
              shape: BoxShape.circle,
            ),
            child: Icon(
              _iconFor(event.eventType),
              size: AppIconSize.inlineGlyph,
              color: scheme.primary,
            ),
          ),
          const SizedBox(width: AppMetrics.p12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  event.eventType.label(l10n),
                  style: context.typeRoles.bodyDense.copyWith(
                    fontWeight: FontWeight.bold,
                    color: scheme.onSurface,
                  ),
                ),
                if (detail != null) ...[
                  const SizedBox(height: AppMetrics.p4),
                  Text(
                    detail,
                    style: context.typeRoles.labelMicro.copyWith(
                      color: scheme.onSurfaceVariant,
                    ),
                  ),
                ],
              ],
            ),
          ),
          const SizedBox(width: AppMetrics.p8),
          // Absolute event timestamp — secondary metadata. Flex-bounded with
          // compact single-line ellipsis so narrow width / high text scale
          // cannot overflow the entry Row. Formatter authority unchanged
          // (AppFormatters.formatDateTime).
          Flexible(
            child: Text(
              AppFormatters.formatDateTime(event.createdAt),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: context.typeRoles.labelMicro.copyWith(
                color: scheme.onSurfaceVariant,
              ),
            ),
          ),
        ],
      ),
    );
  }

  /// Presentation-layer glyph per canonical event identity. Kept here (not on
  /// the domain entity) so the event model stays free of presentation copy.
  IconData _iconFor(SupportEventType type) {
    switch (type) {
      case SupportEventType.ticketCreated:
        return Icons.add_circle_outline;
      case SupportEventType.ticketClaimed:
        return Icons.person_add_alt_1_outlined;
      case SupportEventType.ticketWaitingUser:
        return Icons.hourglass_empty;
      case SupportEventType.statusChanged:
        return Icons.sync;
      case SupportEventType.priorityChanged:
        return Icons.flag_outlined;
      case SupportEventType.categoryChanged:
        return Icons.category_outlined;
      case SupportEventType.ticketResolved:
        return Icons.check_circle_outline;
      case SupportEventType.ticketClosed:
        return Icons.lock_outline;
      case SupportEventType.ticketReopened:
        return Icons.restore;
      case SupportEventType.adminAssigned:
        return Icons.assignment_ind_outlined;
      case SupportEventType.adminUnassigned:
        return Icons.person_remove_outlined;
      case SupportEventType.ticketEscalated:
        return Icons.report_problem_outlined;
      case SupportEventType.unknown:
        return Icons.info_outline;
    }
  }
}
