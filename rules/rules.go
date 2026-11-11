// Package rules holds the rule library that gets compiled into the binary.
package rules

import "embed"

//go:embed */*.yml
var FS embed.FS
