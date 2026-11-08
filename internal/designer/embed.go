// Package designer embeds the Agui visual designer script.
package designer

import _ "embed"

// Source is the Agui script that implements the Windows designer.
//
//go:embed designer.ag
var Source string
