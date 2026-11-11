// Package examples embeds the example folders that ship as setup recipes.
package examples

import "embed"

// FS holds each example folder. Each folder with a recipe.yaml is a recipe.
//
//go:embed all:go all:nextjs all:python all:symfony
var FS embed.FS
