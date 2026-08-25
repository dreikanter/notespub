package page

import (
	"fmt"
	"maps"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"
)

// NotePage represents a single public note.
type NotePage struct {
	UID         string
	ShortUID    string
	Slug        string
	Title       string
	Description string
	Tags        []string
	Body        string // rendered HTML
	PublishedAt time.Time
	SiteRootURL string
	Attachments []Attachment
}

// Attachment is a downloaded image file associated with a note.
type Attachment struct {
	FileName string
	PageUID  string
}

func (p NotePage) LocalPath() string {
	return path.Join(p.Slug, "index.html")
}

func (p NotePage) PublicPath() string {
	return p.Slug
}

func (p NotePage) URL() string {
	return strings.TrimRight(p.SiteRootURL, "/") + "/" + p.PublicPath()
}

func (p NotePage) CanonicalPath() string {
	return p.PublicPath()
}

func SortNotePages(pages []NotePage) {
	slices.SortFunc(pages, func(a, b NotePage) int {
		return b.PublishedAt.Compare(a.PublishedAt)
	})
}

func RelatedTo(pages []NotePage, target NotePage) []NotePage {
	tagSet := make(map[string]struct{}, len(target.Tags))
	for _, t := range target.Tags {
		tagSet[t] = struct{}{}
	}
	var related []NotePage
	for _, p := range pages {
		if p.UID == target.UID {
			continue
		}
		for _, t := range p.Tags {
			if _, ok := tagSet[t]; ok {
				related = append(related, p)
				break
			}
		}
	}
	return related
}

func TaggedPages(pages []NotePage, tag string) []NotePage {
	var result []NotePage
	for _, p := range pages {
		if slices.Contains(p.Tags, tag) {
			result = append(result, p)
		}
	}
	return result
}

func AllTags(pages []NotePage) []string {
	seen := make(map[string]struct{})
	for _, p := range pages {
		for _, t := range p.Tags {
			seen[t] = struct{}{}
		}
	}
	return slices.Sorted(maps.Keys(seen))
}

type RedirectPage struct {
	FromPath   string
	RedirectTo string
}

func (p RedirectPage) LocalPath() string {
	return path.Join(p.FromPath, "index.html")
}

type TagPage struct {
	Tag string
}

func (p TagPage) LocalPath() string {
	return fmt.Sprintf("tags/%s/index.html", url.PathEscape(p.Tag))
}

func (p TagPage) PublicPath() string {
	return fmt.Sprintf("tags/%s", url.PathEscape(p.Tag))
}

func (p TagPage) CanonicalPath() string {
	return p.PublicPath()
}
