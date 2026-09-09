// Package migrations SQL migration dosyalarini binary'nin icine gomer (embed).
//
// Neden gomuyoruz? Boylece "go build" ile tek bir calistirilabilir dosya cikar;
// prod'a giderken yaninda "migrations/" klasorunu tasimana gerek kalmaz.
package migrations

import "embed"

// FS bu klasordeki tum .sql dosyalarini icerir.
// //go:embed direktifi SADECE bu .go dosyasiyla ayni klasordeki (veya alt
// klasorlerdeki) dosyalari gomebilir — bu yuzden embed.go migrations/ icinde.
//
//go:embed *.sql
var FS embed.FS
