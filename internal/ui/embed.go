// Package ui menyajikan frontend SvelteKit hasil build yang di-embed ke binary.
package ui

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"
)

//go:embed all:dist
var embedded embed.FS

var dist, _ = fs.Sub(embedded, "dist")

var inlineScriptRe = regexp.MustCompile(`(?s)<script(?:\s[^>]*)?>(.*?)</script>`)

func index() []byte {
	b, _ := fs.ReadFile(dist, "index.html")
	return b
}

// ScriptHashes mengembalikan hash CSP ('sha256-...') untuk setiap skrip inline di index.html.
func ScriptHashes() []string {
	var out []string
	for _, m := range inlineScriptRe.FindAllSubmatch(index(), -1) {
		if len(bytes.TrimSpace(m[1])) == 0 {
			continue // <script src=...></script>
		}
		sum := sha256.Sum256(m[1])
		out = append(out, "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")
	}
	return out
}

// Handler menyajikan file statis; rute lain jatuh ke index.html (SPA).
func Handler() http.Handler {
	files := http.FileServerFS(dist)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := index()
		if page == nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Frontend belum di-build. Jalankan: cd web && npm install && npm run build\n"))
			return
		}
		p := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if p != "" && p != "index.html" {
			if st, err := fs.Stat(dist, p); err == nil && !st.IsDir() {
				if strings.HasPrefix(p, "_app/immutable/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(page)
	})
}
