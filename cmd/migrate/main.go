// cmd/migrate — veritabani semasini yoneten kucuk arac.
//
// Kullanim:
//
//	go run ./cmd/migrate up         -> bekleyen tum migration'lari uygular
//	go run ./cmd/migrate down       -> son migration'i geri alir (1 adim)
//	go run ./cmd/migrate version    -> su anki sema surumunu gosterir
//	go run ./cmd/migrate force <N>  -> "dirty" bayragini temizler, versiyonu N'e sabitler
//	                                   (SQL'i elle duzelttikten sonra kullanilir)
//
// golang-migrate kutuphanesini "kod icinden" kullaniyoruz; ayri bir CLI binary
// kurmana gerek yok. Migration dosyalari migrations/ klasorunde ve binary'ye gomulu.
package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	// Bu blank import golang-migrate'in "pgx5://" veritabani surucusunu kaydeder.
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/joho/godotenv"

	"github.com/emrerkus/ticket-sale-backend/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migrate hatasi:", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()

	if len(os.Args) < 2 {
		return errors.New("komut gerekli: up | down | version")
	}
	cmd := os.Args[1]

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errors.New("DATABASE_URL bos")
	}
	// golang-migrate'in pgx/v5 surucusu "pgx5://" semasi bekliyor; bizim
	// DATABASE_URL "postgres://" ile basliyor. Basi degistiriyoruz.
	dbURL = strings.Replace(dbURL, "postgres://", "pgx5://", 1)

	// iofs: gomulu dosya sistemini golang-migrate'in "kaynak" arayuzune baglar.
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("migration kaynagi acilamadi: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, dbURL)
	if err != nil {
		return fmt.Errorf("migrate baslatilamadi: %w", err)
	}
	defer m.Close()

	switch cmd {
	case "up":
		err = m.Up()
	case "down":
		err = m.Steps(-1) // sadece 1 adim geri
	case "force":
		if len(os.Args) < 3 {
			return errors.New("kullanim: force <versiyon>, or. 'force 1'")
		}
		v, perr := strconv.Atoi(os.Args[2])
		if perr != nil {
			return fmt.Errorf("gecersiz versiyon %q: %w", os.Args[2], perr)
		}
		// Force: schema_migrations'i (versiyon=v, dirty=false) yapar. SQL'i
		// CALISTIRMAZ — sadece "ben bu versiyondayim, temizim" der. Once semayi
		// gercekten o duruma elle getirmis olman gerekir.
		if err = m.Force(v); err == nil {
			fmt.Printf("versiyon %d'e sabitlendi (dirty temizlendi).\n", v)
		}
		return err
	case "version":
		v, dirty, verr := m.Version()
		if errors.Is(verr, migrate.ErrNilVersion) {
			fmt.Println("sema surumu: (hic migration uygulanmamis)")
			return nil
		}
		if verr != nil {
			return verr
		}
		fmt.Printf("sema surumu: %d (dirty=%v)\n", v, dirty)
		return nil
	default:
		return fmt.Errorf("bilinmeyen komut: %q", cmd)
	}

	// "Degisiklik yok" bir hata degil — sessizce basarili say.
	if errors.Is(err, migrate.ErrNoChange) {
		fmt.Println("degisiklik yok, sema guncel.")
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Printf("%q tamamlandi.\n", cmd)
	return nil
}
