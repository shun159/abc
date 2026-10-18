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

## Hot paths

Code every operation runs is marked hot, with the benchmarks that measure
it, those of its package or of another, after its directory:

```go
// put writes k, under the lock of its shard.
//
//abc:hot BenchmarkPut,BenchmarkMixed the path of every write
func (x *hamtIndex[K, V]) put(...) {
```

It leaves nothing out, as `//abc:disable` does: it tells that a change to
the function is to be measured, and by what. `-diff` says so of a hot
function the change touched, and `-hot` checks that its benchmarks are
there, failing if not, and lists the calls in it the compiler does not
inline, with the compiler's reason:

```
$ abc -hot ./kvs
kvs/index_tree.go:181:1: kvs treeIndex.ascend: hot, measured by BenchmarkOrdered
  line 182: cursors not inlined: function too complex: cost 213 exceeds budget 80
  line 200: down not inlined: function too complex: cost 235 exceeds budget 80
```

A split of a hot path most often costs such a call, a function over the
inliner's budget, or one it could inline and did not there: a small
function of another package, in a generic body. `-hot` compiles each
package with its tests (`go test -c -gcflags=-m=2`), which instantiate
its generic code; a generic function no test instantiates is told so. The
calls are matched to the compiler's by name, so of two functions of one
name, it gives both reasons.

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

## What a change did

`-diff rev` measures the functions a change touched, since the git
revision rev, before and after, instead of checking: a table for a pull
request, and the signs of a split that only spreads the code. A function
the change added is told when it is called at one place only, is of one
statement, or passes its arguments on to another (Ousterhout's
pass-through method): alone, each is easy to read; together, the reader
jumps more. The sum tells the largest ABC size, all of them, and how many
functions, before and after: a split that simplifies lowers the largest,
not the sum alone.

```
$ abc -diff origin/master ./...
against origin/master: 13 functions touched

  abc             lines           cyclo         function
  20.8 → 14.2     28.6 → 17       7.7 → 5.5     internal/champ Node.set
  - → 4.6         - → 3.9         - → 1         internal/champ Node.pushedDown  (new, called at one place)
  ...

  largest abc 20.8 → 16.8, total abc 60.6 → 68.9, functions 3 → 13
  new functions: 9 called at one place, 3 of one statement, 0 passing through
```

`-md` writes the same as a Markdown table, to paste in a pull request.

It never fails: the signs are to weigh, as a function of one place with a
name of its own may well be worth it. With `-json`, it writes the same for
a model.

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
    [-v] [-func name] [-diff rev [-md]] [-hot] [-json] [patterns]
```

The patterns are directories, `./...` by default; `dir/...` takes those
under it too. `-v` lists every size over its limit, in the baseline or not.

## Limits

The limits and the floor were calibrated on a few functions of one code
base, judged readable or not by its author: at a floor of 0.3 to 0.4 the
ABC size told them apart best, and the lines and the cyclomatic complexity
did not. They are a start, not a finding.
