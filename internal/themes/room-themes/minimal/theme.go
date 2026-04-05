package minimal

import (
	"embed"
	"html/template"
	"io/fs"
)

//go:embed roomtheme.json web/templates/room_channel.html
var assets embed.FS

func Template(funcMap template.FuncMap) (*template.Template, error) {
	if funcMap == nil {
		funcMap = template.FuncMap{}
	}
	return template.New("room_channel.html").Funcs(funcMap).ParseFS(assets, "web/templates/room_channel.html")
}

func Assets() fs.FS {
	return assets
}
