package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v4"
)

// Simple fetch-and-split tool for CRD manifests.
//
// Intended usage from hack/crd (via go:generate):
//   go run ../url2crd -url=<crd-yaml-url> -outdir=<output directory>
//
// By default it writes CRD YAML files into ./bases (i.e. hack/crd/bases).

type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

type objectMeta struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
}

var (
	flagOutDir = flag.String("outdir", "./bases", "directory where downloaded CRD manifests will be written")
	flagURLs   multiFlag
	flagKinds  multiFlag

	invalid = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)
)

func main() {
	flag.Var(&flagURLs, "url", "URL to a CRD YAML (repeatable)")
	flag.Var(
		&flagKinds,
		"kind",
		"only write the CRD for this kind name (repeatable); required when a URL contains multiple CRDs",
	)
	flag.Parse()

	if len(flagURLs) == 0 {
		fatalf("no inputs provided; use -url (repeatable)")
	}

	if err := os.MkdirAll(*flagOutDir, 0o755); err != nil {
		fatalf("creating outdir %s: %v", *flagOutDir, err)
	}

	for _, u := range flagURLs {
		b := fetchURLBytes(u)
		if err := writeCRDsFromYAML(b, u, *flagOutDir); err != nil {
			fatalf("%v", err)
		}
	}
}

func fetchURLBytes(u string) []byte {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil)
	if err != nil {
		fatalf("build request: %v", err)
	}
	req.Header.Set("Accept", "application/yaml, text/yaml, text/plain; q=0.9, */*; q=0.1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fatalf("GET %s: %v", u, err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "close response body for %s: %v\n", u, cerr)
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fatalf("GET %s: unexpected status %s", u, resp.Status)
	}
	if ct := resp.Header.Get("Content-Type"); strings.Contains(ct, "text/html") {
		fatalf("URL %s returned HTML, not YAML", u)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fatalf("read response body from %s: %v", u, err)
	}
	return body
}

// crdFileName returns the canonical filename for a CRD document:
// <spec.group>_<spec.names.plural>.yaml  (e.g. skiperator.kartverket.no_applications.yaml)
func crdFileName(raw map[string]any) string {
	spec, _ := raw["spec"].(map[string]any)
	group, _ := spec["group"].(string)
	names, _ := spec["names"].(map[string]any)
	plural, _ := names["plural"].(string)

	if group == "" || plural == "" {
		// Fallback: use metadata.name which is already "<plural>.<group>"
		meta, _ := raw["metadata"].(map[string]any)
		name, _ := meta["name"].(string)
		if name == "" {
			name = "crd"
		}
		return strings.ToLower(sanitize(name)) + ".yaml"
	}
	return strings.ToLower(group) + "_" + strings.ToLower(plural) + ".yaml"
}

func writeCRDsFromYAML(src []byte, label, outDir string) error {
	dec := yaml.NewDecoder(bytes.NewReader(src))

	// Collect all CRD documents first so we can check cardinality before writing.
	type crdEntry struct {
		raw  map[string]any
		meta objectMeta
	}
	var crds []crdEntry

	for {
		var raw map[string]any
		if err := dec.Decode(&raw); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("%s: decode yaml: %w", label, err)
		}
		if len(raw) == 0 {
			continue
		}
		b, err := yaml.Marshal(raw)
		if err != nil {
			return fmt.Errorf("%s: marshal doc: %w", label, err)
		}
		var meta objectMeta
		_ = yaml.Unmarshal(b, &meta)
		if meta.Kind != "CustomResourceDefinition" {
			continue
		}
		crds = append(crds, crdEntry{raw: raw, meta: meta})
	}

	if len(crds) == 0 {
		return fmt.Errorf("%s: no CustomResourceDefinition documents found", label)
	}

	// If the URL has multiple CRDs, -kind is required so the caller is explicit
	// about which one(s) should be written.
	if len(crds) > 1 && len(flagKinds) == 0 {
		kinds := make([]string, 0, len(crds))
		for _, c := range crds {
			kinds = append(kinds, crdDefinedKind(c.raw))
		}
		return fmt.Errorf(
			"%s: contains %d CRDs (%s); use -kind to specify which one(s) to write",
			label, len(crds), strings.Join(kinds, ", "),
		)
	}

	// Build a lookup set for the requested kinds (case-insensitive).
	kindFilter := make(map[string]bool, len(flagKinds))
	for _, k := range flagKinds {
		kindFilter[strings.ToLower(k)] = true
	}

	written := 0
	for _, c := range crds {
		definedKind := crdDefinedKind(c.raw)
		if len(kindFilter) > 0 && !kindFilter[strings.ToLower(definedKind)] {
			continue
		}

		ensureVersionedStatusSubresource(c.raw)

		b, err := yaml.Marshal(c.raw)
		if err != nil {
			return fmt.Errorf("%s: marshal doc: %w", label, err)
		}

		fname := crdFileName(c.raw)
		path := filepath.Join(outDir, fname)

		if len(b) == 0 || b[len(b)-1] != '\n' {
			b = append(b, '\n')
		}

		//nolint:gosec // repo-local generated files
		if werr := os.WriteFile(path, b, 0o644); werr != nil {
			return fmt.Errorf("write %s: %w", path, werr)
		}
		written++
		fmt.Printf("wrote CRD %s -> %s\n", fname, path)
	}

	if written == 0 {
		requested := strings.Join(flagKinds, ", ")
		return fmt.Errorf("%s: no CRDs matched -kind=%s", label, requested)
	}
	return nil
}

// crdDefinedKind returns the kind name defined by a CRD document (spec.names.kind).
func crdDefinedKind(raw map[string]any) string {
	spec, ok := raw["spec"].(map[string]any)
	if !ok {
		return ""
	}
	names, ok := spec["names"].(map[string]any)
	if !ok {
		return ""
	}
	kind, _ := names["kind"].(string)
	return kind
}

func ensureVersionedStatusSubresource(raw map[string]any) {
	spec, ok := raw["spec"].(map[string]any)
	if !ok {
		return
	}

	versions, ok := spec["versions"].([]any)
	if !ok || len(versions) == 0 {
		return
	}

	for _, v := range versions {
		ver, ok := v.(map[string]any)
		if !ok {
			continue
		}

		subresources, ok := ver["subresources"].(map[string]any)
		if !ok || subresources == nil {
			subresources = map[string]any{}
			ver["subresources"] = subresources
		}

		if _, ok := subresources["status"]; !ok {
			subresources["status"] = map[string]any{}
		}
	}
}

func sanitize(s string) string {
	return invalid.ReplaceAllString(s, "-")
}

func fatalf(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
