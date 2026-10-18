# abc

abc keeps the functions of Go code small and readable, for people and for
the models that write code with them. It measures each function by its
ABC size, its lines, its nesting and its cyclomatic complexity; fails a
check when a function is over a limit, or has grown; and, asked about one
function, tells where its size is, statement by statement.

```
$ go run github.com/shun159/abc@latest ./...
gensim/purity.go:60:1: gensim fingerprinter.value: abc 36.1 (A4.8 B33.7 C12.1) > 17, cyclo 12.5 > 10, lines 45.4 > 40, case reflect.Pointer (line 79) abc 16.1 (A3 B15 C5) > 10, case reflect.Map (line 127) abc 15.3 (A3 B15 C1) > 10
abc: 1 functions too large, by 5 sizes: split them, return early, name their conditions, or move their cases out
```

## What it measures

| metric    | limit flag     | default | what                                                                  |
|-----------|----------------|---------|-----------------------------------------------------------------------|
| `abc`     | `-max`         | 17      | sqrt(A² + B² + C²): assignments, branches (calls), conditions          |
| `lines`   | `-max-lines`   | 40      | lines of the body, but blank ones and comments                        |
| `nesting` | `-max-nesting` | 3       | the deepest if, for, switch or select; an else if as deep as its if   |
| `cyclo`   | `-max-cyclo`   | 10      | cyclomatic complexity, as gocyclo counts it                           |
| `case`    | `-max-case`    | 10      | the ABC size of each case of a switch on a value or a type            |

The ABC size is Fitzpatrick's ("Applying the ABC Metric to C, C++, and
Java", C++ Report, 1997), counted as RuboCop's `Metrics/AbcSize` counts
it, adapted to Go:

- **A**: `=`, `:=`, `op=`, `++`, `--`, and `var` names;
- **B**: calls;
- **C**: `if`, `else`, `case`, `==`, `!=`, `<`, `>`, `<=`, `>=`, `&&`, `||`, `!`.

Tests (`_test.go`) and `testdata` are left out.

### Regular code counts less

Code of statements alike reads faster than it counts: once the reader
knows the first, the next ones are read as repeats of it (Jbara and
Feitelson, "How programmers read regular code", Empirical Software
Engineering, 2017). So in every list of siblings, the statements of a block,
the cases of a switch or a select, or the elements of a literal, each counts
as much as it is new (the lines of a literal's elements are its statement's):

    counted = its size × (1 − repeat)
    repeat  = (likeness − floor) / (1 − floor), if likeness > floor

where likeness, 0 to 1, is to the most like sibling before it, of the same
kind (two ifs, two assignments, two cases, two fields of a literal), by the
shape of their code:
the kinds of their nodes in order, identifiers and literals but of a kind.
`-floor` (0.3) is the likeness of any code, which takes nothing off. It
holds for the ABC size, the paths and the lines alike.

So these count little, as they read:

```go
m.Term = d.uint()          // all of it
m.Leader = d.pid()         // a repeat of the line before
m.Index = d.uint()         // and so on
```

```go
switch e := e.(type) {     // a dispatch of cases alike
case actor.Send:
	r.send(e.To, e.Msg)
case actor.Link:
	r.env.Link(e.PID)
// ...
}
```

### A case is measured alone too

A case of a switch on a value or a type is read alone: one that does much
is a function in all but name. The `case` metric measures each as it is,
not less for its repeats, since statements alike in shape may each decide
something else. A case over `-max-case` is to be moved out.

## The baseline: a ratchet

Code written before abc may be over a limit. `-write` records the size of
each function over a limit in `.abc-baseline`, and a check fails only for a
function over a limit and not in it, or grown past its entry:

```
$ abc -write ./...          # once, and whenever a size may change
$ git add .abc-baseline
$ abc ./...                 # fails on new or grown functions
```

A function that shrinks may stay in the baseline; abc says so, and `-write`
lowers its entry. Run it on the whole module: the baseline is of all of it.

## Leaving a function out

Some code reads best whole, or is faster whole: a hot path, a critical
section under a lock, a loop over every record. A comment on the function
leaves metrics of it out, with the reason:

```go
// append appends r to the log, under mu.
//
//abc:disable abc,lines the hot path of every write, under mu
func (w *wal) append(r rec) (uint64, *batch, error) {
```

What is left out is of no limit, but still in the baseline: it may stay as
large, or shrink, but not grow. If it must grow, and the reason still
holds, `abc -write` raises its entry; abc says so instead of asking to
split it. The reason is required. A case over its limit is left out only
by naming `case`.

## Asking about one function

`-func` tells where a function's size is: each statement and case, by what
it adds, less for a repeat, and which line it repeats.

```
$ abc -func fingerprinter.value ./gensim
gensim/purity.go:60:1: gensim fingerprinter.value
  abc 36.1 (A4.8 B33.7 C12.1) > 17, lines 45.4 > 40, nesting 2, cyclo 12.5 > 10

   line   abc  A B C                repeat        code
     66  33.5  A3.8 B31.7 C10.1                   switch v.Kind() {
     67   3.2  A0 B3 C1                             case reflect.Bool:
     71   0.4  A0 B0.4 C0.1          87% of 69      case reflect.Uint, ...
     79  11.4  A2.1 B10.4 C4.1                      case reflect.Pointer:    <- case abc 16.1 > 10, as it is
     ...
```

The name is a function's (`value`, `fingerprinter.value`) or its
directory's and its (`gensim fingerprinter.value`).

## JSON, for a model

`-json` writes what `-func` tells, or the problems of a check, as JSON.
A problem left out by its directive has `"left_out": true`: it is to be
kept whole, and its baseline raised, not split.

```
$ abc -json ./...
{
  "problems": [
    { "pos": "gensim/purity.go:60:1", "func": "gensim fingerprinter.value",
      "metric": "case", "value": 16.1, "bound": 10, "past": "limit",
      "case": "reflect.Pointer", "line": 79 }
  ]
}
```

## In CI

```yaml
      - name: abc
        run: go run github.com/shun159/abc@v0.1.0 ./...
```

Pin a version: a new version may count otherwise, and move the baseline.

## Flags

```
abc [-max 17] [-max-lines 40] [-max-nesting 3] [-max-cyclo 10]
    [-max-case 10] [-floor 0.3] [-baseline .abc-baseline] [-write]
    [-v] [-func name] [-json] [patterns]
```

The patterns are directories, `./...` by default; `dir/...` takes those
under it too. `-v` lists every size over its limit, in the baseline or not.

## Limits

The limits and the floor were calibrated on a few functions of one code
base, judged readable or not by its author: at a floor of 0.3 to 0.4 the
ABC size told them apart best, and the lines and the cyclomatic complexity
did not. They are a start, not a finding.
