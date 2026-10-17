# abc: toward a metric for AI coding

Notes from a day of refactoring molecule with this tool (2026-10-17): what
worked, what was missing, and what would make abc a bridge between code an
AI writes and the people who read it.

## 1. Return how to fix, not only a number

Today the output was `abc 68.7 > 17`, and the model read the code each time
to find what to do. For a model, return the place and the move:

- "name the condition of three terms at line L"
- "this expression is repeated 8 times: make it a function" (as
  `s.alive(pid)` in gensim, `Node.missing(pid)` in proc)
- "move the body of this loop out"

As JSON, a model acts on it at once; a person reads the summary.

## 2. Measure the diff of a pull request

The before → after of each function was measured by hand and written into
every PR description. Measuring only the functions a change touches, and
printing their delta, puts it in the PR by itself: a reviewer sees what the
change made better, and by how much.

## 3. Ask for evidence when a hot path is left out

The most important moments of the day were refactors that measured slower:
`kvs wal.append` ~8% on parallel puts, `wal.syncNow` ~25% for one durable
writer, `readRecords` 3-6% on opening. A model lowers numbers by splitting,
and splits too far. A directive could name its benchmark:

    //abc:disable abc bench=BenchmarkPutDurable the hot path of every durable write

and CI could check the benchmark exists, so that an exception is never a
guess.

## 4. Count what stops a human reader, beside ABC

Where the reader stopped did not always match the ABC size:

- a name and its effects at odds: `translate`, named to convert, changed
  state (gen runner)
- unusual idioms: three nested func types (`eachShard`), a variadic used as
  an optional out parameter (`last ...*uint64`)
- conditions leaning on precedence: `&&` and `||` mixed, three terms or more
  unnamed (now a house rule)
- the state to follow: the fields of a struct one function reads and writes

## 5. Warn of splitting too far

Lowering numbers alone leaves many one-line functions, harder to follow.
The depth of calls, and small functions used once, could flag a split gone
too far, so that a model does not overdo it.

## 6. Calibrate against human judgment

Keep the before and after of the code a person found hard and then easy
(`translate`, `eachShard`, kvs `open.go`, `recover`) as labelled examples,
and check the metrics rank them as the person did. With a YAML config
(limits, rules on or off, exceptions by directory), each repository tunes
its own.

## 7. Keep the baseline across moves

A refactor by a model often moves a function to another file, or renames
it. The baseline keys by directory and name, so a move looks like a new
function. Matching functions by the likeness of their bodies would keep
their entries.

## Since

Done in v0.1.0: 1 in part (`-func` tells where a function's size is,
`-json` writes it and the problems for a model), the
`//abc:disable <metrics> <reason>` directive, with what it leaves out kept
in the baseline, and 6 in a first form (regular code counts less, by the
likeness of siblings; calibrated on ten functions). Next: 2, 3, 5, 7, and
the YAML config.

In short: a tool that gives a model instructions it can act on, and a
person evidence they can check.
