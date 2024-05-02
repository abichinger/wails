package assetserver

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/wailsapp/wails/v3/internal/assetserver/bundledassets"
)

type BundledAssetServer struct {
	handler http.Handler
}

func NewBundledAssetFileServer(fs fs.FS) *BundledAssetServer {
	return &BundledAssetServer{
		handler: NewAssetFileServer(fs),
	}
}

func (b *BundledAssetServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if strings.HasPrefix(req.URL.Path, "/wails/") {
		// Strip the /wails prefix
		req.URL.Path = req.URL.Path[6:]
		switch req.URL.Path {
		case "/runtime.js":
			rw.Header().Set("Content-Type", "application/javascript")
			rw.Write([]byte(bundledassets.RuntimeJS))
			return
		case "/importmap.js":
			rw.Header().Set("Content-Type", "application/javascript")
			rw.Write([]byte(bundledassets.ImportmapJS))
			return
		case "/es-module-shims.js":
			rw.Header().Set("Content-Type", "application/javascript")
			rw.Write([]byte(bundledassets.ESModuleShims))
			return
		}
		return
	}
	b.handler.ServeHTTP(rw, req)
}
