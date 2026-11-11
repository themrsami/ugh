# Contributing

The code is small and mostly done. The rules are where the work is, and a
rule is one YAML file, so that's what most of this page is about.

## Adding a rule

The quickest path, from a clone of this repo:

    go build -o ugh .
    ./ugh <the command that fails>      # "No rule for this one yet."
    ./ugh --new docker/some-short-name  # creates rules/docker/some-short-name.yml
    $EDITOR rules/docker/some-short-name.yml
    ./ugh --check rules

`--new` pastes the tail of the output it just saw into the file as the
first example, so you start from the real text and not from memory.

A finished rule:

```yaml
title: "Port {{port}} is already taken"
match:
  - 'EADDRINUSE.*?:(?P<port>\d+)\b'
why: |
  Something is already listening on {{port}}. Usually it's an earlier copy
  of this same server, still alive in another tab or orphaned by a crash.
fix:
  - run: lsof -i :{{port}}
    note: who has it
  - run: kill $(lsof -ti :{{port}})
    note: evict them
examples:
  - |
    Error: listen EADDRINUSE: address already in use :::3000
```

| field      |          |                                                                 |
|------------|----------|-----------------------------------------------------------------|
| `title`    | required | One line. What happened, in words the error message didn't use. |
| `match`    | required | Regexes. All of them have to match somewhere in the output.     |
| `why`      | required | The explanation. See below.                                     |
| `fix`      |          | Commands to try, in the order you'd try them. `note` is optional. |
| `cmd`      |          | Regex on the command line. Use it when the output alone is ambiguous. |
| `see`      |          | Links. Official docs only, please. Links rot.                   |
| `examples` | required | Real output that must trigger this rule. These are the tests.   |

Things that will save you a round of review:

- **Regexes are Go (RE2) syntax.** No lookahead, no backreferences. `^` and
  `$` match at line boundaries. Named groups are `(?P<name>...)` and show up
  as `{{name}}` in every text field. If two alternatives need to capture the
  same thing, name the second one `name_2`, since Go rejects duplicate names.
- **Match the part of the message that doesn't change.** Paths, versions,
  hashes and line numbers will be different on the next person's machine.
- **Examples must be real.** Copy them from a terminal. Trim them, swap out
  usernames and hostnames, but don't write them from memory: tools reword
  their errors between versions and a made-up example tests nothing. If
  the wording changed between versions, add one example per wording.
- **Your rule has to win its own examples.** `--check` fails if another rule
  scores higher on them. Usually that means your pattern is too short (make
  it match more of the line) or the two rules should be one.
- **`fix` commands get copy-pasted by people in a hurry.** Nothing
  destructive without a note saying so. No `curl | sh`. No `sudo` where it
  isn't needed.

## Writing the `why`

This is the part people actually read, so it's the part reviews focus on.

Say what is going on underneath, then why it usually happens. "Usually" is
the valuable word: the error message already says *what* failed, and what
someone staring at it wants is the two or three situations that cause it
in practice, most likely first.

Write like you'd explain it to a coworker at their desk. Short. No
"this error occurs when". If the obvious fix is a bad idea (`sudo npm`,
`verify=False`, deleting a lock file), say so, because that's what the top
search result told them to do.

Five to eight lines is plenty. If it needs more, it's probably two rules.

## What doesn't belong

- Errors from your company's internal tools. Put those in your own rules
  folder (`ugh --dir`); a team can share one by pointing `UGH_RULES` at a
  checked-out repo.
- Compiler errors about the user's own code (type mismatches, borrow
  checker, syntax errors). The compiler already explains those better than
  a regex can. Environment and tooling failures are the target.
- Rules that only restate the message. If the `why` doesn't tell me
  anything the error didn't, there's no rule there.

## Changing the code

Open an issue first for anything bigger than a bug fix. I'd like to keep
the binary boring: no network, no config file, no plugins. Run `go test
./...` before pushing; CI runs the same plus `gofmt` and `go vet`.
