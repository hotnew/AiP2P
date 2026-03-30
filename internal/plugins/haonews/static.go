package newsplugin

import (
	"io/fs"
	"net/http"
)

func NoStoreStaticHandler(staticFS fs.FS) http.Handler {
	fileHandler := http.StripPrefix("/static/", http.FileServer(http.FS(staticFS)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		fileHandler.ServeHTTP(w, r)
	})
}
