// AI generated code, human reviewed

package routes

import (
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"html"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// sriCache caches computed SRI integrity strings keyed by the src path.
var sriCache sync.Map // map[string]string

// computeLocalSRI computes the sha384 SRI for a local asset path.
// It supports URLs starting with /assets/ (mapped to web/assets/) and /dist/ (mapped to dist/).
func computeLocalSRI(src string) (string, error) {
	// only compute for paths we expect to be local
	if !(strings.HasPrefix(src, "/assets/") || strings.HasPrefix(src, "/dist/")) {
		return "", nil
	}

	// Map URL path to filesystem path
	// drop leading '/'
	rel := strings.TrimPrefix(src, "/")
	var fsPath string
	if strings.HasPrefix(src, "/assets/") {
		// local assets live under web/assets/
		fsPath = filepath.Join("web", rel)
	} else {
		// dist maps to project-level dist/
		fsPath = filepath.Join(rel)
	}

	f, err := os.Open(fsPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha512.New384()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	sum := h.Sum(nil)
	b64 := base64.StdEncoding.EncodeToString(sum)
	return "sha384-" + b64, nil
}

// ScriptTag returns a safe HTML script tag for use in html/templates.
// It accepts the script src and automatically calculates and caches the
// SRI integrity hash for local assets under /assets/ or /dist/.
// When an integrity is available it adds crossorigin="anonymous".
func ScriptTag(src string) template.HTML {
	// Escape attribute values to avoid injection, then build the tag.
	escSrc := html.EscapeString(src)

	var integrity string
	if v, ok := sriCache.Load(src); ok {
		integrity = v.(string)
	} else {
		sri, err := computeLocalSRI(src)
		if err == nil && sri != "" {
			sriCache.Store(src, sri)
			integrity = sri
		}
	}

	attr := ""
	crossorigin := ""
	if integrity != "" {
		attr = fmt.Sprintf(" integrity=\"%s\"", html.EscapeString(integrity))
		crossorigin = " crossorigin=\"anonymous\""
	}

	tag := fmt.Sprintf("<script src=\"%s\"%s%s></script>", escSrc, attr, crossorigin)
	return template.HTML(tag)
}

// TemplateFuncs returns a FuncMap with template helpers for routes templates.
func TemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"script_tag": ScriptTag,
	}
}
