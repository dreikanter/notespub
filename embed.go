package npub

import "embed"

//go:embed templates/*
var TemplateFS embed.FS

//go:embed style.css
var StyleCSS []byte
