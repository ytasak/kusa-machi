// Package migrations は golang-migrate の SQL ファイルを埋め込む。
// これにより .sql を別途配布しなくてもサーバのバイナリ単体で
// マイグレーションを実行できる。
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
