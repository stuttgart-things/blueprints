package clusterapps

import (
	"regexp"
	"strings"
)

// PragmaAllowlist is what detect-secrets accepts as an inline false-positive
// marker.
const PragmaAllowlist = "# pragma: allowlist secret" // pragma: allowlist secret

// secretKeyword are the key words detect-secrets' KeywordDetector looks for
// (lower-cased; `_` optional). A generated file only ever holds variable and
// object NAMES under such keys (INFRA_TLS_SECRET: wildcard-tls), never values:
// those are SOPS in cluster-secrets/.
var secretKeyword = regexp.MustCompile(`(?i)(api_?key|auth_?key|service_?key|account_?key|db_?key|database_?key|priv_?key|private_?key|client_?key|db_?pass|database_?pass|key_?pass|password|passwd|pwd|secret)`)

// mapEntry: indentation, optional "- ", a key, ":" and a non-empty value.
var mapEntry = regexp.MustCompile(`^(\s*)(- )?([^\s#:'"][^:#]*|'[^']*'|"[^"]*"):\s+(\S.*)$`)

// AllowlistSecretKeywords marks every `key: value` line whose key contains a
// detect-secrets keyword with PragmaAllowlist, so a repo's pre-commit hook
// passes on generated files without excluding them (a hand-added pragma would
// be lost on the next render). Block scalars (`|`, `>`) are left alone: a
// comment there would become part of the string. Lines that already carry a
// comment are left alone too.
func AllowlistSecretKeywords(doc string) string {
	lines := strings.Split(doc, "\n")
	blockIndent := -1 // >= 0: inside a block scalar opened at that indent
	for i, l := range lines {
		indent := len(l) - len(strings.TrimLeft(l, " "))
		if blockIndent >= 0 {
			if strings.TrimSpace(l) == "" || indent > blockIndent {
				continue
			}
			blockIndent = -1
		}
		m := mapEntry.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		value := m[4]
		if strings.HasPrefix(value, "|") || strings.HasPrefix(value, ">") {
			blockIndent = len(m[1])
			if m[2] != "" {
				blockIndent += len(m[2])
			}
			continue
		}
		if !secretKeyword.MatchString(m[3]) || strings.Contains(value, " #") {
			continue
		}
		lines[i] = l + " " + PragmaAllowlist
	}
	return strings.Join(lines, "\n")
}
