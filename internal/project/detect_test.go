package project

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectGo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module test\n")
	p := Detect(dir)
	if p.Language != "Go" {
		t.Errorf("expected Go, got %s", p.Language)
	}
	if p.Root != dir {
		t.Errorf("root mismatch: %s vs %s", p.Root, dir)
	}
}

func TestDetectPython(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", "")
	p := Detect(dir)
	if p.Language != "Python" {
		t.Errorf("expected Python, got %s", p.Language)
	}
}

func TestDetectRust(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Cargo.toml", "[package]\nname = \"test\"\n")
	p := Detect(dir)
	if p.Language != "Rust" {
		t.Errorf("expected Rust, got %s", p.Language)
	}
}

func TestDetectJS(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", "{}")
	p := Detect(dir)
	if p.Language != "JavaScript" && p.Language != "TypeScript" {
		t.Errorf("expected JavaScript or TypeScript, got %s", p.Language)
	}
}

func TestDetectEmpty(t *testing.T) {
	dir := t.TempDir()
	p := Detect(dir)
	if p.Language != "Unknown" {
		t.Errorf("expected Unknown, got %s", p.Language)
	}
}

func TestDetectNonExistent(t *testing.T) {
	p := Detect("/tmp/tce-nonexistent-" + t.Name())
	if p.Language != "Unknown" {
		t.Errorf("expected Unknown for non-existent dir, got %s", p.Language)
	}
}

func TestDetectRelativePath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module test\n")
	wd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(wd)

	p := Detect(".")
	if p.Language != "Go" {
		t.Errorf("expected Go from relative path, got %s", p.Language)
	}
}

func TestDetectPriorityGoBeforeJS(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module test\n")
	writeFile(t, dir, "package.json", "{}")

	p := Detect(dir)
	if p.Language != "Go" {
		t.Errorf("expected Go (rule order), got %s", p.Language)
	}
}

func TestDetectFileContents(t *testing.T) {
	dir := t.TempDir()
	content := `module github.com/user/repo

go 1.24

require github.com/foo v1.0.0
`
	writeFile(t, dir, "go.mod", content)

	p := Detect(dir)
	if p.Language != "Go" {
		t.Errorf("expected Go, got %s", p.Language)
	}
	if p.Framework != "" {
		t.Errorf("expected empty framework, got %s", p.Framework)
	}
}

func TestDetectCMakefileWithCFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Makefile", "all:\n\tgcc -Wall -Wextra -Werror main.c\n")
	writeFile(t, dir, "main.c", "int main() { return 0; }")
	p := Detect(dir)
	if p.Language != "C" {
		t.Errorf("expected C, got %s", p.Language)
	}
}

func TestDetectCMakefileNoCFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Makefile", "all:\n\techo hello\n")
	p := Detect(dir)
	if p.Language != "C" {
		t.Errorf("expected C from Makefile, got %s", p.Language)
	}
}

func TestDetectCFtPrefix(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Makefile", "all:\n\tgcc -Wall -Wextra -Werror ft_printf.c\n")
	writeFile(t, dir, "ft_printf.c", "int ft_printf() { return 0; }")
	writeFile(t, dir, "ft_printf.h", "#ifndef FT_PRINTF_H\n#define FT_PRINTF_H\n#endif")
	p := Detect(dir)
	if p.Language != "C" {
		t.Errorf("expected C, got %s", p.Language)
	}
	if p.Framework != "" {
		t.Errorf("expected no framework, got %s", p.Framework)
	}
}

func TestDetectCExtensionFallback(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.c", "int main() { return 0; }")
	p := Detect(dir)
	if p.Language != "C" {
		t.Errorf("expected C from .c extension, got %s", p.Language)
	}
}
