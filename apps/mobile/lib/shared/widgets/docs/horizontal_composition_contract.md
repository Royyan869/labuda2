# Horizontal Composition Contract

**Status:** LOCKED (contract only — no widget)
**Scope:** Mobile UI foundation (`apps/mobile/lib`)
**Enforced by:** `test/shared/widgets/horizontal_composition_contract_test.dart`

---

## 0. One line

Horizontal space in a **bounded** row is distributed as: *bounded/fixed leading
and trailing content stays non-flexible; the dynamic textual/contentual child
receives the remaining width through `Expanded`/`Flexible`; dynamic text
declares an explicit text strategy.* The authority remains the Flutter
primitives — there is **no Labuda layout widget**.

This is not a style preference. It is the only arrangement that cannot overflow
when dynamic content is longer than expected, at any supported width or text
scale.

---

## 1. Why `Expanded`/`Flexible` is in the contract (framework proof)

A `Row` lays out its **non-flexible** children with unbounded main-axis
constraints: each takes its intrinsic width. `Expanded`/`Flexible` children then
split whatever space is **left**. Consequently:

- a non-flexible child with a **bounded** intrinsic width (an icon, an avatar, a
  badge, a fixed-label button) is safe;
- a non-flexible child with **unbounded dynamic** width (a `Text` of unknown
  length, a name, a title) can exceed the row and produce a `RenderFlex`
  overflow;
- a dynamic child inside `Expanded`/`Flexible` is clamped to the available width
  and cannot overflow the row — but it must then be told **what to do with the
  excess** (see §2).

`Expanded` = "take exactly your share"; `Flexible` = "take at most your
intrinsic width, but shrink to your share if needed". Both are correct; the
choice depends on whether the child should fill or hug.

---

## 2. Dynamic text chooses its strategy by semantics

| Semantic intent | Strategy |
|---|---|
| one-line/short label, overflow should truncate | `maxLines: n` + `overflow: TextOverflow.ellipsis` |
| body copy, overflow should wrap | default wrapping (`softWrap: true`), no `maxLines` |
| a variable **set** (chips, badges) that may occupy several lines | `Wrap` |
| a bounded **visual group** whose semantics are scale-to-fit | `ConstrainedBox(maxWidth:)` + `FittedBox(scaleDown:)` |

The strategy is a **semantic** decision, not a layout trick. Do not pick
`ellipsis` for text a user must read in full; do not wrap a value the design
requires on one line.

---

## 3. Two-dynamic-end rows

When **both** ends can grow dynamically (e.g. `label … value`, `title …
action`):

- at least one side must be `Expanded`/`Flexible`;
- the semantic choice of **which** side yields — truncates, wraps, or stacks —
  must be **explicit**;
- **do not invent a universal truncate/wrap/stack semantic**. Different
  surfaces legitimately differ.

> **UNRESOLVED (Owner decision):** the canonical label/value semantic
> (truncate vs wrap vs stack) is **not** locked here.
> `CommerceDetailLabelValue` is an existing correct implementation, not a
> declared cross-domain authority.

---

## 4. Viewport width

Do **not** use raw `MediaQuery.size.width` arithmetic (and especially not
`MediaQuery.of(context).size.width * fraction` or `.width - constant`) as a
generic horizontal layout authority. Layout should respond to the **incoming
constraints** (`LayoutBuilder` / the parent's box), not to the raw viewport.

> **NOT migrated here.** Existing occurrences are known residue and are a
> separate bounded task.

---

## 5. Wrap

`Wrap` is canonical when the content is a **variable set** that may legitimately
occupy multiple lines/rows — badges, chips, tag lists, quick-amount buttons.
Do **not** replace valid `Wrap` usage with `Row`.

---

## 6. FittedBox

`FittedBox` is **not** banned. A bounded `FittedBox(fit: BoxFit.scaleDown)`
inside a `ConstrainedBox(maxWidth:)` is legitimate when the content is a bounded
visual asset/group whose semantic behaviour is **scale-to-fit** (e.g.
`PaymentMethodLogo`'s brand marks). It is not a general text-overflow
mechanism.

---

## 7. What is explicitly NOT required

The contract is a **rule for behaviour**, not an AST shape. The following are
**completely canonical and require no change**:

- `Row` containing only bounded children (`Icon + short label`, `Text + Spacer +
  bounded trailing action`, fixed controls) with **no** `Expanded`/`Flexible`;
- `Row` whose every child is intrinsically bounded;
- bounded badges / chips;
- `Wrap` for variable sets;
- `ConstrainedBox(maxWidth:) + FittedBox(scaleDown:)` for bounded visual groups;
- `Spacer` between two bounded ends.

The test gate proves **behaviour** (no overflow across a width × text-scale
matrix) and does **not** assert "every `Row` must contain `Expanded`".

---

## 8. Do not infer

Future changes must **not** conclude from this document that:

- every `Row` needs an `Expanded`/`Flexible`;
- `FittedBox` is forbidden;
- `Wrap` should become `Row`;
- all label/value rows must truncate (or wrap, or stack) identically;
- the `_InfoRow` family, checkout rows, or any individual consumer should be
  migrated by this contract (they are separate bounded tasks);
- viewport arithmetic must be purged by this contract (separate task).

This document establishes **one invariant** — *dynamic content sits in a flex
child with a declared text strategy; fixed/bounded content may stay
non-flexible* — and one acceptance gate. Nothing else.
