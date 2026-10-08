// Interactive map picker height/inset authority contract.
//
// The picker keeps a legitimate bespoke modal route (map pan/pinch conflicts
// with sheet drag, so `enableDrag: false` on a raw `showModalBottomSheet`),
// but its finite height must come from the canonical usable-height ceiling
// ([AppBottomSheetBase.availableHeight]: window minus keyboard minus system
// top inset) — never from a raw screen-height fraction. The picker body owns
// no keyboard/inset math of its own: lift inside the body stays owned by
// `BottomActionBar`.
//
// The real `GoogleMap` plugin cannot run in widget tests, so this file pins
// the source contract (same style as the bottom-sheet authority gates) while
// behavioral proof of `availableHeight` itself lives in
// `test/core/theme/bottom_inset_authority_contract_test.dart`.
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

const _pickerPath =
    'lib/shared/widgets/interactive_map_picker_bottom_sheet.dart';

String _source() => File(_pickerPath).readAsStringSync();

void main() {
  group('map picker height authority', () {
    test('finite height derives from the canonical available-height ceiling',
        () {
      final source = _source();
      expect(
        source.contains('AppBottomSheetBase.availableHeight(context)'),
        isTrue,
        reason:
            'the picker body height must read the canonical usable-height '
            'ceiling, not raw screen metrics',
      );
    });

    test('no raw screen-height sheet calculation remains', () {
      final source = _source();
      for (final raw in <String>[
        'MediaQuery.of(context).size.height',
        'MediaQuery.sizeOf(context).height',
        'size.height * 0.9',
      ]) {
        expect(
          source.contains(raw),
          isFalse,
          reason: 'raw screen-height height authority ($raw) must be gone',
        );
      }
    });

    test('no duplicate keyboard/top-inset arithmetic in the picker', () {
      final source = _source();
      for (final dup in <String>[
        'viewInsets.bottom',
        'padding.top',
      ]) {
        expect(
          source.contains(dup),
          isFalse,
          reason:
              'keyboard/top-inset math ($dup) belongs to the canonical '
              'ceiling and BottomActionBar, not the picker body',
        );
      }
    });

    test('legitimate bespoke route is preserved', () {
      final source = _source();
      expect(
        source.contains('showModalBottomSheet'),
        isTrue,
        reason: 'the picker keeps its own modal route (not Base.show)',
      );
      expect(
        source.contains('enableDrag: false'),
        isTrue,
        reason: 'map pan/pinch must never fight sheet drag',
      );
    });
  });
}
