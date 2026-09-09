// Package auth sifre hash'leme ve JWT token uretme/dogrulama islerini yapar.
package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword duz sifreyi bcrypt hash'ine cevirir.
//
// Neden bcrypt? Duz SHA-256 gibi HIZLI bir hash sifre icin KOTUDUR: saldirgan
// saniyede milyarlarca deneme yapabilir. bcrypt kasitli olarak YAVASTIR
// (cost parametresiyle ayarlanir) ve her hash'e rastgele bir "salt" gomer,
// yani ayni sifre iki farkli hash uretir -> rainbow table saldirilari ise yaramaz.
func HashPassword(plain string) (string, error) {
	// DefaultCost = 10. Daha yuksek = daha guvenli ama daha yavas login.
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword bir duz sifrenin, saklanan hash ile eslesip eslesmedigini kontrol eder.
// Sabit zamanli (constant-time) karsilastirma yapar -> timing attack'a karsi guvenli.
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
