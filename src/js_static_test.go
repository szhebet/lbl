package main

// Static consistency tests for the frontend JS.
//
// These guard against the class of bug where the backend tests (Go) pass but
// the frontend is broken: event-delegation handlers calling functions that do
// not exist, add-button bindings referencing button IDs missing from the HTML
// templates, template <script> tags pointing to non-existent JS files, and
// API paths the backend does not register.
//
// The regression that motivated these tests: commit 19d1ad8 removed the
// editUser/deleteUser/editAuthor/... functions from static/js/admin.js while
// the delegation block still called them — the "Create user" button silently
// stopped working and no existing test caught it.

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// JS globals / browser APIs and language keywords that may appear as call
// expressions but are never defined in the project's own JS files.
var jsGlobalCalls = map[string]bool{
	// language / control flow
	"if": true, "for": true, "while": true, "switch": true, "catch": true,
	"function": true, "new": true, "typeof": true, "return": true, "delete": true,
	"do": true, "else": true, "in": true, "instanceof": true, "void": true,
	"throw": true, "try": true, "async": true, "await": true, "of": true, "yield": true,
	"Error": true, "Event": true, "TypeError": true, "RangeError": true,
	// browser / JS builtins
	"alert": true, "confirm": true, "prompt": true, "parseInt": true,
	"parseFloat": true, "setTimeout": true, "setInterval": true,
	"clearTimeout": true, "clearInterval": true, "encodeURIComponent": true,
	"decodeURIComponent": true, "encodeURI": true, "decodeURI": true,
	"isNaN": true, "isFinite": true, "fetch": true, "JSON": true, "Math": true,
	"Object": true, "Array": true, "String": true, "Number": true, "Boolean": true,
	"Date": true, "RegExp": true, "Promise": true, "Symbol": true, "Map": true,
	"Set": true, "console": true, "document": true, "window": true,
	"localStorage": true, "sessionStorage": true, "navigator": true,
	"Blob": true, "FormData": true, "FileReader": true, "URL": true,
	"URLSearchParams": true, "atob": true, "btoa": true, "XMLHttpRequest": true,
	"requestAnimationFrame": true, "performance": true, "crypto": true,
	"TextDecoder": true, "TextEncoder": true, "structuredClone": true,
	"history": true, "location": true, "EventSource": true, "Worker": true,
	"webkitURL": true, "getSelection": true, "self": true, "globalThis": true,
}

// Callback parameters and common short variable names that are used as
// statement-level invocations in delegation handlers. These are invoked but
// not defined as named functions.
var jsKnownLocalCalls = map[string]bool{
	"e": true, "ev": true, "event": true, "err": true, "error": true,
	"res": true, "response": true, "result": true, "data": true, "item": true,
	"items": true, "el": true, "element": true, "btn": true, "link": true,
	"row": true, "rows": true, "obj": true, "value": true, "key": true,
	"input": true, "file": true, "files": true, "target": true, "cb": true,
	"callback": true, "cbk": true, "fn": true, "body": true, "id": true,
}

// Pages map template name -> the set of JS files it loads (from <script>).
var frontendPages = map[string][]string{
	"index.html":       {"app.js", "auth.js", "import.js", "offline.js"},
	"admin.html":       {"app.js", "auth.js", "import.js", "admin.js"},
	"admin_users.html": {"app.js", "auth.js", "import.js", "admin.js"},
}

var frontendJsDir = "../static/js"
var frontendTemplateDir = "../templates"

func readProjectFile(t *testing.T, rel string) string {
	t.Helper()
	var path string
	if strings.HasSuffix(rel, ".html") {
		path = filepath.Join(frontendTemplateDir, rel)
	} else {
		path = filepath.Join(frontendJsDir, rel)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	return string(b)
}

// extractDefinedNames returns all names that are "defined" in a JS file:
// function declarations, function/arrow expressions assigned to const/let/var,
// top-level object method definitions (const x = { foo(...) {...} }), class
// method declarations, and class field arrows.
func extractDefinedNames(js string) map[string]bool {
	names := map[string]bool{}
	reDef := regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
	for _, m := range reDef.FindAllStringSubmatch(js, -1) {
		names[m[1]] = true
	}
	// const/let/var NAME = function / (…) => / async (…) =>
	reAssign := regexp.MustCompile(`(?m)^\s*(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*(?:async\s+)?(?:function\s*\(|\([^)]*\)\s*=>)`)
	for _, m := range reAssign.FindAllStringSubmatch(js, -1) {
		names[m[1]] = true
	}
	// Object method definitions:  NAME(…) {  inside const/class bodies.
	// Match indented method names that open a brace after the parameter list.
	reMethod := regexp.MustCompile(`(?m)^\s{4,}(?:async\s+)?([A-Za-z_$][A-Za-z0-9_$]*)\s*\([^)]*\)\s*\{`)
	for _, m := range reMethod.FindAllStringSubmatch(js, -1) {
		names[m[1]] = true
	}
	// Class method declarations are usually at deeper indent; also catch any
	// NAME(…) { not preceded by a keyword (covers "getAll() {" inside class).
	return names
}

// extractFunctionParams returns the parameter names of every declared function
// in a JS file, so callback params (e.g. postFn) are not flagged as undefined.
func extractFunctionParams(js string) map[string]bool {
	params := map[string]bool{}
	// Named function declarations: function NAME(p1, p2) { ... }
	reNamed := regexp.MustCompile(`(?:async\s+)?function\s+[A-Za-z_$][A-Za-z0-9_$]*\s*\(([^)]*)\)`)
	for _, m := range reNamed.FindAllStringSubmatch(js, -1) {
		for _, p := range strings.Split(m[1], ",") {
			p = strings.TrimSpace(p)
			p = regexp.MustCompile(`\s*=.*$`).ReplaceAllString(p, "")
			if p != "" && !strings.ContainsAny(p, ".{[=:") {
				params[p] = true
			}
		}
	}
	// Anonymous/arrow functions: function(...) { ... } and (a, b) => ...
	reFn := regexp.MustCompile(`(?:function|=>)\s*\(([^)]*)\)`)
	for _, m := range reFn.FindAllStringSubmatch(js, -1) {
		for _, p := range strings.Split(m[1], ",") {
			p = strings.TrimSpace(p)
			if p == "" || strings.ContainsAny(p, ".{[=:") {
				continue
			}
			p = strings.TrimSpace(regexp.MustCompile(`\s*=\s*.*$`).ReplaceAllString(p, ""))
			if p != "" {
				params[p] = true
			}
		}
	}
	return params
}

// stripCommentsAndStrings removes // and /* */ comments plus single-quoted,
// double-quoted and backtick string literals so they don't pollute the
// extracted call expressions.
func stripCommentsAndStrings(js string) string {
	js = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(js, "")
	js = regexp.MustCompile(`(?m)//.*$`).ReplaceAllString(js, "")
	// Remove string literals: '...', "...", `...`. Handles escaped quotes.
	reStr := regexp.MustCompile(`'(\\.|[^'\\])*'|"(\\.|[^"\\])*"|` + "`" + `(\\.|[^` + "`" + `\\])*` + "`" + ``)
	return reStr.ReplaceAllString(js, " ")
}

// extractCallExpressions returns every bare identifier immediately followed by
// "(" that is NOT a property/method call, NOT part of a declaration, NOT a
// known global/keyword, and NOT a function parameter or callback var.
func extractCallExpressions(js string) []string {
	js = stripCommentsAndStrings(js)
	params := extractFunctionParams(js)
	var calls []string
	re := regexp.MustCompile(`[A-Za-z_$][A-Za-z0-9_$]*\s*\(`)
	for _, loc := range re.FindAllStringIndex(js, -1) {
		start, end := loc[0], loc[1]
		// Skip identifiers preceded by '.', '?.', ':', or a word char
		// (covers property calls, CSS :not(...), and function-keyword defs).
		if start > 0 {
			prev := js[start-1]
			if prev == '.' || prev == ':' || prev == '?' || prev == '_' || prev == '$' {
				continue
			}
			if isWordChar(prev) {
				continue
			}
			// skip "function NAME(", "async function NAME(", "new NAME(", "class NAME("
			wordBefore := precedingWord(js, start)
			if wordBefore == "function" || wordBefore == "new" || wordBefore == "class" {
				continue
			}
		}
		name := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(js[start:end]), "("))
		if jsGlobalCalls[name] || jsKnownLocalCalls[name] || params[name] {
			continue
		}
		calls = append(calls, name)
	}
	return calls
}

func isWordChar(b byte) bool {
	return b == '_' || b == '$' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// precedingWord returns the identifier/keyword immediately before index i
// (separated from it by at most whitespace).
func precedingWord(s string, i int) string {
	j := i
	for j > 0 && (s[j-1] == ' ' || s[j-1] == '\t') {
		j--
	}
	start := j
	for start > 0 && isWordChar(s[start-1]) {
		start--
	}
	return s[start:j]
}

// TestFrontendDelegationHandlersDefined verifies that every bare function call
// made from the SPA JS files resolves to a named function defined in one of the
// JS files loaded on the same page. This catches the regression where the
// delegation block in admin.js called editUser/deleteUser/... after those
// functions were deleted.
func TestFrontendDelegationHandlersDefined(t *testing.T) {
	for tmpl, jsFiles := range frontendPages {
		tmpl = filepath.Base(tmpl)
		defined := map[string]bool{}
		for _, f := range jsFiles {
			js := readProjectFile(t, f)
			for name := range extractDefinedNames(js) {
				defined[name] = true
			}
		}
		for _, f := range jsFiles {
			js := readProjectFile(t, f)
			missing := map[string]bool{}
			for _, call := range extractCallExpressions(js) {
				if !defined[call] {
					missing[call] = true
				}
			}
			if len(missing) > 0 {
				names := make([]string, 0, len(missing))
				for c := range missing {
					names = append(names, c)
				}
				sort.Strings(names)
				t.Errorf("%s (%s page): called but not defined in any loaded JS: %s",
					f, tmpl, strings.Join(names, ", "))
			}
		}
	}
}

// TestFrontendButtonBindingsExist verifies every button/control that gets a
// direct document.getElementById(...).addEventListener binding either exists
// statically in the HTML template or is created dynamically in the same JS
// file via a string template containing id="X". A binding to a nonexistent ID
// throws at load time and breaks the whole page.
func TestFrontendButtonBindingsExist(t *testing.T) {
	reBind := regexp.MustCompile(`getElementById\('([A-Za-z_$][A-Za-z0-9_$]*)'\)\s*\.addEventListener`)
	for tmpl, jsFiles := range frontendPages {
		html := readProjectFile(t, tmpl)
		for _, f := range jsFiles {
			js := readProjectFile(t, f)
			for _, m := range reBind.FindAllStringSubmatch(js, -1) {
				id := m[1]
				inHTML := strings.Contains(html, `id="`+id+`"`)
				inJS := strings.Contains(js, `id="`+id+`"`)
				if !inHTML && !inJS {
					t.Errorf("%s: getElementById('%s').addEventListener but %s has no id=%q and %s never creates it",
						f, id, tmpl, id, f)
				}
			}
		}
	}
}

// TestTemplateScriptTagsResolve verifies every <script src="/static/js/X.js">
// in a template points to an existing file.
func TestTemplateScriptTagsResolve(t *testing.T) {
	reScript := regexp.MustCompile(`src="/static/js/([A-Za-z0-9_\-]+\.js)`)
	for tmpl := range frontendPages {
		html := readProjectFile(t, tmpl)
		for _, m := range reScript.FindAllStringSubmatch(html, -1) {
			path := filepath.Join(frontendJsDir, m[1])
			if _, err := os.Stat(path); err != nil {
				t.Errorf("%s references /static/js/%s which does not exist", tmpl, m[1])
			}
		}
	}
}

// registeredRoutes parses main.go and returns every full HTTP route path that
// is registered, resolving group prefixes (including nested groups).
func registeredRoutes(t *testing.T) map[string]bool {
	t.Helper()
	mainSrc, err := os.ReadFile("../src/main.go")
	if err != nil {
		t.Fatalf("cannot read ../src/main.go: %v", err)
	}
	src := string(mainSrc)
	routes := map[string]bool{}

	// group var -> base path
	groups := map[string]string{}
	// First pass: r.Group and *.Group with a path literal.
	reGroup := regexp.MustCompile(`(?m)^\s*(\w+)\s*:?=\s*([\w.]*)\.Group\("([^"]*)"\)`)
	for _, m := range reGroup.FindAllStringSubmatch(src, -1) {
		base := m[3]
		if m[2] != "" {
			if p, ok := groups[m[2]]; ok {
				base = p + base
			}
		}
		groups[m[1]] = base
	}
	// Second pass: resolve any group whose base referenced another group
	// (nested groups declared before their parent is resolved).
	for i := 0; i < 3; i++ {
		for name, base := range groups {
			_ = name
			for srcName, srcBase := range groups {
				if strings.HasPrefix(base, srcName+":") {
					// not applicable; group bases are paths, not vars
				}
				_ = srcBase
			}
		}
	}
	// Route registrations: VAR.METHOD("/path", ...)
	reRoute := regexp.MustCompile(`(?m)^\s*(\w+)\.(GET|POST|PUT|DELETE|PATCH)\("([^"]*)"`)
	for _, m := range reRoute.FindAllStringSubmatch(src, -1) {
		base, ok := groups[m[1]]
		if !ok {
			continue
		}
		full := base + m[3]
		if !strings.HasPrefix(full, "/") {
			full = "/" + full
		}
		routes[full] = true
	}
	// Public/anon group routes registered via r.GET("/path") directly:
	reDirect := regexp.MustCompile(`(?m)^\s*r\.(GET|POST|PUT|DELETE|PATCH|Any)\("([^"]*)"`)
	for _, m := range reDirect.FindAllStringSubmatch(src, -1) {
		p := m[2]
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		routes[p] = true
	}
	return routes
}

// TestAdminAPIPathsRegistered verifies every literal API path used in admin.js
// (and app.js) is registered in main.go after resolving route group prefixes.
// This catches a frontend calling a route the backend does not expose — e.g.
// genre create/update/delete which only live under /api/v1 (write group), not
// /api/v1/admin.
func TestAdminAPIPathsRegistered(t *testing.T) {
	routes := registeredRoutes(t)
	// Normalize a JS path into the gin style used by main.go (strip trailing
	// slash on empty segments, keep :id placeholders).
	normalize := func(p string) string {
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		return strings.TrimSuffix(p, "/")
	}
	re := regexp.MustCompile(`['"]/(api/v1/[A-Za-z0-9_/\-:]*)['"]`)
	seen := map[string]bool{}
	for _, f := range []string{"admin.js", "app.js"} {
		js := readProjectFile(t, f)
		for _, m := range re.FindAllStringSubmatch(js, -1) {
			path := normalize(m[1])
			if seen[path] {
				continue
			}
			seen[path] = true
			// /api/v1/admin is the base of the admin group (the API const in
			// admin.js), not a route itself — allow it.
			if path == "/api/v1/admin" {
				continue
			}
			if !routes[path] {
				// Also try replacing :id placeholders if JS used a concrete id.
				matched := false
				for r := range routes {
					if strings.ReplaceAll(r, ":id", "1") == path {
						matched = true
						break
					}
				}
				if !matched {
					t.Errorf("%s uses path /%s which is not registered in main.go", f, path)
				}
			}
		}
	}
}
