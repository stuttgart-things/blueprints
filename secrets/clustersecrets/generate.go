package clustersecrets

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"strings"
)

const (
	lower   = "abcdefghijklmnopqrstuvwxyz"
	upper   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digits  = "0123456789"
	symbols = "-_.!#%+="
)

// GenerateSpec describes a random value.
//
//	type: password  letters, digits and "-_.!#%+=", at least one of each class
//	type: alnum     letters and digits
//	type: hex       hex characters (length = number of characters)
//	type: base64    standard base64 of `length` random bytes
//	type: uuid      random (v4) UUID; length is ignored
//
// charset replaces the alphabet for password/alnum; the class guarantee is
// then dropped.
type GenerateSpec struct {
	Type    string `yaml:"type"`
	Length  int    `yaml:"length"`
	Charset string `yaml:"charset"`
}

const defaultLength = 32

func (g GenerateSpec) validate() error {
	switch g.Type {
	case "password", "alnum", "hex", "base64", "uuid":
	default:
		return fmt.Errorf("generate: unknown type %q (want password, alnum, hex, base64 or uuid)", g.Type)
	}
	if g.Length < 0 || g.Length > 4096 {
		return fmt.Errorf("generate: length %d out of range", g.Length)
	}
	if g.Type == "password" && g.Charset == "" && g.Length != 0 && g.Length < 4 {
		return fmt.Errorf("generate: password needs length >= 4 to contain every character class")
	}
	if g.Charset != "" && g.Type != "password" && g.Type != "alnum" {
		return fmt.Errorf("generate: charset only applies to password and alnum")
	}
	return nil
}

// Generate returns a value for g, reading randomness from r (crypto/rand in
// production; tests pass a deterministic reader).
func Generate(g GenerateSpec, r io.Reader) (string, error) {
	if r == nil {
		r = rand.Reader
	}
	n := g.Length
	if n == 0 {
		n = defaultLength
	}
	switch g.Type {
	case "alnum":
		cs := g.Charset
		if cs == "" {
			cs = lower + upper + digits
		}
		return randomString(r, cs, n)
	case "password":
		if g.Charset != "" {
			return randomString(r, g.Charset, n)
		}
		// Rejection sampling keeps the distribution uniform over all strings
		// that contain every class; with n >= 4 it terminates quickly.
		for {
			s, err := randomString(r, lower+upper+digits+symbols, n)
			if err != nil {
				return "", err
			}
			if strings.ContainsAny(s, lower) && strings.ContainsAny(s, upper) &&
				strings.ContainsAny(s, digits) && strings.ContainsAny(s, symbols) {
				return s, nil
			}
		}
	case "hex":
		b := make([]byte, (n+1)/2)
		if _, err := io.ReadFull(r, b); err != nil {
			return "", err
		}
		return hex.EncodeToString(b)[:n], nil
	case "base64":
		b := make([]byte, n)
		if _, err := io.ReadFull(r, b); err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(b), nil
	case "uuid":
		b := make([]byte, 16)
		if _, err := io.ReadFull(r, b); err != nil {
			return "", err
		}
		b[6] = (b[6] & 0x0f) | 0x40
		b[8] = (b[8] & 0x3f) | 0x80
		return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
	}
	return "", fmt.Errorf("generate: unknown type %q", g.Type)
}

func randomString(r io.Reader, charset string, n int) (string, error) {
	alphabet := []rune(charset)
	max := big.NewInt(int64(len(alphabet)))
	var sb strings.Builder
	for i := 0; i < n; i++ {
		idx, err := rand.Int(r, max)
		if err != nil {
			return "", err
		}
		sb.WriteRune(alphabet[idx.Int64()])
	}
	return sb.String(), nil
}
