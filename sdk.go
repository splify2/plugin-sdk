// Package sdk несёт файлы SDK, которые инструмент s2p вшивает в себя: схемы, заголовки, исходники
// библиотек для программ и шаблоны. Так s2p работает одним файлом, без дерева SDK рядом.
package sdk

import "embed"

//go:embed schema/*.json include lib templates
var FS embed.FS
