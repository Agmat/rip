package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// wikiPageFixture is a trimmed excerpt of mtg.wiki's real rendered HTML for
// a set page (the infobox image, srcset and all) - enough to exercise the
// real regex against the real markup shape, not a hand-simplified stand-in.
const wikiPageFixture = `<html><body>
<figure><a href="/page/Special:FilePath/FND_Play_Booster.png">
<img alt="" src="https://files.mtg.wiki/thumb/FND_Play_Booster.png/200px-FND_Play_Booster.png"
srcset="https://files.mtg.wiki/thumb/FND_Play_Booster.png/300px-FND_Play_Booster.png 1.5x,
https://files.mtg.wiki/thumb/FND_Play_Booster.png/400px-FND_Play_Booster.png 2x"
width="200" height="364"></a></figure>
</body></html>`

const fakePNG = "\x89PNG\r\n\x1a\nfake-png-bytes"

func TestFetchPackImageFromWiki_FindsImage(t *testing.T) {
	var gotPagePath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/page/Foundations":
			gotPagePath = r.URL.Path
			_, _ = w.Write([]byte(wikiPageFixture))
		case "/FND_Play_Booster.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte(fakePNG))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	imp := &importer{httpClient: srv.Client()}
	img, found, err := imp.fetchPackImageFromWiki(context.Background(), srv.URL+"/page", srv.URL, "Foundations")
	if err != nil {
		t.Fatalf("fetchPackImageFromWiki: %v", err)
	}
	if !found {
		t.Fatal("found = false, want true")
	}
	if string(img) != fakePNG {
		t.Errorf("img = %q, want %q", img, fakePNG)
	}
	if gotPagePath != "/page/Foundations" {
		t.Errorf("requested page path = %q, want /page/Foundations", gotPagePath)
	}
}

func TestFetchPackImageFromWiki_SetNameIsURLEscaped(t *testing.T) {
	var gotEscapedPath, gotDecodedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEscapedPath = r.URL.EscapedPath() // the actual wire form
		gotDecodedPath = r.URL.Path
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	imp := &importer{httpClient: srv.Client()}
	_, _, err := imp.fetchPackImageFromWiki(context.Background(), srv.URL+"/page", srv.URL, "Kaladesh Remastered")
	if err == nil {
		t.Fatal("expected an error for a 404 page response")
	}
	if wantEscaped := "/page/" + url.PathEscape("Kaladesh Remastered"); gotEscapedPath != wantEscaped {
		t.Errorf("wire path = %q, want %q (the space must be percent-encoded on the wire)", gotEscapedPath, wantEscaped)
	}
	if wantDecoded := "/page/Kaladesh Remastered"; gotDecodedPath != wantDecoded {
		t.Errorf("decoded path = %q, want %q", gotDecodedPath, wantDecoded)
	}
}

func TestFetchPackImageFromWiki_NoImageOnPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><body>no pack image here</body></html>"))
	}))
	defer srv.Close()

	imp := &importer{httpClient: srv.Client()}
	img, found, err := imp.fetchPackImageFromWiki(context.Background(), srv.URL+"/page", srv.URL, "Some Obscure Set")
	if err != nil {
		t.Fatalf("fetchPackImageFromWiki: %v", err)
	}
	if found {
		t.Fatal("found = true, want false (decorative art missing is not an error)")
	}
	if img != nil {
		t.Errorf("img = %v, want nil", img)
	}
}

func TestFetchPackImageFromWiki_PageFetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	imp := &importer{httpClient: srv.Client()}
	if _, _, err := imp.fetchPackImageFromWiki(context.Background(), srv.URL+"/page", srv.URL, "Foundations"); err == nil {
		t.Fatal("expected an error for a 500 page response")
	}
}
