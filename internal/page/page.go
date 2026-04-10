package page

import (
	"fmt"
	"net/url"
	"path"
	"sort"
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
	return path.Join(p.UID, p.Slug, "index.html")
}

func (p NotePage) PublicPath() string {
	return path.Join(p.UID, p.Slug)
}

func (p NotePage) URL() string {
	return strings.TrimRight(p.SiteRootURL, "/") + "/" + p.PublicPath()
}

func (p NotePage) CanonicalPath() string {
	return p.PublicPath()
}

func SortNotePages(pages []NotePage) {
	sort.Slice(pages, func(i, j int) bool {
		return pages[i].PublishedAt.After(pages[j].PublishedAt)
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
		for _, t := range p.Tags {
			if t == tag {
				result = append(result, p)
				break
			}
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
	tags := make([]string, 0, len(seen))
	for t := range seen {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return tags
}

type RedirectPage struct {
	UID        string
	RedirectTo string
}

func (p RedirectPage) LocalPath() string {
	return path.Join(p.UID, "index.html")
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
