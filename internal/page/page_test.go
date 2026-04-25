package page

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNotePageLocalPath(t *testing.T) {
	p := NotePage{
		UID:  "20230130_3961",
		Slug: "rails-devise-manual-password-change",
	}

	assert.Equal(t, "20230130_3961/rails-devise-manual-password-change/index.html", p.LocalPath())
}

func TestNotePageURL(t *testing.T) {
	p := NotePage{
		UID:         "20230130_3961",
		Slug:        "rails-devise-manual-password-change",
		SiteRootURL: "https://notes.musayev.com",
	}

	assert.Equal(t, "https://notes.musayev.com/20230130_3961/rails-devise-manual-password-change", p.URL())
}

func TestNotePagePublicPath(t *testing.T) {
	p := NotePage{
		UID:  "20230130_3961",
		Slug: "rails-devise-manual-password-change",
	}

	assert.Equal(t, "20230130_3961/rails-devise-manual-password-change", p.PublicPath())
}

func TestRedirectPageLocalPath(t *testing.T) {
	p := RedirectPage{
		UID: "20230130_3961",
	}

	assert.Equal(t, "20230130_3961/index.html", p.LocalPath())
}

func TestTagPageLocalPath(t *testing.T) {
	p := TagPage{Tag: "golang"}

	assert.Equal(t, "tags/golang/index.html", p.LocalPath())
}

func TestSortNotePages(t *testing.T) {
	pages := []NotePage{
		{UID: "20230130_3961", PublishedAt: time.Date(2023, 1, 30, 0, 0, 0, 0, time.UTC)},
		{UID: "20230725_4324", PublishedAt: time.Date(2023, 7, 25, 0, 0, 0, 0, time.UTC)},
		{UID: "20230620_4164", PublishedAt: time.Date(2023, 6, 20, 0, 0, 0, 0, time.UTC)},
	}

	SortNotePages(pages)

	assert.Equal(t, "20230725_4324", pages[0].UID)
	assert.Equal(t, "20230130_3961", pages[2].UID)
}
