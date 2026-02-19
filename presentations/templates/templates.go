/*
Copyright © 2026 Patrick Hermann patrick.hermann@sva.de
*/

package templates

type TemplateDestination struct {
	Template    string
	Destination string
}

var slide = `{{ .slideContent }}"
`

var hugoTomlTmpl = `baseURL = "{{ .BaseURL }}"
languageCode = "{{ .LanguageCode }}"
title = "{{ .Title }}"
theme = [{{ range $i, $t := .Themes }}{{ if $i }}, {{ end }}"{{ $t }}"{{ end }}]

[author]
name = "{{ .Author.name }}"
email = "{{ .Author.email }}"

[params.author]
name = "{{ .Author.name }}"
email = "{{ .Author.email }}"

[module]
  proxy = "{{ .Module.Proxy }}"
  vendored = {{ .Module.Vendored }}

[markup.goldmark.renderer]
unsafe = {{ .Markup.Goldmark.Renderer.Unsafe }}

[outputFormats.Reveal]
baseName = "{{ .OutputFormats.Reveal.BaseName }}"
mediaType = "{{ .OutputFormats.Reveal.MediaType }}"
isHTML = {{ .OutputFormats.Reveal.IsHTML }}
`

var revealSlideTmpl = `+++
title = "{{ .Title }}"
outputs = [{{ range $i, $o := .Reveal.Outputs }}{{ if $i }}, {{ end }}"{{ $o }}"{{ end }}]

[logo]

[reveal_hugo]
history = {{ .Reveal.Hugo.History }}
slide_number = {{ .Reveal.Hugo.SlideNumber }}
custom_theme = "{{ .Reveal.Hugo.CustomTheme }}"
margin = {{ .Reveal.Hugo.Margin }}
mermaid = {{ .Reveal.Hugo.Mermaid }}
highlight_theme = "{{ .Reveal.Hugo.HighlightTheme }}"
transition = "{{ .Reveal.Hugo.Transition }}"
transition_speed = "{{ .Reveal.Hugo.TransitionSpeed }}"

[reveal_hugo.templates.hotpink]
class = "{{ .Reveal.Hugo.Templates.Hotpink.Class }}"
background = "{{ .Reveal.Hugo.Templates.Hotpink.Background }}"
+++

{{"{{"}}<  slide
id={{ .Slide.ID }}
background-color="{{ .Slide.BackgroundColor }}"
type="{{ .Slide.Type }}"
transition="{{ .Slide.Transition }}"
transition-speed="{{ .Slide.TransitionSpeed }}"
background-image="{{ .Slide.BackgroundImage }}"
background-size="{{ .Slide.BackgroundSize }}"
>{{"}}"}}<

{{"{{"}}% section %{{"}}"}}

{{ .Section.Spacer }}

{{ .Section.Content }}

{{"{{"}}% /section %{{"}}"}}
`

var PresentationFiles = []TemplateDestination{
	{
		Template:    hugoTomlTmpl,
		Destination: "hugo.toml",
	},
	{
		Template:    revealSlideTmpl,
		Destination: "_index.md",
	},
}
