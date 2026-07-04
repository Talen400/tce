package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Profile struct {
	Root        string
	Language    string
	Framework   string
	BuildSystem string
	TestRunner  string
	Linter      string
	Formatter   string
	PackageName string
	Deps        []string
	HasGit      bool
	KeyFiles    []string
}

type rule struct {
	files   []string
	lang    string
	fw      string
	build   string
	test    string
	lint    string
	fmt     string
	pkgFunc func(root string) string
}

var rules = []rule{
	{
		files: []string{"go.mod"},
		lang:  "Go",
		build: "go build",
		test:  "go test",
		lint:  "golangci-lint",
		fmt:   "gofmt",
		pkgFunc: func(root string) string {
			data, _ := os.ReadFile(filepath.Join(root, "go.mod"))
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "module ") {
					return strings.TrimSpace(strings.TrimPrefix(line, "module "))
				}
			}
			return "unknown"
		},
	},
	{
		files: []string{"package.json"},
		lang:  "JavaScript",
		build: "npm run build",
		test:  "npm test",
		lint:  "eslint",
		fmt:   "prettier",
		pkgFunc: func(root string) string {
			return filepath.Base(root)
		},
	},
	{
		files: []string{"Cargo.toml"},
		lang:  "Rust",
		build: "cargo build",
		test:  "cargo test",
		lint:  "clippy",
		fmt:   "rustfmt",
		pkgFunc: func(root string) string {
			data, _ := os.ReadFile(filepath.Join(root, "Cargo.toml"))
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "name = ") {
					name := strings.TrimPrefix(line, "name = \"")
					return strings.TrimRight(name, "\"")
				}
			}
			return filepath.Base(root)
		},
	},
	{
		files:   []string{"pyproject.toml"},
		lang:    "Python",
		build:   "pip install -e .",
		test:    "pytest",
		lint:    "ruff",
		fmt:     "ruff format",
		pkgFunc: func(root string) string { return filepath.Base(root) },
	},
	{
		files:   []string{"Gemfile"},
		lang:    "Ruby",
		build:   "bundle install",
		test:    "rspec",
		lint:    "rubocop",
		fmt:     "rubocop -A",
		pkgFunc: func(root string) string { return filepath.Base(root) },
	},
	{
		files:   []string{"composer.json"},
		lang:    "PHP",
		build:   "composer install",
		test:    "phpunit",
		lint:    "phpcs",
		fmt:     "phpcbf",
		pkgFunc: func(root string) string { return filepath.Base(root) },
	},
	{
		files:   []string{"build.gradle", "build.gradle.kts"},
		lang:    "Kotlin/Java",
		build:   "gradle build",
		test:    "gradle test",
		lint:    "ktlint",
		fmt:     "ktlint -F",
		pkgFunc: func(root string) string { return filepath.Base(root) },
	},
	{
		files: []string{"Makefile"},
		lang:  "C",
		build: "make",
		test:  "make test",
		pkgFunc: func(root string) string {
			return filepath.Base(root)
		},
	},
	{
		files:   []string{"CMakeLists.txt"},
		lang:    "C/C++",
		build:   "cmake --build build",
		test:    "ctest",
		pkgFunc: func(root string) string { return filepath.Base(root) },
	},
}

func Detect(root string) *Profile {
	p := &Profile{
		Root:     root,
		KeyFiles: make([]string, 0),
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		p.Language = "Unknown"
		return p
	}

	fileSet := make(map[string]bool)
	for _, e := range entries {
		fileSet[e.Name()] = true
	}

	for _, r := range rules {
		for _, f := range r.files {
			if fileSet[f] {
				p.Language = r.lang
				p.Framework = r.fw
				p.BuildSystem = r.build
				p.TestRunner = r.test
				p.Linter = r.lint
				p.Formatter = r.fmt
				p.KeyFiles = append(p.KeyFiles, f)
				if r.pkgFunc != nil {
					p.PackageName = r.pkgFunc(root)
				}

				if fileSet["tsconfig.json"] || fileSet["tsconfig.ts"] {
					p.Language = "TypeScript"
				}
				if fileSet["jest.config.js"] || fileSet["jest.config.ts"] || fileSet["vitest.config.ts"] {
					p.TestRunner = "jest/vitest"
				}
				if fileSet[".eslintrc.js"] || fileSet[".eslintrc.json"] || fileSet["eslint.config.js"] {
					p.Linter = "eslint"
				}
				if fileSet["next.config.js"] || fileSet["next.config.ts"] || fileSet["next.config.mjs"] {
					p.Framework = "Next.js"
				}
				if fileSet["vite.config.ts"] || fileSet["vite.config.js"] {
					p.Framework = "Vite"
				}
				if fileSet["tailwind.config.ts"] || fileSet["tailwind.config.js"] {
					p.Framework = p.Framework + " + Tailwind"
					if strings.HasPrefix(p.Framework, " + ") {
						p.Framework = "Tailwind"
					}
				}
				if fileSet["django"] || fileSet["manage.py"] {
					p.Framework = "Django"
				}
				break
			}
		}
		if p.Language != "" {
			break
		}
	}

	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		p.HasGit = true
	}

	if p.Language == "" {
		for _, e := range entries {
			if !e.IsDir() {
				ext := filepath.Ext(e.Name())
				switch ext {
				case ".go":
					p.Language = "Go"
				case ".py":
					p.Language = "Python"
				case ".rs":
					p.Language = "Rust"
				case ".ts", ".tsx":
					p.Language = "TypeScript"
				case ".js", ".jsx":
					p.Language = "JavaScript"
				case ".java":
					p.Language = "Java"
				case ".rb":
					p.Language = "Ruby"
				case ".php":
					p.Language = "PHP"
				case ".c", ".h":
					p.Language = "C"
				case ".cpp", ".hpp", ".cc":
					p.Language = "C++"
				case ".swift":
					p.Language = "Swift"
				case ".kt", ".kts":
					p.Language = "Kotlin"
				}
				if p.Language != "" {
					p.KeyFiles = append(p.KeyFiles, e.Name())
					break
				}
			}
		}
	}

	if p.Language == "" {
		p.Language = "Unknown"
	}

	return p
}

func (p *Profile) String() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Language: %s\n", p.Language))
	if p.Framework != "" {
		b.WriteString(fmt.Sprintf("Framework: %s\n", p.Framework))
	}
	if p.BuildSystem != "" {
		b.WriteString(fmt.Sprintf("Build: %s\n", p.BuildSystem))
	}
	if p.TestRunner != "" {
		b.WriteString(fmt.Sprintf("Test: %s\n", p.TestRunner))
	}
	if p.Linter != "" {
		b.WriteString(fmt.Sprintf("Lint: %s\n", p.Linter))
	}
	if p.HasGit {
		b.WriteString("Git: yes\n")
	}
	return b.String()
}

func (p *Profile) Summary() string {
	parts := []string{p.Language}
	if p.Framework != "" {
		parts = append(parts, p.Framework)
	}
	return strings.Join(parts, " + ")
}
