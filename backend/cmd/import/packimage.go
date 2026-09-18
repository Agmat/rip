package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
)

const (
	// DefaultMTGWikiPageBaseURL and DefaultMTGWikiFilesBaseURL are mtg.wiki's
	// real hosts. Overridable so tests can point fetchPackImageFromWiki at
	// an httptest server instead of the real network.
	DefaultMTGWikiPageBaseURL  = "https://mtg.wiki/page"
	DefaultMTGWikiFilesBaseURL = "https://files.mtg.wiki"

	// userAgent identifies this project, matching internal/scryfall's own
	// policy of always sending a descriptive one - mtg.wiki's robots.txt
	// shows they pay attention to how bots use the site.
	userAgent = "rip-import/0.1 (+https://github.com/Agmat/rip)"
)

// playBoosterImageRe finds the Play Booster product image on a rendered
// mtg.wiki set page: a thumbnail URL under <filesBaseURL>/thumb/, whose
// filename ends in _Play_Booster.png. Hardcoded to "Play Booster" since
// that's the only booster type this importer opens (see boosterType);
// generalize this if a second type is ever added.
var playBoosterImageRe = regexp.MustCompile(`/thumb/([^"/]+_Play_Booster\.png)/`)

// fetchPackImageFromWiki finds and downloads a set's Play Booster product
// photo from mtg.wiki. Unlike TCGplayer's studio JPEGs, these are PNGs
// with real alpha transparency already cut around the pack, so the bytes
// are stored exactly as served - no background-removal processing needed.
//
// Wiki filenames are hand-curated by editors and don't reliably match a
// set's official MTGJSON/Scryfall code (Foundations, for one, is filed as
// "FND_Play_Booster.png" rather than "FDN") - so this resolves by fetching
// the set's page by its plain name (mtg.wiki reliably redirects that to the
// right article) and scraping the image out of the rendered page, rather
// than constructing the filename directly.
//
// robots.txt disallows /api.php, so this deliberately doesn't use the
// wiki's search API - a plain page fetch is allowed. false, nil means "no
// pack image found for this set", not an error: this is decorative art and
// must never block getting the cards in.
func (imp *importer) fetchPackImageFromWiki(ctx context.Context, pageBaseURL, filesBaseURL, setName string) ([]byte, bool, error) {
	pageURL := pageBaseURL + "/" + url.PathEscape(setName)

	html, err := imp.fetchBytes(ctx, pageURL)
	if err != nil {
		return nil, false, fmt.Errorf("fetch wiki page: %w", err)
	}

	m := playBoosterImageRe.FindSubmatch(html)
	if m == nil {
		return nil, false, nil
	}
	imageURL := filesBaseURL + "/" + string(m[1])

	img, err := imp.fetchBytes(ctx, imageURL)
	if err != nil {
		return nil, false, fmt.Errorf("fetch pack image: %w", err)
	}
	return img, true, nil
}

func (imp *importer) fetchBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := imp.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}
