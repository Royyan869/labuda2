/// GEOMETRY AUTHORITY RATCHET.
///
/// The colour, type, corner, elevation and motion ladders are defended by HARD
/// gates (`theme_authority_contract_test.dart`): one raw literal outside the
/// three authority files fails the lib-wide sweep outright. Five geometry
/// decisions never got that protection, so each grew a second, unguarded
/// spelling beside the ladder that already names it — see
/// `test/support/geometry_authority_gate.dart` for the census and the rule.
///
/// One more lens was added on 2026-10-02 after a regression shipped through it:
/// `frozenExtent` — a box that promises a literal width/height to content the
/// ladder measures, which is how a toolbar `height: 60` died by 3 px when a type
/// step moved under it.
///
/// A hard gate cannot be switched on for these yet: the real app holds ~2.9k of
/// them, and turning the rule on first would clean nothing — it would only
/// paint the suite red. So the same ratchet doctrine used for typography is
/// applied here:
///
/// 1. every category is capped at the count measured on 2026-10-02, so a new
///    `SizedBox(height: 16)` or `Icon(size: 20)` fails the gate instead of
///    quietly joining the backlog;
/// 2. a finished file is locked to ZERO BY PATH, so a migrated slice cannot
///    regain a literal;
/// 3. as long as a category still has a cap, its detector must still SEE that
///    backlog — a census that suddenly reads nothing fails instead of looking
///    like a finished migration;
/// 4. the detectors must fire on a planted resurrection, and must spare the
///    ladders and proportional geometry, before they may certify anything.
library;

import 'package:flutter_test/flutter_test.dart';

import '../../support/geometry_authority_gate.dart';

/// Frozen counts, measured by [geometryCensus] on 2026-10-02 (1069 files
/// swept; 1070 at the split). They may only go DOWN: lowering a number here is
/// the reward for finishing a slice, and raising one is a second authority
/// growing.
///
/// `spacing` is the only category whose steps the app can already name
/// (`AppMetrics.p16`): all but six of its sites sit on a value the ladder owns,
/// and those six are the finding — three `SizedBox(height: 100)` clearances for
/// a bottom bar, two 18px gaps, one `bottomPadding > 0 ? … : 16`. The
/// `iconSize`, `strokeWidth`, `lineMetric` and `shadow` categories need a ladder
/// (or a role) to exist before they can migrate — a separate decision, not a
/// licence to keep writing literals. (iconSize got its ladder first —
/// `AppIconSize` — and migrated to cap 0 on 2026-10-02; the other three are
/// still waiting for their own decision.)
///
/// `contentDimension` is NOT a migration target for the spacing ladder, and that
/// is exactly why it was cut out: a 64×64 avatar box, a 40×4 drag handle or a 1px
/// divider is the SIZE OF something, so it needs a size policy of its own. Cap it
/// so the backlog cannot grow, and triage it on its own.
///
/// `frozenExtent` asks a DIFFERENT question of the same family of sites: not "is
/// this literal on the ladder?" but "does this literal promise an extent that
/// content the ladder measures has to fit inside?". It was added on 2026-10-02
/// because a real regression shipped through exactly that hole —
/// `content_toolbar_widget` kept `height: 60` while its label moved to a
/// different type step, the icon+label column overflowed by 3 px, and only a
/// widget test could see it. The literal never changed; the ladder moved. A
/// frozen box is usually BOTH a `contentDimension` and a `frozenExtent`, and the
/// two are deliberately kept apart: "migrate this to a ladder" and "this promise
/// has no slack for the ladder to grow into" are different jobs.
///
/// Measured 2026-10-02, the day the lens landed: 72 sites over 45 files. Five
/// slices then cut it to 35, each one re-measured rather than assumed; four
/// more (slices 6–9, the day the content-size policy landed) took it to zero:
///
/// 1. the wilayah field family spelled one promise — `height: 50` on the row that
///    stands in for a dropdown — twenty-one times across five files, and three of
///    those files were re-implementing the shared `DropdownStateBuilders`
///    privately, four methods each. The shared row is now content-driven and the
///    private copies are purged, which also lowered `spacing` (the deleted copies
///    were full of `SizedBox(width: 12)`), `contentDimension` and `iconSize`
///    (`size: 20` → `AppIconSize.action`);
/// 2. the toolbar's `height: 60` itself (the regression that produced the lens);
/// 3. `PendingMediaStrip`, where the strip's height and the tile's extent were the
///    same `72` spelled five times with nothing forcing them to agree — one named
///    extent now drives both;
/// 4. the DRAG HANDLE: `width: 40 / height: 4 / outlineVariant / AppShape.r2`
///    was spelled fourteen times. This one taught two lessons. First, this lens
///    saw only TEN of them — the other four carried no ladder token in their
///    block, so a `frozenExtent` census could not find them; they were found by
///    grepping the shape, which is a reminder that the lens guards "a promise
///    that ladder-measured content must fit inside", not "a duplicated bar".
///    Second, the rule was sharpened to require a CHILD: a childless `height: 1`
///    divider or bar cannot overflow, so it belongs to `contentDimension` alone
///    (that refinement dropped the count 40 → 36 while `contentDimension` stayed
///    put at 177 — the sites moved lens, they did not vanish);
/// 5. the NOTIFICATION FILTER RAIL — `height: 50` around a horizontal ListView.
///    This was the one case a census can flag but not repair: a horizontal list
///    CLIPS its cross axis with no overflow stripe whatsoever, so nothing in the
///    source could ever warn that a chip had grown past 50. The fix is structural
///    — a Row inside a horizontal scroll view (the filters are a CLOSED enum, so
///    the lazy builder bought nothing) — and the rail now sizes itself from its
///    chips. That was the LAST `height: 50` in `lib`; the literal is gone app-wide.
/// 6. CONTROLS, on the day the content-size policy landed (2026-10-02): ten
///    promises carrying one job each — button heights `48` ×6 and `52` ×2 (the
///    welcome CTAs had drifted 4 px taller for no reason), an in-card `36`, a
///    header select `116×32`, a step badge `24×24` and the app-bar search pill
///    `40`. The heights and widths now read `AppContentSize`; the search pill
///    went CONTENT-DRIVEN instead (its height was hand-summed around a 20 px
///    icon — vertical `p8` padding says the same thing and cannot drift).
///    `frozenExtent` 35 → 25, `contentDimension` 177 → 171, re-measured.
/// 7. COLUMNS AND CTAS (2026-10-02, same pass): the description-row label
///    column spelled `100` ×3, `110` and `120` — five names for one job — and
///    the CTA width spelled `280` ×2 and `240` ×2. Both now read
///    `AppContentSize.termLabel` / `AppContentSize.actionWidth` (ties rounded
///    UP: labels never shrink, CTAs never narrow). The rating breakdown's
///    count slot (`22`) became `AppContentSize.badge` rather than
///    content-driven: the five bars only end at the same x BECAUSE the count
///    column has one fixed width — removing the box would have misaligned
///    them. `frozenExtent` 25 → 15, `contentDimension` 171 → 161.
/// 8. MEDIA AND PREVIEWS (2026-10-02): the surfaces a koi marketplace
///    actually shows — recommendation rail `180` + its card `140`, static map
///    `180` (same `preview` step), capture areas `200` ×2, media grid tile
///    `112`, suggestion tray `120`, mention overlay `250` — plus two
///    structural repairs: the upload dropzone dropped its hand-summed
///    `height: 150` for `p32` vertical padding (content-driven, −2 px), and
///    the time-picker wheels now derive `height` from ONE named
///    `wheelItemExtent` (×3 rows) with a named column width, so the box and
///    its `itemExtent` cannot drift apart — exactly the agreement the
///    `PendingMediaStrip` slice established. `frozenExtent` 15 → 4.
/// 9. SURFACES AND SPECIALS — the last four: dialog body `340` and the
///    cropper canvas `500×600` (ONE `Size` name, so the axes cannot drift)
///    onto the ladder; the helper-text reserve became content-driven (the
///    `? 20 : 0` wrapper WAS a hand-summed budget for text the ladder
///    measures); the wizard's label clamp now spells its bounds once instead
///    of `35.0` twice. `frozenExtent` 4 → 0 — at that moment the category
///    was CAP-AT-ZERO.
/// 10. THE ICON MIGRATION RE-OPENED THIS LENS (2026-10-02, same day) —
///     re-measured visibility, NOT a growing backlog: `AppIconSize` joined
///     `_ladderContent` when the 657-site `iconSize` migration landed, and a
///     box that freezes an extent while its subtree holds an icon token is a
///     frozen budget the census was previously BLIND to (those subtrees were
///     pure raw literals — no ladder evidence, no finding). 56 appeared; the
///     11 whose files carry path locks (plus one identical twin) were fixed
///     the same day — circle icon-slots became content-driven padding
///     (`(box − icon) / 2` must land on the spacing ladder: 80→`p16`,
///     56→`p12`, 48→`p12`) or the icon step itself (48→`display`,
///     20→`action`, 40→`action * 2` so the avatar fallback keeps parity with
///     the live 40-px avatar), the 56-px thumbnail became `display + p8`,
///     and the two hand-summed `height: 225` gallery placeholders became
///     `AspectRatio(4 / 5)` — the SAME ratio the live carousel passes beside
///     them, so the extent is derived, not spelled. The remaining 45 ARE the
///     new measured backlog: cap 0 → 45 is a LENS-WIDENING re-baseline
///     (existing debt becoming visible), never permission for new debt — and
///     the next triage finds exactly the sites this lens was built to point
///     at.
///
/// THE ICON MIGRATION itself (same day): `iconSize` 657 → 0 in four measured
/// slices, designed from the histogram (657 sites · 18 distinct values · 226
/// files), not guesswork. Iris 1 — the 383 sites already ON the approved
/// steps (16/20/24/32/48): pure token swaps, zero pixel change. Iris 2 — the
/// micro band 14/12/11 → `inlineGlyph` (+2…+5 px). Iris 3 — the near band
/// 18 → `action`, 22 → `header`, 28/36 → `emphasis`. Iris 4 — the display
/// band 40/56/60/64 (×67!)/72/80 → `display` (nearest step, ties UP,
/// clamped at the ends). Two
/// ternary windows kept a SECOND literal alive after their first number
/// migrated (`compact ? … : 48` and `… : 14`) and were closed by a counted
/// second pass. Five files with no import path to `app_theme` received one
/// before their slice ran. Migration and census shared ONE implementation of
/// the gate's host/own-level/value-window rules, so "sites migrated" ≡
/// "sites counted" — verified by the census reading 0.
///
/// A cap only ever moves DOWN; that is the whole mechanism (a lens getting
/// STRONGER re-measures and says so out loud — that is what entry 10 is).
const _frozenCaps = <String, int>{
  'spacing': 1823,
  'contentDimension': 145,
  'frozenExtent': 45,
  'iconSize': 0,
  'strokeWidth': 126,
  'lineMetric': 48,
  'shadow': 27,
  'defaultedLiteral': 3,
};

/// The one `gap` count the two sizing categories were split out of. The split may
/// only MOVE sites, never create them: if `spacing + contentDimension` exceeds
/// this, the refactor grew the backlog it was cut from.
const _gapTotalBeforeSplit = 2075;

/// Slices that are DONE. Each path must read zero findings forever; the lock is
/// by path (not by category) because a finished file is the unit of work.
///
/// The list is currently EMPTY: its only member, the link picker's tab label,
/// lived in the orphaned `link_picker/` island that was purged with the dead
/// `LinkPickerModal` (bottom-sheet foundation convergence). An empty lock is
/// tolerated here because the file it guarded no longer exists.
const _slicesLockedToZero = <String>[];

/// Files locked to zero in ONE category only (`frozenExtent`), for a fix that
/// must not be undone even though the file still holds other categories.
///
/// The whole-file lock above cannot express this: the toolbar image reads the
/// ladder for its icons and still carries a couple of sizing literals, so
/// demanding zero everywhere would be a lie about the file. What must never come
/// back is the specific disease — a width or height literal promised to content
/// the ladder measures. Locking it per PATH+CATEGORY keeps the reward exact:
/// re-adding `height: 60` there fails this lock immediately, and the lock list is
/// asserted non-empty because a lock that has never held anything cannot be
/// trusted.
const _frozenExtentLockedToZero = <String>[
  // The regression site itself: its `height: 60` was hand-summed and died by
  // 3 px the moment the foundation moved a type step. The box is now sized by
  // its content and roles; a literal extent here is a return of the disease.
  'lib/domains/social/content/presentation/widgets/content_toolbar_widget.dart',
  // Slice 1 of the triage — the wilayah field family. Twenty of its twenty-one
  // `height: 50` promises lived here, most of them inside four private copies of
  // `DropdownStateBuilders` that `city`/`district`/`village` kept while
  // `province_dropdown` used the shared one. The row is now content-driven in ONE
  // place (its padding mirrors the live field's `contentPadding`), and the
  // private copies were purged. These files still hold other categories
  // (`SizedBox(height: 8)` and friends), which is exactly why the lock is
  // per-path + per-category instead of whole-file.
  'lib/shared/widgets/wilayah/dropdown_state_builders.dart',
  'lib/shared/widgets/wilayah/city_dropdown.dart',
  'lib/shared/widgets/wilayah/district_dropdown.dart',
  'lib/shared/widgets/village_dropdown.dart',
  // Slice 2 — the media strip. Its tiles carry their own extent through one
  // named `_tileExtent`, and the strip's height reads the same name, so a taller
  // thumbnail can no longer be clipped by a strip that kept the old number. The
  // file still holds other categories (a `size: 12` with no ladder step to move
  // to), which is why the lock is per-category.
  'lib/shared/widgets/pending_media_strip.dart',
  // Slice 4 — the drag handle, now `AppDragHandle` beside the bottom-sheet base
  // and consumed by every sheet that used to spell 40×4 itself.
  //
  // Be precise about WHAT these locks guard. A reintroduced handle copy is
  // childless, so this lens rightly ignores it; it is caught by the
  // `contentDimension` cap instead (proved by planting one: `contentDimension:
  // 178 > cap 177`). What the lock below forbids is any OTHER frozen box that
  // would have to CONTAIN something — the kind of site this lens exists for.
  // The surviving files read zero frozen extents here (the last one,
  // `coordinate_preview_modal`, lost its `height: 180` map preview to
  // `AppContentSize.preview` in slice 8) and are locked by path.
  'lib/domains/finance/transaction/payment/presentation/widgets/'
      'payment_method_picker_sheet.dart',
  'lib/domains/social/comment/presentation/widgets/commerce_resource_picker.dart',
  'lib/domains/social/share/presentation/widgets/share_bottom_sheet.dart',
  'lib/domains/system/support/presentation/widgets/pre_chat_form_sheet.dart',
  'lib/domains/user/profile/presentation/widgets/address_form_dialog.dart',
  'lib/shared/widgets/create_content_bottom_sheet.dart',
  'lib/shared/widgets/app_bottom_sheet_base.dart',
  'lib/domains/commerce/catalog/auction/presentation/widgets/detail/'
      'auction_action_modal.dart',
  'lib/domains/commerce/catalog/auction/presentation/widgets/detail/'
      'auction_claim_shipping_modal.dart',
  'lib/shared/widgets/language_selector.dart',
  'lib/shared/widgets/theme_selector.dart',
  // Slice 5 — the notification filter rail. It still holds `spacing` and
  // `iconSize` literals (5 + 3), so it is locked per-category rather than whole
  // file: what must not come back is a frozen extent around that rail.
  'lib/domains/system/notification/presentation/screens/'
      'notification_list_screen.dart',
  // Slice 6 — controls, migrated onto the content-size policy
  // (`AppContentSize`) the same day it landed. These files still hold other
  // categories (raw icon `size:`s, spacing literals), so the lock stays
  // per-path + per-category: what must not come back is a raw width/height
  // promised to ladder-measured content.
  'lib/domains/commerce/catalog/for_sale/presentation/screens/'
      'for_sale_detail_screen.dart',
  'lib/domains/commerce/negotiation/negotiation/presentation/widgets/'
      'negotiation_proposal_card.dart',
  'lib/domains/commerce/transaction/checkout/presentation/widgets/'
      'checkout_action_bar.dart',
  // Bottom-action-bar convergence: the dead `ActionButtons` helper was purged
  // and every bar reads the canonical foundation, whose spinner pair is ONE
  // named policy (`_spinnerExtent`/`_spinnerStroke`), never a literal.
  'lib/shared/widgets/bottom_action_bar.dart',
  'lib/domains/user/preference/onboarding/presentation/screens/'
      'welcome_screen.dart',
  'lib/features/home/presentation/widgets/main_app_bar.dart',
  'lib/domains/social/content/presentation/widgets/create_content/'
      'content_type_visibility_header.dart',
  'lib/domains/user/preference/seller/presentation/screens/'
      'seller_dashboard_screen.dart',
  // Slice 7 — columns and CTAs onto the same ladder.
  'lib/features/home/presentation/screens/home_screen.dart',
  'lib/domains/commerce/transaction/order/presentation/screens/'
      'order_list_screen.dart',
  'lib/domains/chat/chat/presentation/screens/chat_list_screen.dart',
  'lib/domains/commerce/transaction/order/presentation/screens/order_detail/'
      'dispute_escalation_dialog.dart',
  'lib/domains/user/profile/presentation/screens/profile_screen/'
      'profile_about_tab.dart',
  'lib/domains/user/profile/presentation/widgets/profile_info_row.dart',
  'lib/domains/user/profile/presentation/widgets/'
      'bank_account_card_widget.dart',
  'lib/domains/user/preference/seller/presentation/widgets/wizard/'
      'seller_wizard_preview_widget.dart',
  'lib/domains/user/profile/presentation/widgets/profile_reviews_tab/'
      'rating_overview_section.dart',
  // Slices 8–9 — media/previews onto `AppContentSize`, surfaces onto the
  // ladder, and the two structural repairs (dropzone and helper reserve went
  // content-driven; the wheels and the wizard clamp named component-local
  // policy). With these the category reads ZERO app-wide: the cap above is
  // now a hard gate — any frozen box anywhere fails it.
  'lib/domains/commerce/catalog/auction/presentation/widgets/detail/'
      'auction_recommendations_section.dart',
  'lib/domains/system/support/presentation/widgets/'
      'suggested_messages_widget.dart',
  'lib/domains/user/preference/seller/presentation/screens/'
      'seller_verification_screen.dart',
  'lib/shared/widgets/media_grid_uploader.dart',
  'lib/shared/widgets/mentions/mention_text_field.dart',
  'lib/shared/widgets/coordinate_preview_modal.dart',
  'lib/domains/user/identity/authentication/presentation/widgets/'
      'username_field.dart',
  'lib/shared/widgets/web_image_cropper.dart',
  'lib/shared/widgets/wizard_progress_indicator.dart',
];

/// The anti-vacuum floor for the sweep: the census has to read the real app,
/// not a subtree that happens to be clean.
const _censusFileFloor = 1000;

int _count(String category, String source) =>
    geometryViolationsIn(source, path: 'probe.dart')[category]!.length;

/// The whole sizing family (both halves of the split), used to prove the
/// detectors spare something without pinning which half sees it.
int _sizing(String source) =>
    _count('spacing', source) + _count('contentDimension', source);

String _sorted(Map<String, int> census) {
  final keys = census.keys.toList()..sort();
  return '{${keys.map((key) => '$key: ${census[key]}').join(', ')}}';
}

void main() {
  group('the geometry census', () {
    test('it sweeps the real app and cannot go vacuous', () {
      expect(
        geometryCensusFiles().length,
        greaterThan(_censusFileFloor),
        reason:
            'the census found no Dart files — it is reading the wrong tree, '
            'not observing a finished migration',
      );
      final census = geometryCensus();
      expect(
        census.keys.toList()..sort(),
        (geometryCategories.toList()..sort()),
        reason: 'a category silently disappeared from the census',
      );

      // A category that still has a backlog must still be SEEN: if the detector
      // dies, the cap below would pass by reading nothing. Once a category
      // reaches zero its floor retires with it.
      for (final entry in _frozenCaps.entries) {
        if (entry.value == 0) continue;
        expect(
          census[entry.key],
          greaterThan(0),
          reason:
              '${entry.key} is capped at ${entry.value} but the detector found '
              'nothing — the detector is broken, not the backlog solved',
        );
      }
    });

    test('no geometry category grows past its frozen cap', () {
      final census = geometryCensus();
      final offenders = <String>[];
      for (final entry in census.entries) {
        final cap = _frozenCaps[entry.key];
        if (cap == null) {
          offenders.add(
            '${entry.key} is a NEW geometry category (${entry.value})',
          );
          continue;
        }
        if (entry.value > cap) {
          offenders.add('${entry.key}: ${entry.value} > cap $cap');
        }
      }
      expect(
        offenders,
        isEmpty,
        reason:
            'a raw geometry literal joined the backlog. The app already names '
            'these steps (AppMetrics for spacing, AppShape for radii, '
            'AppElevation for shadow); migrate the call site or lower the cap in '
            'the same commit as the migration:\n'
            'census: ${_sorted(census)}\n${offenders.join('\n')}',
      );
    });

    test('the split only moved sites, it did not create them', () {
      final census = geometryCensus();
      expect(
        census['spacing']! + census['contentDimension']!,
        lessThanOrEqualTo(_gapTotalBeforeSplit),
        reason:
            'splitting one `gap` cap into spacing + contentDimension must not '
            'raise the backlog it was cut from',
      );
    });

    test('finished slices stay at zero', () {
      // The lock list may be empty: its last member lived in the `link_picker/`
      // island that was purged with the dead `LinkPickerModal`. Locking by path
      // is still the contract for whatever slice is added next.
      final offenders = <String>[];
      for (final path in _slicesLockedToZero) {
        final counts = geometryViolationsInFile(path);
        if (counts.isEmpty) {
          offenders.add('$path does not exist');
          continue;
        }
        final total = counts.values.fold(0, (sum, count) => sum + count);
        if (total > 0) {
          offenders.add('$path regained ${_sorted(counts)}');
        }
      }
      expect(
        offenders,
        isEmpty,
        reason:
            'a migrated slice grew a geometry literal back. Finished files '
            'read the ladder (`AppMetrics`/`AppShape`), never a number:'
            '\n${offenders.join('\n')}',
      );
    });

    test(
      'the toolbar that shipped the regression keeps zero frozen extents',
      () {
        expect(
          _frozenExtentLockedToZero,
          isNotEmpty,
          reason:
              'a per-category lock that holds nothing has never proven it can '
              'hold anything',
        );
        for (final path in _frozenExtentLockedToZero) {
          final counts = geometryViolationsInFile(path);
          expect(counts, isNotEmpty, reason: '$path does not exist');
          expect(
            counts['frozenExtent'],
            0,
            reason:
                'a frozen literal extent came back to $path. This is the file '
                'where `height: 60` died by 3 px when a type step moved: an '
                'extent is driven by its content and the roles, never by a '
                'summed-up literal.',
          );
        }
      },
    );
  });

  group('the geometry detectors (negative proof)', () {
    test('they fire on the spellings that used to slip through', () {
      // Each of these is a real shape found in the app, not a hypothetical.
      expect(_count('spacing', 'const SizedBox(height: 16),'), 1);
      expect(
        _count('spacing', 'SizedBox(\n  width: 24,\n)'),
        1,
        reason: 'a literal on the following line is the same call site',
      );
      expect(_count('iconSize', 'Icon(Icons.x, size: 20)'), 1);
      expect(_count('strokeWidth', 'Border.all(width: 1)'), 1);
      expect(_count('strokeWidth', 'Divider(thickness: 2)'), 1);
      expect(_count('lineMetric', 'TextStyle(letterSpacing: 0.5)'), 1);
      expect(_count('lineMetric', 'TextStyle(height: 1.1)'), 1);
      expect(_count('shadow', 'BoxShadow(blurRadius: 10)'), 1);
      expect(_count('shadow', 'BoxShadow(offset: Offset(0, 2))'), 1);
      // The `??` dodge: a regex that only looks for a digit right after
      // `circular(` cannot see this radius decision at all.
      expect(
        _count(
          'defaultedLiteral',
          'borderRadius: BorderRadius.circular(borderRadius ?? 12),',
        ),
        1,
      );
    });

    test('the split tells empty space from a box that IS something', () {
      // The line the split draws: one axis, no child, no paint = a gap.
      expect(_count('spacing', 'SizedBox(height: 16)'), 1);
      expect(_count('contentDimension', 'SizedBox(height: 16)'), 0);

      expect(_count('contentDimension', 'SizedBox(width: 280, child: btn)'), 1);
      expect(_count('spacing', 'SizedBox(width: 280, child: btn)'), 0);

      expect(_count('contentDimension', 'Container(width: 64, height: 64)'), 1);
      expect(_count('spacing', 'Container(width: 64, height: 64)'), 0);

      // Real shapes from the app: a bottom-sheet drag handle and a hairline
      // divider are drawn, so they are content geometry, not gaps.
      expect(
        _count(
          'contentDimension',
          'Container(width: 40, height: 4, decoration: d)',
        ),
        1,
      );
      expect(_count('contentDimension', 'Container(height: 1, color: c)'), 1);
      expect(_count('spacing', 'Container(height: 1, color: c)'), 0);
    });

    test('a nested property belongs to the nested call, not to the box', () {
      // The `width: 1` is the stroke's decision. Before own-level scanning the
      // box was also counted as a sizing site because of it — one decision,
      // counted twice, under the wrong category.
      const card =
          'Container(padding: p, child: Text(x), decoration: '
          'BoxDecoration(border: Border.all(width: 1)))';
      expect(_count('spacing', card), 0);
      expect(
        _count('contentDimension', card),
        0,
        reason:
            'the box decides no dimension of its own — only the border does',
      );
      expect(_count('strokeWidth', card), 1);
    });

    test('one site is counted once, never twice', () {
      // A host that contains another host is not the innermost decision; the
      // inner block is scanned on its own.
      expect(
        _count('spacing', 'SizedBox(height: 8, child: SizedBox(height: 4))'),
        1,
        reason: 'the outer box delegates to the inner one',
      );
      // One `SizedBox` with two decided axes is one thing to migrate — and it is
      // a box, not a gap.
      expect(_count('contentDimension', 'SizedBox(width: 8, height: 8)'), 1);
      expect(_count('spacing', 'SizedBox(width: 8, height: 8)'), 0);
    });

    test('they spare the ladders, computed geometry and prose', () {
      expect(_sizing('SizedBox(height: AppMetrics.p16)'), 0);
      expect(_sizing('SizedBox(height: _getSize())'), 0);
      expect(_sizing('SizedBox(height: core.AppMetrics.p16)'), 0);
      expect(_count('iconSize', 'Icon(Icons.x, size: AppIconSize.action)'), 0);
      expect(
        _count(
          'strokeWidth',
          'Border.all(width: AppMetrics.focusedBorderWidth)',
        ),
        0,
      );
      expect(_count('lineMetric', 'TextStyle(height: bodyHeight)'), 0);
      expect(_count('shadow', 'BoxShadow(blurRadius: step.blur)'), 0);
      expect(
        _count('shadow', 'BoxShadow(offset: Offset(0, size * 0.1))'),
        0,
        reason:
            'proportional geometry is not a step decision — a value window '
            'that stops at the comma inside `Offset(` reports it as one',
      );
      expect(_sizing('SizedBox(width: screenWidth * 0.5)'), 0);
      expect(_count('defaultedLiteral', 'borderRadius: r ?? 0,'), 0);
      expect(
        _sizing('/// SizedBox(height: 16)'),
        0,
        reason: 'prose about a ladder is not a call site',
      );
    });

    test('they fire on a frozen extent holding ladder-measured content', () {
      // The shape that shipped a real 3 px overflow on 2026-10-02: the box
      // promised a literal height while the ladder decided how tall the content
      // inside it came out. The literal never changed; the ladder moved.
      const toolbar =
          'Container(\n'
          '  height: 60,\n'
          '  padding: const EdgeInsets.symmetric(vertical: AppMetrics.p8),\n'
          '  child: Row(children: [Text(t, style: context.typeRoles.labelMicro)]),\n'
          ') ';
      expect(_count('frozenExtent', toolbar), 1);
      expect(
        _count('contentDimension', toolbar),
        1,
        reason:
            'the same site carries both lenses — "not on a ladder" and "no '
            'slack budget" are different questions about one box',
      );
      // Both axes are promises, and both the icon ladder and the role view of
      // the type ladder count as evidence that the content is ladder-measured.
      expect(
        _count('frozenExtent', 'SizedBox(width: 200, child: f(AppMetrics.p8))'),
        1,
      );
      expect(
        _count(
          'frozenExtent',
          'Container(height: 40, decoration: d, child: c(AppIconSize.action))',
        ),
        1,
      );
      expect(
        _count(
          'frozenExtent',
          'Container(height: 40, child: c(context.typeRoles.labelMicro))',
        ),
        1,
      );
    });

    test('the frozen-extent lens spares what the ladder already owns', () {
      expect(
        _count(
          'frozenExtent',
          'Container(height: AppMetrics.p48, padding: e, child: c(AppMetrics.p8))',
        ),
        0,
        reason: 'the extent itself is a step — nothing about it is frozen',
      );
      expect(
        _count(
          'frozenExtent',
          'Container(height: size * 0.5, child: c(AppMetrics.p8))',
        ),
        0,
        reason: 'proportional geometry is not a frozen promise',
      );
      expect(
        _count('frozenExtent', 'SizedBox(height: 16)'),
        0,
        reason: 'an empty gap has nothing inside to overflow',
      );
      expect(_count('frozenExtent', 'Container(height: 1, color: c)'), 0);
      expect(
        _count(
          'frozenExtent',
          'Container(height: 1, margin: EdgeInsets.symmetric('
              'horizontal: AppMetrics.p24), decoration: d)',
        ),
        0,
        reason:
            'a CHILDLESS divider cannot overflow — nothing has to fit inside '
            'it, so its size is a contentDimension decision, not a budget. A '
            'ladder token in the margin must not launder it into this lens.',
      );
      expect(
        _count(
          'frozenExtent',
          'Container(height: 40, child: Text(t, style: TextStyle(color: koi)))',
        ),
        0,
        reason: 'a colour decides no extent, so it is not ladder-measured size',
      );
      expect(
        _count(
          'frozenExtent',
          '/// Container(height: 60, child: c(AppMetrics.p8))',
        ),
        0,
        reason: 'prose about a ladder is not a call site',
      );
    });
  });
}
