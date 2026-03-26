// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package api

import (
	_ "embed"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"dnsplane/data"
)

//go:embed embedded/tabler_icons.bundle
var tablerIconsBundle []byte

// dashboardHTMLRendered is dashboardHTML with §Ic:key§ placeholders replaced by inline Tabler SVG.
var dashboardHTMLRendered string

// tablerIconsMap maps logical icon keys to raw (minified) SVG markup from tabler_icons.bundle.
var tablerIconsMap map[string]string

var (
	reSVGWidth24  = regexp.MustCompile(`\s+width="24"`)
	reSVGHeight24 = regexp.MustCompile(`\s+height="24"`)
)

func init() {
	var err error
	tablerIconsMap, err = parseTablerIconsBundle(string(tablerIconsBundle))
	if err != nil {
		panic("api: parse embedded/tabler_icons.bundle: " + err.Error())
	}
	pairs := make([]string, 0, len(tablerIconsMap)*2)
	for k, svg := range tablerIconsMap {
		pairs = append(pairs, "§Ic:"+k+"§", wrapDashboardIconSVG(svg))
	}
	dashboardHTMLRendered = strings.NewReplacer(pairs...).Replace(dashboardHTML)
}

func parseTablerIconsBundle(raw string) (map[string]string, error) {
	m := make(map[string]string)
	lines := strings.Split(raw, "\n")
	for num, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, svg, ok := strings.Cut(line, "\t")
		if !ok || key == "" || strings.TrimSpace(svg) == "" {
			return nil, fmt.Errorf("line %d: expected key<TAB>svg", num+1)
		}
		m[key] = strings.TrimSpace(svg)
	}
	if len(m) == 0 {
		return nil, fmt.Errorf("no icon entries parsed")
	}
	return m, nil
}

// normalizeDashboardIconSVG strips fixed 24px sizing and ensures a dash-icon class for CSS.
func normalizeDashboardIconSVG(svg string) string {
	svg = strings.TrimSpace(svg)
	svg = reSVGWidth24.ReplaceAllString(svg, "")
	svg = reSVGHeight24.ReplaceAllString(svg, "")
	if strings.Contains(svg, `class="icon icon-tabler`) {
		svg = strings.Replace(svg, `class="icon icon-tabler`, `class="dash-icon icon icon-tabler`, 1)
	} else {
		svg = strings.Replace(svg, "<svg ", `<svg class="dash-icon" `, 1)
	}
	return svg
}

func wrapDashboardIconSVG(svg string) string {
	return `<span class="dash-icon-wrap" aria-hidden="true">` + normalizeDashboardIconSVG(svg) + `</span>`
}

// dashboardIconSVGHandler serves one embedded Tabler icon as image/svg+xml (?name=<key>).
func dashboardIconSVGHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !requireStatsHTMLPage(w, r, data.StatsDashboardHTMLEnabled()) {
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	svg, ok := tablerIconsMap[name]
	if !ok || name == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(normalizeDashboardIconSVG(svg)))
}
