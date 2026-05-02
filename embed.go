package npub

import (
	"embed"

	"github.com/dreikanter/npub/internal/build"
)

//go:embed templates/*
var TemplateFS embed.FS

//go:embed style.css
var StyleCSS []byte

//go:embed favicon.svg
var FaviconSVG []byte

//go:embed icons/sun.svg
var SunSVG []byte

//go:embed icons/moon.svg
var MoonSVG []byte

//go:embed npub.yml.sample
var SampleConfig []byte

var Assets = build.Assets{
	Templates:  TemplateFS,
	StyleCSS:   StyleCSS,
	FaviconSVG: FaviconSVG,
	SunSVG:     SunSVG,
	MoonSVG:    MoonSVG,
}
