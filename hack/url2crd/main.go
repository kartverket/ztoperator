package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	pathpkg "path"

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

	invalid = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)
)

func main() {
	flag.Var(&flagURLs, "url", "URL to a CRD YAML (repeatable)")
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

func baseFilenameFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	base := pathpkg.Base(u.Path)
	if base == "." || base == "/" {
		return ""
	}
	return base
}

func writeCRDsFromYAML(src []byte, label, outDir string) error {
	dec := yaml.NewDecoder(bytes.NewReader(src))
	written := 0

	baseFromURL := baseFilenameFromURL(label)

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

		ensureVersionedStatusSubresource(raw)

		b, err = yaml.Marshal(raw)
		if err != nil {
			return fmt.Errorf("%s: marshal doc: %w", label, err)
		}

		name := strings.TrimSpace(meta.Metadata.Name)
		if name == "" {
			// Fall back to a generic filename if name is missing.
			name = "crd"
		}

		fname := ""
		if baseFromURL != "" {
			// Use the same filename as the URL path.
			fname = baseFromURL
			// If the URL file contains multiple CRDs, disambiguate with CRD name.
			if written > 0 {
				ext := filepath.Ext(fname)
				stem := strings.TrimSuffix(fname, ext)
				if ext == "" {
					ext = ".yaml"
				}
				fname = fmt.Sprintf("%s-%s%s", stem, sanitize(name), ext)
			}
		} else {
			// Fallback if label isn't a URL.
			fname = sanitize(name) + ".yaml"
		}
		path := filepath.Join(outDir, strings.ToLower(fname))

		// Ensure each file ends with a newline.
		if len(b) == 0 || b[len(b)-1] != '\n' {
			b = append(b, '\n')
		}

		//nolint:gosec // repo-local generated files
		if werr := os.WriteFile(path, b, 0o644); werr != nil {
			return fmt.Errorf("write %s: %w", path, werr)
		}
		written++
		fmt.Printf("wrote CRD %s -> %s\n", name, path)
	}

	if written == 0 {
		return fmt.Errorf("%s: no CustomResourceDefinition documents found", label)
	}
	return nil
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
