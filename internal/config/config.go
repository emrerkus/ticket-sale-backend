// Package config uygulamanin tum ayarlarini tek bir yerden, ortam
// degiskenlerinden (environment variables) okur.
//
// Neden env degiskeni? (12-factor app prensibi)
//   - Kod her ortamda AYNI kalir; degisen sey sadece ayarlardir.
//   - Sirlar (DB sifresi) kodun icine yazilmaz, repoya sizmaz.
//   - Lokal'de .env dosyasindan, prod'da gercek env'den okunur; kod farketmez.
package config

import (
	"fmt"
	"os"
	"time"
)

// Config uygulamanin ihtiyac duydugu tum ayarlari tutar.
// Uygulama basladiginda BIR kez doldurulur, sonra sadece okunur (immutable gibi dusun).
type Config struct {
	HTTPPort        string        // Sunucunun dinleyecegi port, or. "8080"
	AppEnv          string        // "development" | "production" — loglama/hata detayi vs. bunu kullanir
	ShutdownTimeout time.Duration // Kapanirken acik isteklere taninan sure

	DatabaseURL string // postgres://kullanici:sifre@host:port/db?sslmode=disable
	RedisAddr   string // host:port, or. "localhost:6379"

	JWTSecret  string        // JWT imzalama anahtari (zorunlu)
	JWTTTL     time.Duration // token gecerlilik suresi
	CORSOrigin string        // frontend'in adresi (CORS icin), or. "http://localhost:5173"

	LokiURL string // Loki push API adresi, or. "http://localhost:3100". Bos ise Loki'ye log gonderilmez.
}

// Load ortam degiskenlerini okuyup bir Config uretir.
// Zorunlu bir degisken eksikse hata doner — uygulama "yarim ayarla" baslamasin.
func Load() (Config, error) {
	cfg := Config{
		HTTPPort:    getStr("HTTP_PORT", "8080"),
		AppEnv:      getStr("APP_ENV", "development"),
		DatabaseURL: getStr("DATABASE_URL", ""),
		RedisAddr:   getStr("REDIS_ADDR", "localhost:6379"),
		JWTSecret:   getStr("JWT_SECRET", ""),
		CORSOrigin:  getStr("CORS_ORIGIN", "http://localhost:5173"),
		LokiURL:     getStr("LOKI_URL", "http://localhost:3100"),
	}

	jwtTTLStr := getStr("JWT_TTL", "24h")
	jwtTTL, err := time.ParseDuration(jwtTTLStr)
	if err != nil {
		return Config{}, fmt.Errorf("JWT_TTL gecersiz (%q): %w", jwtTTLStr, err)
	}
	cfg.JWTTTL = jwtTTL

	// ShutdownTimeout'u parse et: env'de "10s", "500ms", "1m" gibi bir string var.
	// time.ParseDuration bunu time.Duration'a cevirir.
	timeoutStr := getStr("SHUTDOWN_TIMEOUT", "10s")
	d, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT gecersiz (%q): %w", timeoutStr, err)
	}
	cfg.ShutdownTimeout = d

	// --- Zorunlu alanlarin dogrulamasi ---
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL zorunlu ama bos")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET zorunlu ama bos")
	}

	return cfg, nil
}

// getStr bir env degiskenini okur; yoksa (veya bossa) verilen varsayilani doner.
func getStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
