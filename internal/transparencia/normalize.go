package transparencia

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// NormalizeHeader uppercases, strips accents, and collapses whitespace — the
// header-lookup key ("Localidade do gasto" ≡ "LOCALIDADE DO GASTO"). Mirrors
// sync.NormalizeName; duplicated here because the adapter must not import the
// sync package (sync imports this adapter).
func NormalizeHeader(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		out = s
	}
	return strings.Join(strings.Fields(strings.ToUpper(out)), " ")
}
