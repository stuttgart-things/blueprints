package clustersecrets

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	SchemeSops  = "sops"
	SchemeVault = "vault"
)

// Ref is a parsed reference to an existing value, in the style of vals:
//
//	ref+sops://secrets/minio.enc.yaml#/accessKey
//	ref+vault://secret/edge/teams#/webhook        (KV v2, mount "secret")
//	ref+vault://kv1/legacy?kv=1#/token            (KV v1)
//
// The fragment is a JSON pointer into the decrypted file or the Vault
// secret's data.
type Ref struct {
	Raw     string
	Scheme  string
	Path    string
	Pointer string
	// KVVersion is 1 or 2; Vault refs only.
	KVVersion int
}

func ParseRef(raw string) (Ref, error) {
	rest, ok := strings.CutPrefix(raw, "ref+")
	if !ok {
		return Ref{}, fmt.Errorf("ref %q: must start with ref+", raw)
	}
	u, err := url.Parse(rest)
	if err != nil {
		return Ref{}, fmt.Errorf("ref %q: %w", raw, err)
	}
	r := Ref{Raw: raw, Scheme: u.Scheme, Path: strings.Trim(u.Host+u.Path, "/"), Pointer: u.Fragment}
	if r.Path == "" {
		return Ref{}, fmt.Errorf("ref %q: path is empty", raw)
	}
	if !strings.HasPrefix(r.Pointer, "/") {
		return Ref{}, fmt.Errorf("ref %q: fragment must be a JSON pointer like #/key", raw)
	}
	switch r.Scheme {
	case SchemeSops:
		if len(u.Query()) > 0 {
			return Ref{}, fmt.Errorf("ref %q: sops refs take no query parameters", raw)
		}
		if strings.Contains(r.Path, "..") {
			return Ref{}, fmt.Errorf("ref %q: path must not contain '..'", raw)
		}
	case SchemeVault:
		r.KVVersion = 2
		for k, v := range u.Query() {
			if k != "kv" {
				return Ref{}, fmt.Errorf("ref %q: unknown query parameter %q", raw, k)
			}
			n, err := strconv.Atoi(v[0])
			if err != nil || (n != 1 && n != 2) {
				return Ref{}, fmt.Errorf("ref %q: kv must be 1 or 2", raw)
			}
			r.KVVersion = n
		}
		if r.KVVersion == 2 && !strings.Contains(r.Path, "/") {
			return Ref{}, fmt.Errorf("ref %q: KV v2 path needs <mount>/<secret>", raw)
		}
	default:
		return Ref{}, fmt.Errorf("ref %q: unsupported scheme %q (want sops or vault)", raw, r.Scheme)
	}
	return r, nil
}

// VaultAPIPath is the path below /v1/ to GET for this ref.
func (r Ref) VaultAPIPath() string {
	if r.KVVersion == 1 {
		return r.Path
	}
	mount, secret, _ := strings.Cut(r.Path, "/")
	return mount + "/data/" + secret
}

// CollectRefs returns the distinct refs used by specs, sorted by Raw.
func CollectRefs(specs []SecretSpec) []Ref {
	seen := map[string]Ref{}
	for _, s := range specs {
		for _, v := range s.Data {
			if v.Ref != "" {
				r, _ := ParseRef(v.Ref) // validated in Merge
				seen[r.Raw] = r
			}
		}
	}
	out := make([]Ref, 0, len(seen))
	for _, r := range seen {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Raw < out[j].Raw })
	return out
}

// SopsFiles and VaultPaths list what has to be fetched to resolve refs.
func SopsFiles(refs []Ref) []string {
	return distinct(refs, SchemeSops, func(r Ref) string { return r.Path })
}
func VaultPaths(refs []Ref) []string { return distinct(refs, SchemeVault, Ref.VaultAPIPath) }

func distinct(refs []Ref, scheme string, key func(Ref) string) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range refs {
		if r.Scheme == scheme && !seen[key(r)] {
			seen[key(r)] = true
			out = append(out, key(r))
		}
	}
	sort.Strings(out)
	return out
}

// ResolveRefs looks every ref up in the fetched documents: sopsDocs maps a
// sops file path to its decrypted YAML/JSON, vaultDocs a Vault API path to
// the raw JSON response body. Returns ref.Raw -> value.
func ResolveRefs(refs []Ref, sopsDocs map[string][]byte, vaultDocs map[string][]byte) (map[string]string, error) {
	parsedSops := map[string]any{}
	parsedVault := map[string]any{}
	out := map[string]string{}

	for _, r := range refs {
		var root any
		switch r.Scheme {
		case SchemeSops:
			doc, ok := parsedSops[r.Path]
			if !ok {
				raw, found := sopsDocs[r.Path]
				if !found {
					return nil, fmt.Errorf("%s: file %s not fetched", r.Raw, r.Path)
				}
				if err := yaml.Unmarshal(raw, &doc); err != nil {
					return nil, fmt.Errorf("%s: parse %s: %w", r.Raw, r.Path, err)
				}
				parsedSops[r.Path] = doc
			}
			root = doc
		case SchemeVault:
			p := r.VaultAPIPath()
			if _, ok := parsedVault[p]; !ok {
				raw, found := vaultDocs[p]
				if !found {
					return nil, fmt.Errorf("%s: vault path %s not fetched", r.Raw, p)
				}
				var body any
				if err := yaml.Unmarshal(raw, &body); err != nil {
					return nil, fmt.Errorf("%s: parse vault response: %w", r.Raw, err)
				}
				// KV v1: {"data": {...}}, KV v2: {"data": {"data": {...}}}
				ptr := "/data"
				if r.KVVersion == 2 {
					ptr = "/data/data"
				}
				doc, err := lookup(body, ptr)
				if err != nil {
					return nil, fmt.Errorf("%s: unexpected vault response: %w", r.Raw, err)
				}
				parsedVault[p] = doc
			}
			root = parsedVault[p]
		}

		v, err := lookup(root, r.Pointer)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", r.Raw, err)
		}
		s, err := scalarString(v)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", r.Raw, err)
		}
		out[r.Raw] = s
	}
	return out, nil
}

// lookup walks a JSON pointer (RFC 6901) through decoded YAML/JSON.
func lookup(doc any, pointer string) (any, error) {
	if pointer == "" || pointer == "/" {
		return doc, nil
	}
	cur := doc
	for _, tok := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		tok = strings.ReplaceAll(strings.ReplaceAll(tok, "~1", "/"), "~0", "~")
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[tok]
			if !ok {
				return nil, fmt.Errorf("key %q not found (pointer %s)", tok, pointer)
			}
			cur = v
		case []any:
			i, err := strconv.Atoi(tok)
			if err != nil || i < 0 || i >= len(node) {
				return nil, fmt.Errorf("index %q out of range (pointer %s)", tok, pointer)
			}
			cur = node[i]
		default:
			return nil, fmt.Errorf("cannot descend into %T at %q (pointer %s)", cur, tok, pointer)
		}
	}
	return cur, nil
}

func scalarString(v any) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case nil:
		return "", fmt.Errorf("value is null")
	case map[string]any, []any:
		return "", fmt.Errorf("value is a %T, not a scalar", v)
	default:
		return fmt.Sprint(t), nil
	}
}
