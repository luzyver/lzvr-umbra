package importer

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestArchives(t *testing.T) {
	for _, kind := range []string{"zip", "tar.gz"} {
		var b bytes.Buffer
		if kind == "zip" {
			w := zip.NewWriter(&b)
			f, _ := w.Create("db/clients/one.json")
			f.Write([]byte(`{"name":"a"}`))
			w.Close()
		} else {
			g := gzip.NewWriter(&b)
			w := tar.NewWriter(g)
			data := []byte(`{"name":"a"}`)
			w.WriteHeader(&tar.Header{Name: "db/clients/one.json", Mode: 0600, Size: int64(len(data))})
			w.Write(data)
			w.Close()
			g.Close()
		}
		files, err := Extract("db."+kind, b.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		if string(files["db/clients/one.json"]) != `{"name":"a"}` {
			t.Fatalf("%s: %v", kind, files)
		}
	}
}

func TestZipSlip(t *testing.T) {
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	f, _ := w.Create("../evil.json")
	f.Write([]byte(`{}`))
	w.Close()
	if _, err := Extract("db.zip", b.Bytes()); err == nil {
		t.Fatal("path keluar arsip diterima")
	}
}

func TestWGConf(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "wg0.conf"))
	if err != nil {
		t.Fatal(err)
	}
	x, err := ParseWGConf(b)
	if err != nil {
		t.Fatal(err)
	}
	if x.Server.Address != "10.8.0.1/24" || x.Server.ListenPort != 51820 || len(x.Clients) != 3 {
		t.Fatalf("hasil: %+v", x)
	}
	for i, name := range []string{"Laptop", "Ponsel", "Rumah"} {
		if x.Clients[i].Name != name {
			t.Errorf("nama %d: %q", i, x.Clients[i].Name)
		}
	}
	if x.Clients[0].PrivateKey != "" || x.Clients[0].Address != "10.8.0.2" || x.Clients[0].Keepalive != 25 {
		t.Fatalf("perangkat: %+v", x.Clients[0])
	}
}

func TestWGConfNameInsideAfterKey(t *testing.T) {
	b := []byte("[Interface]\nPrivateKey = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\nAddress = 10.8.0.1/24\n[Peer]\nPublicKey = BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=\n# Setelah kunci\nAllowedIPs = 10.8.0.2/32\n")
	x, err := ParseWGConf(b)
	if err != nil {
		t.Fatal(err)
	}
	if x.Clients[0].Name != "Setelah kunci" {
		t.Fatalf("nama: %q", x.Clients[0].Name)
	}
}

func TestWGUI(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "client.json"))
	if err != nil {
		t.Fatal(err)
	}
	iface, _ := os.ReadFile(filepath.Join("testdata", "interfaces.json"))
	key, _ := os.ReadFile(filepath.Join("testdata", "keypair.json"))
	global, _ := os.ReadFile(filepath.Join("testdata", "global_settings.json"))
	x, err := ParseWGUI(map[string][]byte{"db/clients/abc.json": b, "db/server/interfaces.json": iface, "db/server/keypair.json": key, "db/server/global_settings.json": global})
	if err != nil {
		t.Fatal(err)
	}
	if len(x.Clients) != 1 || x.Clients[0].Name != "Laptop" || x.Clients[0].Address != "10.8.0.2" || x.Clients[0].PrivateKey == "" || !x.Clients[0].Enabled {
		t.Fatalf("hasil: %+v", x)
	}
	if x.Clients[0].CreatedAt.Year() != 2025 || x.Clients[0].UpdatedAt.Day() != 2 {
		t.Fatalf("tanggal: %+v", x.Clients[0])
	}
	if x.Server.Address != "10.8.0.1/24" || x.Server.ListenPort != 51820 || x.Server.PrivateKey == "" {
		t.Fatalf("server: %+v", x.Server)
	}
	if x.DefaultMTU != 1420 || x.DefaultKeepalive != 25 || len(x.DefaultDNS) != 1 || x.DefaultDNS[0] != "10.8.0.1" {
		t.Fatalf("bawaan: %+v", x)
	}
}

// Arsip db wireguard-ui lengkap: server/hashes.json dan db/users dibuat upstream dan bukan perangkat.
func TestWGUIFullArchive(t *testing.T) {
	read := func(n string) []byte {
		b, err := os.ReadFile(filepath.Join("testdata", n))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	entries := map[string][]byte{
		"db/clients/abc.json":            read("client.json"),
		"db/server/interfaces.json":      read("interfaces.json"),
		"db/server/keypair.json":         read("keypair.json"),
		"db/server/global_settings.json": read("global_settings.json"),
		"db/server/hashes.json":          []byte(`{"client":"none","server":"none"}`),
		"db/users/admin.json":            []byte(`{"username":"admin","password_hash":"x","admin":true}`),
	}
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for n, data := range entries {
		f, _ := w.Create(n)
		f.Write(data)
	}
	w.Close()
	files, err := Extract("db.zip", b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	x, err := ParseWGUI(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(x.Clients) != 1 || x.Clients[0].Name != "Laptop" || x.Server.PrivateKey == "" {
		t.Fatalf("hasil: %+v", x)
	}
	// Unggahan satuan: berkas perangkat tetap terbaca, hashes.json yang ikut terpilih dilewati.
	x, err = ParseWGUI(map[string][]byte{"abc.json": read("client.json"), "hashes.json": entries["db/server/hashes.json"]})
	if err != nil || len(x.Clients) != 1 {
		t.Fatalf("satuan: %+v %v", x, err)
	}
}

func TestDuplicate(t *testing.T) {
	existing := []Candidate{{Name: "lama", PublicKey: "k1", Address: "10.8.0.2"}}
	got := Check([]Candidate{{Name: "kunci", PublicKey: "k1", Address: "10.8.0.3"}, {Name: "ip", PublicKey: "k2", Address: "10.8.0.2"}, {Name: "luar", PublicKey: "k3", Address: "10.9.0.2"}, {Name: "baru", PublicKey: "k4", Address: "10.8.0.4"}}, existing, "10.8.0.1/24")
	for i, want := range []string{"kunci publik sudah ada", "alamat sudah ada", "alamat di luar subnet server", ""} {
		if got[i].Reason != want {
			t.Errorf("baris %d: %q", i, got[i].Reason)
		}
	}
}
