package importer

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path"
	"strings"
)

const MaxUpload = 8 << 20

func safePath(name string) bool {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimSuffix(name, "/")
	return name != "" && !strings.HasPrefix(name, "/") && !strings.Contains(name, ":") && path.Clean(name) == name && !strings.HasPrefix(name, "../") && name != ".."
}

func Extract(name string, data []byte) (map[string][]byte, error) {
	out := map[string][]byte{}
	total := 0
	add := func(n string, r io.Reader) error {
		if !safePath(n) {
			return fmt.Errorf("path arsip tidak aman: %s", n)
		}
		if strings.HasSuffix(n, "/") {
			return nil
		}
		if len(out) >= 512 {
			return fmt.Errorf("terlalu banyak berkas")
		}
		b, e := io.ReadAll(io.LimitReader(r, (1<<20)+1))
		if e != nil {
			return e
		}
		if len(b) > 1<<20 {
			return fmt.Errorf("berkas terlalu besar: %s", n)
		}
		total += len(b)
		if total > MaxUpload {
			return fmt.Errorf("isi arsip terlalu besar")
		}
		out[n] = b
		return nil
	}
	switch {
	case strings.HasSuffix(strings.ToLower(name), ".zip"):
		z, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if e != nil {
			return nil, e
		}
		for _, f := range z.File {
			if !safePath(f.Name) {
				return nil, fmt.Errorf("path arsip tidak aman: %s", f.Name)
			}
			if f.FileInfo().IsDir() {
				continue
			}
			if !f.Mode().IsRegular() {
				return nil, fmt.Errorf("jenis berkas tidak didukung")
			}
			r, e := f.Open()
			if e != nil {
				return nil, e
			}
			e = add(f.Name, r)
			r.Close()
			if e != nil {
				return nil, e
			}
		}
	case strings.HasSuffix(strings.ToLower(name), ".tar.gz") || strings.HasSuffix(strings.ToLower(name), ".tgz"):
		g, e := gzip.NewReader(bytes.NewReader(data))
		if e != nil {
			return nil, e
		}
		defer g.Close()
		tr := tar.NewReader(g)
		for {
			h, e := tr.Next()
			if e == io.EOF {
				break
			}
			if e != nil {
				return nil, e
			}
			if !safePath(h.Name) {
				return nil, fmt.Errorf("path arsip tidak aman: %s", h.Name)
			}
			if h.Typeflag == tar.TypeDir {
				continue
			}
			if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
				return nil, fmt.Errorf("jenis berkas tidak didukung")
			}
			if e = add(h.Name, tr); e != nil {
				return nil, e
			}
		}
	default:
		return nil, fmt.Errorf("format arsip tidak didukung")
	}
	return out, nil
}
