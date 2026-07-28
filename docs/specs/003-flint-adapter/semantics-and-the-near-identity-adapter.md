# About semantics, the `loc`/`count` split, and the near-identity adapter

An explanation of one design decision in the codelens output envelope (D3, the
shape of the `semantics` map) and what happens to it when the data reaches an
external chart compiler. It exists because the decision looks trivial and is not:
the choice to make a semantic a *bare string* is what makes a chart adapter cheap,
and the `loc`/`count` split is the one place where that cheapness appears to break
down.

This document is for reflection, not for doing. It contains no instructions.

## The two vocabularies

codelens emits, on every result, a flat map from field name to a word describing
what the field *means*. The vocabulary is closed at twelve members, defined in
`internal/analysis/semantics.go`:

```go
const (
 SemanticFilepath       = "filepath"        // repository path, splittable on "/"
 SemanticPerson         = "person"          // actor name (an author, or a team under --team-map)
 SemanticDate           = "date"            // calendar date, YYYY-MM-dd
 SemanticCommitID       = "commit_id"       // opaque commit identifier
 SemanticText           = "text"            // free prose; never a plottable category
 SemanticLabel          = "label"           // categorical name
 SemanticFlag           = "flag"            // boolean
 SemanticCount          = "count"           // tally of things
 SemanticLoc            = "loc"             // line count (a size measure)
 SemanticPercentage     = "percentage"      // integer 0-100
 SemanticRatio          = "ratio"           // float 0-1
 SemanticDurationMonths = "duration_months" // whole calendar months
)
```

Flint has the same idea at a much larger scale: roughly seventy types in a
three-tier registry, where tier 0 is a broad family (Temporal, Measure, Discrete,
Geographic, Categorical, Identifier), tier 1 is a category, and tier 2 is a
specific type. Flint's contract is that the *specific type drives every downstream
decision*: number formatting, whether the axis is anchored at zero, which colour
scheme, which direction the scale runs, what aggregation to apply.

The two vocabularies were designed independently, for the same reason, and they
rhyme. That is not luck. Both are answering the observation that a JSON type is
not enough information to draw a chart: knowing a field is an `int` tells you
nothing about whether zero is meaningful, whether summing it is valid, or whether
it belongs on an axis at all.

## What "near-identity" means, and why anyone cares

An adapter is the code that turns a codelens result into a chart specification.
Flint's input looks like this:

```jsonc
{
  "data": { "values": [ /* rows */ ] },
  "semantic_types": { "region": "Category", "revenue": "Quantity" },
  "chart_spec": {
    "chartType": "Bar Chart",
    "encodings": { "x": { "field": "region" }, "y": { "field": "revenue" } }
  }
}
```

The middle key is the interesting one. `semantic_types` is a flat map from field
name to a type word. codelens `semantics` is a flat map from field name to a
semantic word. If the two vocabularies can be related by a lookup table, then the
adapter's hardest-looking job reduces to:

```python
semantic_types = {field: FLINT[sem] for field, sem in envelope["semantics"].items()}
```

That is what "near-identity" means: a per-field rename with no inspection of the
data, no branching on which analysis produced it, and no domain knowledge in the
adapter. It is *near* identity rather than identity only because the words differ.

This is the payoff that justified the design. Decision D3 in the spec-002 plan
records it explicitly:

> A semantic is a bare closed enum string; range and unit are implied by the name
> and documented once. This is the flat `field -> string` map Flint's
> `semantic_types` expects, keeping the adapter near-identity.

The word doing the load-bearing work there is **bare**. A semantic could have been
an object with a range, a unit, and an aggregation hint. It is deliberately not,
because an object would have to be translated structurally rather than looked up,
and because every field of that object is a promise codelens would then have to
keep across eighteen analyses. Fixing the range and unit *by convention*, once, in
the comment above the enum, pushes that cost from runtime into documentation.

There is a real cost to that choice, and it is worth naming: the constraints are
invisible to a consumer who has not read the comment. Nothing in the envelope says
`percentage` is 0-100 while `ratio` is 0-1. An agent that has only seen the data
must infer it. codelens accepts that in exchange for the flatness, on the grounds
that `schema` output and the documented vocabulary are both available and an agent
is instructed to consult them.

## Why `loc` and `count` are separate

Eight of the twelve semantics are uncontroversial as vocabulary. The four numeric
measures are where the design actually took a position. From D3a:

> Numeric measures split four ways: `count` (tallies), `loc` (line counts),
> `percentage` (0-100 ints), `ratio` (0-1 floats). Separating `loc` from `count`
> is what lets a renderer pick a size channel over a frequency channel without
> domain knowledge.

The comment in the source states the reasoning more sharply:

```go
//   - Count is a tally of things; Loc is a count of LINES. The split is not
//     cosmetic: lines are the conventional size channel of a treemap while
//     frequencies are the colour channel, and a renderer cannot tell them apart
//     from the type alone.
```

Both are non-negative integers. Nothing in the JSON distinguishes them. The
distinction is entirely about *what a chart should do with them*, and it comes
from the crime-scene analysis tradition the tool implements: in a hotspot map, a
file's size is how much code it contains and its colour is how often it changes.
Those are different channels carrying different meanings, and swapping them
produces a picture that is not merely uglier but wrong.

How load-bearing is this really? It is worth looking at the case that proves the
split is not a naming convention in disguise. Two analyses, `main-developer` and
`main-developer-by-revisions`, emit **identical column names** with **different
meanings**:

```go
// internal/analysis/maindev.go
{Name: "added", Type: "int", Semantic: SemanticLoc, Desc: "lines added by the main developer"},
{Name: "total_added", Type: "int", Semantic: SemanticLoc, Desc: "total lines added to the entity by all authors"},
```

```go
// internal/analysis/maindevbyrevs.go
// added/total_added are REVISION counts here, not line counts, so their
// semantic is count, not loc (D3c). main-developer uses the identical column
// names for line counts; do not copy this pair across without checking.
{Name: "added", Type: "int", Semantic: SemanticCount, Desc: "revisions by the main developer"},
{Name: "total_added", Type: "int", Semantic: SemanticCount, Desc: "total revisions of the entity across all authors"},
```

This pair is why semantics are assigned per (analysis, column) rather than per
column name globally, which is decision D3c. A consumer that built a table keyed
on `added` would silently size circles by revision count in one analysis and by
lines in the other. The `semantics` map is the only thing in the envelope that
tells them apart, and it is per-result precisely so it can.

So the split earns its place. Which makes it uncomfortable that Flint does not
have it.

## The apparent problem

Flint's registered measure types are `Amount`, `Price`, `Quantity`, `Count`,
`Number`, plus `Percentage` for proportions and a handful of signed/diverging
types. There is no line-count type, and no notion of "this is a size measure."

Mapping `count` is easy: Flint has `Count`. Mapping `loc` appeared to offer two
candidates, both of which looked wrong:

- `Count` compiles correctly. Lines are additive, summing them is meaningful, zero
  is meaningful. But then `loc` and `count` both become `Count`, and the
  distinction the codelens comment insists "is not cosmetic" is gone by the time
  the chart compiler sees the data.
- `Quantity` keeps them distinct as words. But Flint's design documentation places
  `Quantity` in the `Physical` category, whose documented behaviour is *average
  aggregation* and a unit suffix. Averaging lines added is not what anyone wants.

That framing turned out to rest on a stale document, which is worth recording
because the lesson generalizes. Flint's prose documentation disagrees with itself:
`docs/design-semantics.md` puts `Quantity` under `Physical` and glosses `Physical`
as "avg aggregation", while the agent-facing authoring skill lists `Quantity` under
"Measure (amount)" alongside `Count`. Neither page is the contract. The contract is
`packages/flint-js/src/core/type-registry.ts`, and it says:

```typescript
Quantity: { t0: 'Measure', t1: 'Physical', visEncodings: ['quantitative'],
            aggRole: 'additive', domainShape: 'open', diverging: 'none',
            formatClass: 'unit-suffix', zeroBaseline: 'meaningful', zeroPad: 0 },
Count:    { t0: 'Measure', t1: 'GenericMeasure', visEncodings: ['quantitative'],
            aggRole: 'additive', domainShape: 'open', diverging: 'none',
            formatClass: 'integer', zeroBaseline: 'meaningful', zeroPad: 0 },
```

`Quantity` is `additive` with a `meaningful` zero. The average-aggregation worry
was never real; it came from prose that describes the `Physical` category by its
other member, `Temperature`, which is the `intensive` one. The two entries differ
in exactly one field that matters, `formatClass`: `unit-suffix` versus `integer`.

So `loc` maps to `Quantity` and `count` maps to `Count`, both compile identically
where compilation matters, and the words stay distinct. There is no collapse and no
dilemma. The general lesson: when a project ships both prose and a registry, the
registry is the contract, and a design decision taken against the prose is taken
against a guess.

The fork as I originally posed it, accept the collapse or reach for a richer
annotation, was therefore a false one on both sides. It is still worth walking
through why the second side fails, because the reasoning survives the correction
and applies to every future annotation question.

## Why the richer annotation would not have helped either

Flint accepts either a bare string or an object in `semantic_types`:

```typescript
interface SemanticAnnotation {
    semanticType: string;
    intrinsicDomain?: [number, number];
    unit?: string;
    sortOrder?: string[];
}
```

Suppose the registry had said what the prose said, so that `Quantity` really was
unusable and `loc` really did have to become `Count`. The tempting rescue is
`{"semanticType": "Count", "unit": "lines"}`: keep the correct compilation
behaviour, and record the line-ness in the `unit`.

That rescue does not work, for a reason stated plainly in Flint's own
documentation. The `unit` field is annotated:

> Unit of measurement — **cosmetic when present**; omit if mixed units. Drives
> format prefix/suffix.

`unit` adds a suffix to labels. It does not influence channel selection,
aggregation, or scale. And Flint's table of "which types need metadata" lists
`Count` and `Quantity` together in the row whose justification is simply **"no
ambiguity"**. Flint's position is that these types need no annotation at all.

So the annotation buys a cosmetic label suffix, not a preserved distinction. Had
the collapse been forced, paying for a non-flat `semantics` map to escape it would
have bought nothing, and accepting the collapse would have been the right call.

## The deeper point: this was never a semantics problem

Both horns being false is not a coincidence, and the registry finding is not really
the interesting part of this episode. Even if Flint had no `Quantity`, the
distinction would have survived, because `loc`-versus-`count` is not a claim about
what the number *is*. Both are counts of things; the distinction is which *visual
channel* the number belongs on. And in Flint's model, channel assignment is not
expressed in `semantic_types` at all. It is expressed in `chart_spec.encodings`:

```jsonc
"encodings": {
  "size":  { "field": "loc_added" },
  "color": { "field": "n_revs" }
}
```

`semantic_types` answers "what kind of quantity is this?". `encodings` answers
"where does it go?". The codelens comment describes a channel concern ("the
conventional size channel of a treemap ... the colour channel"), so it belongs on
the second question, not the first.

This means the distinction survives translation in a place that does not depend on
the type registry at all. An adapter reads `semantics`, sees `loc`, and routes that
field to `size`; sees `count`, and routes it to `color`. The split is doing exactly
the job D3a claimed for it: letting the adapter "pick a size channel over a
frequency channel without domain knowledge."

The near-identity property is therefore intact, with a correction to how it should
be understood. The lookup table `semantic -> flint type` is a rename, and it is
lossy in general: it is many-to-one, and several codelens semantics do land on the
same entry. `filepath`, `person`, and `text` all become `Name`, whose registry entry
is byte-for-byte identical to `Category`'s, so even the distinction between a path
and a person is erased at the type level. But the adapter is not *only* that lookup,
and never could have been, because something has to choose the chart type and bind
the channels. The right framing is that `semantics` feeds two different consumers
inside the adapter:

- a **type map**, which is a lossy many-to-one lookup, and
- a **channel policy**, which is where the distinctions that the type map drops
  are consumed instead.

D3's claim survives, restricted to the first of those. That restriction is the
thing worth writing down, because "the adapter is near-identity" invites the
reading that the adapter is *nothing but* a rename, and that reading would have
led to throwing away `loc` as redundant with `count`.

## Where a richer annotation does earn its cost

Having concluded that the object form is not worth it for `loc`, it would be neat
to conclude it is never worth it. That is not true, and the exception is
instructive because it is a correctness issue rather than a cosmetic one. Note that
`Percentage`, unlike `Count` and `Quantity`, is registered with
`domainShape: 'bounded'`, which is the registry's own signal that its domain is
something a caller might need to state.

Flint resolves `Percentage` fields into one of two representations by inspecting
the data:

> Priority: (1) explicit `intrinsicDomain`, (2) data inspection, (3) conservative
> default. If at least 80% of absolute values are ≤1, treat the field as
> fractional.

codelens has both representations as separate semantics by design: `percentage` is
an integer 0-100 and `ratio` is a float 0-1, and the enum comment states a field
is "one or the other, never both." Both map to Flint's `Percentage`, and Flint
then re-derives from the data the very thing codelens already knew for certain.

Usually it guesses right. But consider a `percentage` column after filtering,
where every surviving row happens to have a value of 1 or 0, a plausible outcome
for an ownership table restricted to marginal contributors. Flint's heuristic sees
values ≤1, concludes fractional, and formats 1% as 100%. The data is not wrong and
the chart is not empty; the axis is simply off by two orders of magnitude, which is
the worst class of visualization defect because nothing looks broken.

The obvious rescue is to state the range: pass
`{"semanticType": "Percentage", "intrinsicDomain": [0, 100]}` and let the producer's
certainty override the consumer's guess.

**That does not work, and finding out why is the sharpest lesson in this whole
episode.** `intrinsicDomain` is a GATE, not an override. `field-semantics.ts` checks
only whether it is PRESENT before running the detection, and never consults its
value for the representation decision:

```typescript
case 'percent': {
    // Without intrinsicDomain we can't reliably distinguish 0–1
    // from 0–100, so defer to VL.
    if (!annotation.intrinsicDomain) {
        return { tooltipFormat: { pattern: precisionFormat(nums) } };
    }
    const rep = detectPercentageRepresentation(nums);
    ...
```

Compiled output confirms it. Axis format across three data shapes and three
annotations:

| data                         | bare `Percentage` | `intrinsicDomain [0,100]` | `intrinsicDomain [0,1]` |
| ---------------------------- | ----------------- | ------------------------- | ----------------------- |
| `percentage` spread 0 to 100 | `,.12~g`          | `,.12~g`                  | `,.12~g`                |
| `percentage` all values <= 1 | `,.12~g`          | **`.0~%`**                | **`.0~%`**              |
| `ratio` 0 to 1               | `,.12~g`          | `.0~%`                    | `.0~%`                  |

The two `intrinsicDomain` columns are identical, which is the proof that the value is
ignored. And `.0~%` multiplies by 100. So annotating a `percentage` column does not
prevent the 100x error; **it is the sole cause of it.** Bare, the field is never
misread.

This document originally recommended the opposite, on the strength of a documented
priority list that the code does not implement. Three sources disagreed here in
sequence: the prose said one thing, the registry implied another, and only the
compiled output settled it. The rule that survives: **for anything affecting a
rendered number, compile a spec and read the output.**

So the honest summary is a split verdict, and a narrower one than first written. Nine
of the twelve semantics translate to bare strings and should. Three want the object
form, for two different grades of reason. `ratio` wants it for CORRECTNESS: the
annotation is the only way to turn `0.34` into `34%`, and the transform is exactly
what a proportion wants. `duration_months` and `loc` want it only for RENDERING,
because `Duration` and `Quantity` both carry `formatClass: 'unit-suffix'` and a
suffix with no unit to print is a worse label than a plain integer.

`percentage` gets nothing, and pays for it with a missing `%` sign on the axis. That
suffix is simply unobtainable: whole-number percent formatting requires the gate to
be open, and opening it hands the representation decision back to the heuristic.

Written out, the whole translation is small enough to see at once, which is itself
the argument that the design worked:

| codelens semantic | Flint annotation |
| --- | --- |
| `filepath` | `Name` |
| `person` | `Name` |
| `text` | `Name`, and never bound to a channel |
| `label` | `Category` |
| `flag` | `Boolean` |
| `commit_id` | `ID` |
| `date` | `Date` |
| `count` | `Count` |
| `loc` | `{semanticType: Quantity, unit: "lines"}` |
| `duration_months` | `{semanticType: Duration, unit: "months"}` |
| `percentage` | `Percentage`, deliberately BARE |
| `ratio` | `{semanticType: Percentage, intrinsicDomain: [0, 1]}` |

Twelve semantics reach nine distinct Flint types. The flat map is the right
default and the object form is a targeted escape hatch, which is roughly what Flint
itself recommends by listing only a minority of its types as needing metadata.

The asymmetry in the last two rows is the part worth remembering. `percentage` and
`ratio` share a target type and are annotated differently, which reads as an
oversight and is the opposite: making them symmetric reintroduces a hundredfold error
in rendered ownership and coupling figures.

One thing this table quietly settles: `ratio` becomes `Percentage` rather than
`Number`, even though `Number` looks like the closer literal match for a 0-1 float.
The registry is the reason. `Number` is `additive`, and summing ownership ratios is
meaningless; `Percentage` is `intensive`, which is the correct aggregation
behaviour for a proportion. Flint's own dropped-type table recommends `Number` for a
ratio, and on this point the registry contradicts the recommendation.

## Connections

The reason this matters beyond one adapter is that it is a small instance of a
recurring problem in the codelens design: the tool knows things about its own
output that the output cannot express, and every consumer either re-derives them
or is told. The envelope's `semantics`, `transforms`, and `params` fields all exist
to move facts from documentation into data. This episode is a reminder that the
move is never complete, and that the interesting question is not "is the contract
lossless" (it is not) but "is each loss absorbed somewhere it can be absorbed
correctly."

It also connects to a rule established elsewhere in the project, that a shape is
added to the enum by the change that makes it emittable and never ahead of it. The
same instinct applies to annotation richness: `intrinsicDomain` is worth adding
when a specific misclassification has been identified, and not before, because
speculative precision in a contract is a promise that has to be kept forever.
