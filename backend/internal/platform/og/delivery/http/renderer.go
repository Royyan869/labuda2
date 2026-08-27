package http

import (
	"html/template"
	"io"
)

var ogTemplate = template.Must(template.New("og").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <meta property="og:type" content="website">
  <meta property="og:title" content="{{.Title}}">
  <meta property="og:description" content="{{.Description}}">
  <meta property="og:url" content="{{.URL}}">
  {{if .ImageURL}}<meta property="og:image" content="{{.ImageURL}}">{{end}}
  <meta name="twitter:card" content="summary_large_image">
  <meta name="twitter:title" content="{{.Title}}">
  <meta name="twitter:description" content="{{.Description}}">
  {{if .ImageURL}}<meta name="twitter:image" content="{{.ImageURL}}">{{end}}
</head>
<body>
</body>
</html>
`))

func renderPage(w io.Writer, m metadata) error {
	return ogTemplate.Execute(w, m)
}


