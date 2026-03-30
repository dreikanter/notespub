package page

import (
	"testing"
	"time"
)

func TestNotePageLocalPath(t *testing.T) {
	p := NotePage{
		UID:  "20230130_3961",
		Slug: "rails-devise-manual-password-change",
	}
	want := "20230130_3961/rails-devise-manual-password-change/index.html"
	if got := p.LocalPath(); got != want {
		t.Errorf("LocalPath() = %q, want %q", got, want)
	}
}

func TestNotePageURL(t *testing.T) {
	p := NotePage{
		UID:         "20230130_3961",
		Slug:        "rails-devise-manual-password-change",
		SiteRootURL: "https://notes.musayev.com",
	}
	want := "https://notes.musayev.com/20230130_3961/rails-devise-manual-password-change"
	if got := p.URL(); got != want {
		t.Errorf("URL() = %q, want %q", got, want)
	}
}

func TestNotePagePublicPath(t *testing.T) {
	p := NotePage{
		UID:  "20230130_3961",
		Slug: "rails-devise-manual-password-change",
	}
	want := "20230130_3961/rails-devise-manual-password-change"
	if got := p.PublicPath(); got != want {
		t.Errorf("PublicPath() = %q, want %q", got, want)
	}
}

func TestRedirectPageLocalPath(t *testing.T) {
	p := RedirectPage{
		UID: "20230130_3961",
	}
	want := "20230130_3961/index.html"
	if got := p.LocalPath(); got != want {
		t.Errorf("LocalPath() = %q, want %q", got, want)
	}
}

func TestTagPageLocalPath(t *testing.T) {
	p := TagPage{Tag: "golang"}
	want := "tags/golang/index.html"
	if got := p.LocalPath(); got != want {
		t.Errorf("LocalPath() = %q, want %q", got, want)
	}
}

func TestSortNotePages(t *testing.T) {
	pages := []NotePage{
		{UID: "20230130_3961", PublishedAt: time.Date(2023, 1, 30, 0, 0, 0, 0, time.UTC)},
		{UID: "20230725_4324", PublishedAt: time.Date(2023, 7, 25, 0, 0, 0, 0, time.UTC)},
		{UID: "20230620_4164", PublishedAt: time.Date(2023, 6, 20, 0, 0, 0, 0, time.UTC)},
	}
	SortNotePages(pages)
	if pages[0].UID != "20230725_4324" {
		t.Errorf("pages[0].UID = %q, want 20230725_4324 (newest first)", pages[0].UID)
	}
	if pages[2].UID != "20230130_3961" {
		t.Errorf("pages[2].UID = %q, want 20230130_3961 (oldest last)", pages[2].UID)
	}
}
