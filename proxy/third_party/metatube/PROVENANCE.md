# MetaTube source subset

Source: https://github.com/metatube-community/metatube-sdk-go

Commit: `fdadc9c8e321947fb6533fb0beab2a92f1c2b065`

The detector, embedded Pigo facefinder cascade, its collection helpers, and
`imageutil/crop.go` are copied unchanged. `SHA256SUMS` records their original bytes.
The upstream Apache-2.0 LICENSE is retained. The local `go.mod` includes only the
two image/detection dependencies, with their upstream pinned versions; it avoids
pulling unrelated scraping, database and translation components into the proxy.

Local additions `go.mod`, this provenance document and `SHA256SUMS` are not upstream
files. Update the commit and checksums together when refreshing the subset.

The proxy always requests advanced multi-angle face detection for marked scene
posters. On no face or processing error it preserves the input image; those policy
choices differ from MetaTube's number-dependent detection and default right fallback.
