package clustersecrets

import (
	"sort"

	"gopkg.in/yaml.v3"
)

// EncryptedRecipients reads the age recipients from a sops file's metadata,
// which sops leaves in plaintext.
func EncryptedRecipients(encrypted []byte) ([]string, error) {
	var doc struct {
		Sops struct {
			Age []struct {
				Recipient string `yaml:"recipient"`
			} `yaml:"age"`
		} `yaml:"sops"`
	}
	if err := yaml.Unmarshal(encrypted, &doc); err != nil {
		return nil, err
	}
	var out []string
	for _, a := range doc.Sops.Age {
		out = append(out, a.Recipient)
	}
	return out, nil
}

// Unchanged reports whether a previously encrypted file can be kept as is:
// same content and encrypted for exactly the wanted recipients. sops output
// is not deterministic, so re-encrypting unchanged secrets would rewrite
// every file on every run and bury real changes in the diff.
func Unchanged(prev Secret, prevEncrypted []byte, next Secret, recipients []string) bool {
	if prev.Name != next.Name || prev.Namespace != next.Namespace || prev.Type != next.Type ||
		len(prev.Data) != len(next.Data) {
		return false
	}
	for k, v := range next.Data {
		if pv, ok := prev.Data[k]; !ok || pv != v {
			return false
		}
	}
	have, err := EncryptedRecipients(prevEncrypted)
	if err != nil {
		return false
	}
	return sameSet(have, recipients)
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	a, b = append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(a)
	sort.Strings(b)
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
