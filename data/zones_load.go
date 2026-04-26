// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package data

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"dnsplane/config"
	"dnsplane/dnsrecords"
	"dnsplane/zones"
)

var (
	zoneStmtRE          = regexp.MustCompile(`(?i)\bzone\s+"[^"]*"\s*\{`)
	zoneFileDirectiveRE = regexp.MustCompile(`(?i)file\s+"([^"]+)"`)
)

// loadRecordsBindDir reads zone files from a directory (glob or named.conf) and merges into one slice.
// Duplicate name+type+value: later files win; a warning is logged when a key is overwritten.
func loadRecordsBindDir(rs *config.RecordsSourceConfig) ([]dnsrecords.DNSRecord, error) {
	if rs == nil {
		return nil, fmt.Errorf("records_source is nil")
	}
	dir := strings.TrimSpace(rs.Location)
	if dir == "" {
		return nil, fmt.Errorf("bind_dir: location is empty")
	}
	st, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("bind_dir: stat %q: %w", dir, err)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("bind_dir: %q is not a directory", dir)
	}

	pattern := strings.TrimSpace(rs.IncludePattern)
	if pattern == "" {
		pattern = "*.db"
	}

	var files []string
	if nc := strings.TrimSpace(rs.NamedConf); nc != "" {
		files, err = zoneFilesFromNamedConf(nc, dir)
		if err != nil {
			return nil, err
		}
	} else {
		files, err = filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			return nil, fmt.Errorf("bind_dir: glob: %w", err)
		}
		sort.Strings(files)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("bind_dir: no zone files matched (dir=%q pattern=%q named_conf=%q)", dir, pattern, strings.TrimSpace(rs.NamedConf))
	}

	byKey := make(map[string]dnsrecords.DNSRecord)
	for _, f := range files {
		res, err := zones.ParseFile(f)
		if err != nil {
			return nil, fmt.Errorf("bind_dir: parse %q: %w", f, err)
		}
		for _, w := range res.Warnings {
			resolverSlog().Debug("bind_dir zone parse warning", "file", f, "warning", w)
		}
		for _, rec := range res.Records {
			rec.Name = dnsrecords.CanonicalizeRecordNameForStorage(rec.Name)
			k := bindDirRecordKey(rec)
			if _, dup := byKey[k]; dup {
				resolverSlog().Warn("bind_dir: duplicate record; later file wins", "file", f, "key", k)
			}
			byKey[k] = rec
		}
	}

	out := make([]dnsrecords.DNSRecord, 0, len(byKey))
	for _, rec := range byKey {
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		return a.Value < b.Value
	})
	return out, nil
}

func bindDirRecordKey(r dnsrecords.DNSRecord) string {
	return dnsrecords.NormalizeRecordNameKey(r.Name) + "|" +
		dnsrecords.NormalizeRecordType(r.Type) + "|" +
		dnsrecords.NormalizeRecordValueKey(r.Type, r.Value)
}

// zoneFilesFromNamedConf extracts file "..."; paths from zone { } blocks (BIND subset).
// Relative paths are resolved against the directory containing named.conf, then cleaned.
func zoneFilesFromNamedConf(namedConfPath, zoneDir string) ([]string, error) {
	b, err := os.ReadFile(namedConfPath) // #nosec G304 -- path from operator (named.conf)
	if err != nil {
		return nil, fmt.Errorf("bind_dir named_conf: read %q: %w", namedConfPath, err)
	}
	conf := string(b)
	baseDir := filepath.Dir(namedConfPath)
	_ = zoneDir // reserved for future: alternate relative base

	var paths []string
	idx := 0
	for {
		loc := zoneStmtRE.FindStringIndex(conf[idx:])
		if loc == nil {
			break
		}
		bodyStart := idx + loc[1]
		depth := 1
		j := bodyStart
		for j < len(conf) && depth > 0 {
			switch conf[j] {
			case '{':
				depth++
			case '}':
				depth--
			}
			j++
		}
		if depth != 0 {
			return nil, fmt.Errorf("bind_dir named_conf: unbalanced braces in %q", namedConfPath)
		}
		body := conf[bodyStart : j-1]
		sm := zoneFileDirectiveRE.FindStringSubmatch(body)
		if len(sm) >= 2 {
			p := strings.TrimSpace(sm[1])
			if p == "" {
				idx = j
				continue
			}
			if !filepath.IsAbs(p) {
				p = filepath.Join(baseDir, p)
			}
			paths = append(paths, filepath.Clean(p))
		}
		idx = j
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("bind_dir named_conf: no zone file entries found in %q", namedConfPath)
	}
	sort.Strings(paths)
	uniq := paths[:0]
	var prev string
	for _, p := range paths {
		if p == prev {
			continue
		}
		uniq = append(uniq, p)
		prev = p
	}
	return uniq, nil
}
