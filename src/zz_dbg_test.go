package main

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

func TestDebugExtract(t *testing.T) {
	routes := registeredRoutes(t)
	js := readProjectFile(t, "admin.js")
	re := regexp.MustCompile(`['"]/(api/v1/[A-Za-z0-9_/\-:]*)['"]`)
	for _, m := range re.FindAllStringSubmatch(js, -1) {
		path := strings.TrimSuffix(m[1], "/")
		fmt.Printf("EXTRACTED %-25q routes[%v]=%v\n", m[1], path, routes[path])
	}
}
