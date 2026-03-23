//go:build generate

package crd

// Create an alias for the tool command to download CRD manifests.
// This runs the downloader program located at hack/url2crd.
//go:generate -command urlcrd go run ../../hack/url2crd

// Skiperator Application CRD
//go:generate urlcrd -outdir=./bases -url=https://raw.githubusercontent.com/kartverket/skiperator/refs/heads/main/config/crd/skiperator.kartverket.no_applications.yaml
