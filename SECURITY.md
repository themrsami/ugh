# Security

ugh runs the command you give it and prints text. It never executes a
suggested fix and makes no network requests.

The things worth reporting privately:

- a way for crafted command output to make ugh do anything other than
  print (captured text is stripped of control characters before display)
- a rule in this repo whose `fix` is harmful or misleading

Use GitHub's "Report a vulnerability" button under the Security tab, or
open a normal issue if it's a bad rule: that's a bug, not a secret.
