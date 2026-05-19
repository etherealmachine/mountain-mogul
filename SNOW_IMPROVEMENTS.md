# Snow system — improvement notes

These notes capture a design discussion about replacing the current continuous
`Packed`/`Ice` layer attributes with a discrete snow-type model. Nothing here
is implemented yet.

## Problem with the current model

`SnowLayer` carries `Packed` (0..1) and `Ice` (0..1) as independent continuous
scalars. This is flexible but has two issues:

1. "Type" is emergent and hard for the player to read — they see numbers, not
   concepts like "wind slab" or "boilerplate."
2. The weather-to-layer transition function needs to move two independent
   scalars, which makes it harder to design in a way that players can predict.

## Proposed model: discrete type + accumulation

Each `SnowLayer` has:

- `Kind SnowLayerKind` — one of ~7 discrete types (see below)
- `Accumulation float32` — SWE metres, conserved as before

`Packed` and `Ice` are removed. Physics constants (friction, speed, etc.) are
looked up per `Kind` rather than derived from continuous scalars.

`Grooming` and `MogulSize` remain as cell-level surface modifiers — they are
not depositional events and belong on `Cell`, not in a layer.

## Snow types

| Kind | Description | Skier effect |
|---|---|---|
| Powder | Cold, dry, light | Deep and floaty; slow but fun; tiring in quantity |
| Packed Powder | Groomed or skied-in | Fast, predictable, good grip |
| Cement | Warm storm, dense and wet | Heavy, slow, leg-burning |
| Wind Slab | Wind-consolidated | Hollow-feeling; can shatter into chunks |
| Crust | Sun or wind surface glaze | Breakable top over softer snow below; edge-catching |
| Boilerplate | Hard frozen surface | Very fast; hard to hold an edge; dangerous |
| Slush / Corn | Saturated or freeze-thaw | Suction-y and slow when warm; smooth spring corn when right temp |

Champagne Powder vs. Powder is not a separate type — depth (visible from
`Accumulation`) communicates that distinction. Crud is a traffic effect (skier
wear breaking up Powder or Packed Powder), not a weather-deposited type.

## Weather and layer operations

Weather affects layers in two distinct ways:

### 1. Snowfall — push a new layer

When it snows, a new layer is pushed on top of the stack. The kind of the new
layer is determined solely by temperature at storm time:

| Weather event | New layer kind |
|---|---|
| Light or heavy snow, cold (< −5°C) | Powder |
| Light or heavy snow, moderate (−5 to −1°C) | Packed Powder |
| Light or heavy snow, warm (−1 to 0°C) | Cement |

No transition of existing layers happens during snowfall.

### 2. Non-precipitation weather — modify the top layer in place

The surface layer's kind transitions based on the weather event. This is the
core transition matrix:

| Current kind \ Event | Rain | Cold clear | Warm clear | Wind |
|---|---|---|---|---|
| Powder | Cement | Powder | Packed Powder | Wind Slab |
| Packed Powder | Cement | Crust | Slush/Corn | Wind Slab |
| Cement | Cement | Boilerplate | Slush/Corn | Wind Slab |
| Wind Slab | Cement | Boilerplate | Slush/Corn | Wind Slab |
| Crust | Cement | Boilerplate | Slush/Corn | Wind Slab |
| Boilerplate | Cement | Boilerplate | Slush/Corn | Boilerplate |
| Slush/Corn | Slush/Corn | Boilerplate | Slush/Corn | Wind Slab |

Key patterns:
- **Rain** always wets the surface → Cement (or stays Cement/Slush if already wet).
- **Cold clear** hardens whatever is there → Crust or Boilerplate.
- **Warm clear** softens → Slush/Corn (spring freeze-thaw cycles land here).
- **Wind** consolidates → Wind Slab, regardless of current type (except
  Boilerplate, which is already hard).
- **Overcast** — no change (holding pattern).

## Conceptual framing: Density + Moisture

Before settling on the discrete type approach, we considered continuous
`Density` and `Moisture` attributes:

- **Density** ← temperature at snowfall time (cold = light, warm = heavy)
- **Moisture** ← liquid water input (rain/melt raises it; refreeze and
  sublimation lower it)

Refreeze is the key transition: moisture converts to ice bonds, raising density
and lowering moisture simultaneously — this is what produces Crust, Boilerplate,
and Corn depending on cycle count.

The discrete type model captures the same structure but trades granularity for
predictability. The player learns one matrix rather than two interacting scalars.
`Accumulation` remains continuous so depth and SWE variation are still present.

## Open questions

- Should `Accumulation` be able to partially transfer between layers when the
  top layer transitions (e.g. heavy rain merges top two layers)?
- What is the exact temperature boundary between "cold clear" and "warm clear"?
  Currently 0°C is the obvious split but the DayWeather `TempC` field already
  carries this continuously.
- How many layers should the stack be capped at before merging the two oldest?
  8–12 is a reasonable range.
- Weather currently lacks an explicit wind state. Wind slab formation could
  happen stochastically within Clear/Overcast days, or wind could become a
  first-class `WeatherState`.
