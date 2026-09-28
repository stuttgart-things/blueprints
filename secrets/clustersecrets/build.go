package clustersecrets

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Secret is a fully resolved Kubernetes Secret.
type Secret struct {
	Name      string
	Namespace string
	Type      string
	Data      map[string]string
}

func (s Secret) ID() string { return s.Namespace + "/" + s.Name }

// Existing holds the values of a previous run: secret ID -> key -> value.
type Existing map[string]map[string]string

// Rotation selects generated values to regenerate even though a previous
// value exists. Entries are "<secret>" (every generated key of that secret)
// or "<secret>:<key>"; <secret> is a name or namespace/name.
type Rotation []string

// ParseRotation splits a comma-separated rotate list.
func ParseRotation(s string) Rotation {
	var out Rotation
	for _, e := range strings.Split(s, ",") {
		if e = strings.TrimSpace(e); e != "" {
			out = append(out, e)
		}
	}
	return out
}

func (r Rotation) matches(entry string, s SecretSpec, key string) bool {
	sel, k, hasKey := strings.Cut(entry, ":")
	if sel != s.Name && sel != s.ID() {
		return false
	}
	return !hasKey || k == key
}

// Build turns merged specs into secrets:
//
//   - value:    the literal
//   - ref:      the resolved value (always current; refs are never cached)
//   - generate: the previous value if one exists and is not rotated,
//     otherwise a fresh one
//
// Keeping previous generated values is what makes re-running safe: without
// it every run would replace every password on the cluster.
func Build(specs []SecretSpec, existing Existing, resolved map[string]string, rotate Rotation, rnd io.Reader) ([]Secret, error) {
	used := make([]bool, len(rotate))
	var out []Secret
	for _, s := range specs {
		sec := Secret{Name: s.Name, Namespace: s.Namespace, Type: s.Type, Data: map[string]string{}}
		keys := make([]string, 0, len(s.Data))
		for k := range s.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			v := s.Data[k]
			switch {
			case v.Value != nil:
				sec.Data[k] = *v.Value
			case v.Ref != "":
				val, ok := resolved[v.Ref]
				if !ok {
					return nil, fmt.Errorf("secret %s, key %s: %s was not resolved", s.ID(), k, v.Ref)
				}
				sec.Data[k] = val
			case v.Generate != nil:
				rotateThis := false
				for i, e := range rotate {
					if rotate.matches(e, s, k) {
						rotateThis, used[i] = true, true
					}
				}
				if prev, ok := existing[s.ID()][k]; ok && !rotateThis {
					sec.Data[k] = prev
					continue
				}
				val, err := Generate(*v.Generate, rnd)
				if err != nil {
					return nil, fmt.Errorf("secret %s, key %s: %w", s.ID(), k, err)
				}
				sec.Data[k] = val
			}
		}
		out = append(out, sec)
	}

	// A rotate entry that matched nothing is almost certainly a typo; failing
	// beats reporting success without rotating anything.
	for i, e := range rotate {
		if !used[i] {
			return nil, fmt.Errorf("rotate %q matches no generated value", e)
		}
	}
	return out, nil
}
