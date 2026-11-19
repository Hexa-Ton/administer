// Package wordlists embeds the built-in wordlist so administer ships as a
// single self-contained executable that works from any directory.
package wordlists

import _ "embed"

// Builtin is the full text of administer-wordlist.txt: 1,800 unique paths,
// one per line, with the six mandatory priority paths first.
//
//go:embed administer-wordlist.txt
var Builtin string
