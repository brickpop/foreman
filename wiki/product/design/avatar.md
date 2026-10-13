---
type: spec
status: planned
tags: [design, ui, agents]
---

# Avatar

Every agent has a generated face, drawn as a small SVG from the `look` of its [agent soul](../models/agent-soul.md). The same face appears everywhere the agent does: tabs, panels, rows, the agent page. An optional corner **dot** shows the instance status and an optional **ring** highlights it. Reference: `Avatar.dc.html` in the [prototype](./prototype/foreman-prototype.dc.html) folder.

## Inputs

| Input | Type | Default | Notes |
|---|---|---|---|
| `size` | px | 32 | Square |
| `color` | CSS color | `#8E9AAF` | Background, one of the background palette |
| `face.skin` | CSS color | `#E2AE85` | |
| `face.hair` | enum | `short` | `short`, `long`, `curly`, `bun`, `bald` |
| `face.hairColor` | CSS color | `#2B2420` | Also used for beard and mustache |
| `face.eyes` | enum | `dots` | `dots`, `glasses`, `shades`, `sleepy` |
| `face.beard` | enum | `none` | `none`, `stubble`, `full`, `mustache` |
| `dot` | CSS color | none | Status dot color; none = no dot |
| `dotBorder` | CSS color | `var(--bg)` | Must match the surface behind the avatar (`--panel`, `--side`, `--term`, …) |
| `ring` | CSS color | none | Inset ring color; none = no ring |

## Palettes

| Trait | Options |
|---|---|
| Background (8) | `#D9825B` `#8DB86B` `#6FA8DC` `#B48BD6` `#E07A9A` `#4FB3A9` `#CDB46B` `#8E9AAF` |
| Skin (5) | `#F1C9A5` `#E2AE85` `#C38A62` `#8D5A3B` `#5E3B26` |
| Hair color (5) | `#2B2420` `#6B4A2F` `#D8B46A` `#A8A8A8` `#A4482C` |
| Hair | `short` `long` `curly` `bun` `bald` |
| Eyes | `dots` `glasses` `shades` `sleepy` |
| Beard | `none` `stubble` `full` `mustache` |

A new agent gets a random face and background ([Chief of Staff](../features/chief-of-staff.md) hire). **Shuffle** on the [Agent page](../features/agent-page.md) randomizes all traits and the background.

## Drawing

A `size × size` box (inline-block, never shrinks) holding an SVG with `viewBox="0 0 32 32"`, `border-radius: round(size × 0.24)px`, overflow hidden. Layers, bottom to top:

1. Background: `rect` 32×32, fill `color`.
2. **Long hair (back)** when hair = `long`: `M7 16 C7 5.5 25 5.5 25 16 L25.6 27 L6.4 27 Z`, fill hairColor.
3. **Shoulders** (always): `M4.5 33 C4.5 25.6 27.5 25.6 27.5 33 Z`, fill `rgba(20,20,20,0.28)`.
4. **Face**: ellipse `cx=16 cy=16.6 rx=7.4 ry=8.4`, fill skin.
5. **Hair top** when hair ∈ {`short`, `long`, `bun`}: `M8.4 15.2 C7.6 6.8 24.4 6.8 23.6 15.2 C22.6 11.6 19.2 10.6 16 10.6 C12.8 10.6 9.4 11.6 8.4 15.2 Z`, fill hairColor.
6. **Bun** when hair = `bun`: circle `cx=16 cy=6.2 r=3`, fill hairColor.
7. **Curly** when hair = `curly`: circles (9.6, 12.6, r 2.6), (12, 9.6, r 2.9), (16, 8.4, r 3.1), (20, 9.6, r 2.9), (22.4, 12.6, r 2.6), fill hairColor.
8. **Full beard**: `M8.7 17.5 C9.2 24.5 12.8 26.6 16 26.6 C19.2 26.6 22.8 24.5 23.3 17.5 C22.2 21.6 19.6 23.2 16 23.2 C12.4 23.2 9.8 21.6 8.7 17.5 Z`, fill hairColor. **Stubble** is the same path with `fill-opacity: 0.32`.
9. **Eyes**, all `#1A1A1A`:
   - `dots`: circles at (13, 16.4) and (19, 16.4), r 1.1.
   - `glasses`: the dots, plus circles r 2.6 at the same centers, fill `rgba(255,255,255,0.18)`, stroke `#1A1A1A` 0.9, and a bridge `M15.6 16.2 L16.4 16.2`.
   - `shades`: rects (10.1, 14.9, 5.4×3, rx 1.3) and (16.5, 14.9, 5.4×3, rx 1.3), and a bridge rect (15.4, 15.6, 1.2×0.7).
   - `sleepy`: arcs `M11.7 16.5 Q13 17.4 14.3 16.5` and `M17.7 16.5 Q19 17.4 20.3 16.5`, stroke 0.9, round caps, no fill.
10. **Mustache** when beard = `mustache`: `M13.4 20.3 C14.6 19.2 15.6 19.7 16 20.1 C16.4 19.7 17.4 19.2 18.6 20.3 C17.4 21 16.6 20.7 16 20.5 C15.4 20.7 14.6 21 13.4 20.3 Z`, fill hairColor.
11. **Mouth** (always): `M14.3 21.4 Q16 22.7 17.7 21.4`, stroke `rgba(40,20,10,0.65)` 0.9, no fill.

`bald` draws no hair layer.

## Ring and dot

One meaning per mark: the **dot is the instance status**, everywhere a status is shown; the **ring is a "needs you" highlight** only (amber, on the requesting agent in request panels). The ring never encodes status.

- **Ring**: an overlay with the avatar's radius and `box-shadow: inset 0 0 0 max(1.5, size/15)px <ring>` (one decimal).
- **Dot**: a circle at `right: -3px; bottom: -3px`, diameter `max(7, round(size × 0.32))px`, background `<dot>`, `box-shadow: 0 0 0 2px <dotBorder>`. The dot color is the instance status color ([Design system](./design-system.md#status-colors)).

## Sizes in use

| Size | Where | Decoration |
|---|---|---|
| 20 | Board card assignee | none |
| 24 | Agent tab (Sessions), Changes request panel | dot on tabs (`dotBorder --side`) |
| 26 | Pane header, In-flight row | dot on pane header (`dotBorder --term`) |
| 28 | Team rosters in lists | none, tooltip `<Name> · <Role>` |
| 32 | Home request panel; project settings member rows | amber ring on Home; dot on settings rows (`dotBorder --panel`) |
| 38 | Team Members tab | none |
| 72 | Agent page header | none |

Elsewhere, status is a separate 7–8px dot next to text, not on the avatar.

## Chief of Staff

The [Chief of Staff](../features/chief-of-staff.md) is shown as a 22–28px amber tile with a "C" (`--acc` background, `--accInk` letter), not a face. When a CoS request appears in a request list, it uses a fixed face: background `#E8A33D`, skin `#E2AE85`, bald, glasses, no beard.
