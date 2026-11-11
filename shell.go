package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// The hooks all do the same thing: a bare `ugh` at the prompt re-runs the
// previous command under ugh. Anything else goes straight to the binary.

// bash: fc misbehaves inside $(...), so read the history list instead. The
// last entry is the `ugh` being typed right now, hence "2 | head -1".
const bashHook = `ugh() {
  if [ $# -eq 0 ] && [ -t 0 ]; then
    local last
    last=$(HISTTIMEFORMAT= builtin history 2 | head -1 | sed 's/^ *[0-9]*\*\{0,1\} *//')
    if [ -z "$last" ] || [ "$last" = "ugh" ]; then
      command ugh
      return
    fi
    printf '\033[2m$ %s\033[0m\n' "$last" >&2
    if [ -t 2 ] && [ -n "$MSYSTEM" ]; then
      UGH_COLOR=1 command ugh -c "$last"
    else
      command ugh -c "$last"
    fi
  else
    command ugh "$@"
  fi
}
`

// zsh adds the current line to history only after it has run, so -1 is
// already the previous command.
const zshHook = `ugh() {
  if [ $# -eq 0 ] && [ -t 0 ]; then
    local last
    last=$(fc -ln -1 2>/dev/null)
    if [ -z "$last" ] || [ "$last" = "ugh" ]; then
      command ugh
      return
    fi
    printf '\033[2m$ %s\033[0m\n' "$last" >&2
    command ugh -c "$last"
  else
    command ugh "$@"
  fi
}
`

const fishHook = `function ugh
  if test (count $argv) -eq 0; and isatty stdin
    set -l last $history[1]
    if test -z "$last"; or test "$last" = ugh
      command ugh
      return
    end
    echo (set_color brblack)"\$ $last"(set_color normal) >&2
    command ugh -c "$last"
  else
    command ugh $argv
  end
end
`

// PowerShell rewrites quotes when it passes arguments to a program, so the
// command travels in an environment variable instead.
const powershellHook = `function ugh {
  if ($args.Count -eq 0 -and -not $MyInvocation.ExpectingInput) {
    $last = (Get-History -Count 1).CommandLine
    if (-not $last -or $last.Trim() -eq 'ugh') { & ugh.exe; return }
    Write-Host "> $last" -ForegroundColor DarkGray
    $env:UGH_SHELL = (Get-Process -Id $PID).Path
    $env:UGH_CMD = $last
    try { & ugh.exe -c } finally { Remove-Item Env:UGH_SHELL, Env:UGH_CMD -ErrorAction SilentlyContinue }
  } elseif ($MyInvocation.ExpectingInput) {
    $input | & ugh.exe @args
  } else {
    & ugh.exe @args
  }
}
`

func initShell(args []string) int {
	sh := ""
	if len(args) > 0 {
		sh = args[0]
	} else {
		sh = filepath.Base(os.Getenv("SHELL"))
	}
	switch sh {
	case "bash":
		os.Stdout.WriteString(bashHook)
	case "zsh":
		os.Stdout.WriteString(zshHook)
	case "fish":
		os.Stdout.WriteString(fishHook)
	case "powershell", "pwsh":
		os.Stdout.WriteString(powershellHook)
	default:
		fmt.Fprintf(os.Stderr, `ugh: don't know the shell %q. Pick one:

  bash   echo 'eval "$(ugh --init bash)"' >> ~/.bashrc
  zsh    echo 'eval "$(ugh --init zsh)"' >> ~/.zshrc
  fish   echo 'ugh --init fish | source' >> ~/.config/fish/config.fish

  powershell   Add-Content $PROFILE 'ugh.exe --init powershell | Out-String | Invoke-Expression'
  git bash     same as bash
`, sh)
		return 2
	}
	return 0
}
