package routes

import (
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScriptTag_External_NoIntegrity(t *testing.T) {
	out := ScriptTag("https://cdn.example.com/lib.js")
	if strings.Contains(string(out), "integrity=") {
		t.Fatalf("external script should not have integrity: %q", out)
	}
}

func TestScriptTag_Local_ComputesIntegrity(t *testing.T) {
	// create a temporary local asset under web/assets/js
	dir := filepath.Join("web", "assets", "js")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdirall: %v", err)
	}
	fname := "test-sri.js"
	fpath := filepath.Join(dir, fname)
	content := []byte("console.log('sri-test');\n")
	if err := os.WriteFile(fpath, content, 0644); err != nil {
		t.Fatalf("writefile: %v", err)
	}
	defer os.Remove(fpath)

	// compute expected sha384
	h := sha512.New384()
	h.Write(content)
	sum := h.Sum(nil)
	expected := "sha384-" + base64.StdEncoding.EncodeToString(sum)

	src := "/assets/js/" + fname
	out := ScriptTag(src)
	want := fmt.Sprintf(`<script src="%s" integrity="%s" crossorigin="anonymous"></script>`, src, expected)
	if string(out) != want {
		t.Fatalf("unexpected output, got: %q, want: %q", out, want)
	}
}

func TestScriptTag_EscapesAttributes(t *testing.T) {
	// Attributes containing characters that could be problematic should be escaped.
	out := ScriptTag(`/" onerror="alert(1)`) // intentionally malicious-like
	// ensure no raw attribute injection like onerror="...
	if strings.Contains(string(out), `onerror="`) {
		t.Fatalf("attribute injection detected in output: %q", out)
	}
	// Ensure it is template.HTML (safe to insert into template)
	_ = template.HTML(out)
}
