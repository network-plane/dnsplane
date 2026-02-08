// Package data - records_source.go loads DNS records from URL or git (read-only sources).
package data

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dnsplane/config"
	"dnsplane/dnsrecords"

	"github.com/go-git/go-git/v5"
)

const (
	recordsHTTPTimeout = 30 * time.Second
	recordsFileName    = "dnsrecords.json"
)

type recordsJSON struct {
	Records []dnsrecords.DNSRecord `json:"records"`
}

// loadRecordsFromURL fetches JSON from url and returns parsed records. Canonicalizes names.
func loadRecordsFromURL(url string) ([]dnsrecords.DNSRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), recordsHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("records url: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("records url fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("records url: status %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("records url read: %w", err)
	}
	var out recordsJSON
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("records url json: %w", err)
	}
	for i := range out.Records {
		out.Records[i].Name = dnsrecords.CanonicalizeRecordNameForStorage(out.Records[i].Name)
	}
	return out.Records, nil
}

// loadRecordsFromGit clones or opens repo at repoURL, reads recordsFileName from worktree, returns records.
// Uses a cache dir under os.TempDir() keyed by a hash/sanitized repo URL so we can pull on refresh.
func loadRecordsFromGit(repoURL string) ([]dnsrecords.DNSRecord, error) {
	cacheDir := filepath.Join(os.TempDir(), "dnsplane-records-git", sanitizeForDir(repoURL))
	_, err := os.Stat(filepath.Join(cacheDir, ".git"))
	if err != nil {
		if os.IsNotExist(err) {
			_ = os.MkdirAll(filepath.Dir(cacheDir), 0o755)
			_, err = git.PlainClone(cacheDir, false, &git.CloneOptions{
				URL:   repoURL,
				Depth: 1,
			})
			if err != nil {
				return nil, fmt.Errorf("records git clone: %w", err)
			}
		} else {
			return nil, fmt.Errorf("records git: %w", err)
		}
	} else {
		repo, err := git.PlainOpen(cacheDir)
		if err != nil {
			return nil, fmt.Errorf("records git open: %w", err)
		}
		w, err := repo.Worktree()
		if err != nil {
			return nil, fmt.Errorf("records git worktree: %w", err)
		}
		err = w.Pull(&git.PullOptions{Depth: 1})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			return nil, fmt.Errorf("records git pull: %w", err)
		}
	}
	path := filepath.Join(cacheDir, recordsFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("records git read file: %w", err)
	}
	var out recordsJSON
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("records git json: %w", err)
	}
	for i := range out.Records {
		out.Records[i].Name = dnsrecords.CanonicalizeRecordNameForStorage(out.Records[i].Name)
	}
	return out.Records, nil
}

func sanitizeForDir(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, ":", "_")
	if s == "" {
		s = "default"
	}
	return s
}

// RecordsSourceIsReadOnly returns true if the current config uses a read-only records source (url or git).
func RecordsSourceIsReadOnly() bool {
	cfg := currentConfig().Config.FileLocations
	if cfg.RecordsSource == nil {
		return false
	}
	t := strings.ToLower(strings.TrimSpace(cfg.RecordsSource.Type))
	return t == config.RecordsSourceURL || t == config.RecordsSourceGit
}
