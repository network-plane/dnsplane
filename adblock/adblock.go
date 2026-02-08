// Package adblock provides functionality for parsing and managing adblock lists.
package adblock

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	ErrHelpRequested = errors.New("help requested")
	ErrInvalidArgs   = errors.New("invalid arguments")
)

// BlockList maintains a set of blocked domains for fast lookup.
type BlockList struct {
	domains map[string]bool
	mu      sync.RWMutex
}

// NewBlockList creates a new empty block list.
func NewBlockList() *BlockList {
	return &BlockList{
		domains: make(map[string]bool),
	}
}

// IsBlocked checks if a domain is blocked. The domain should be normalized
// (lowercase, trailing dot removed if present).
func (bl *BlockList) IsBlocked(domain string) bool {
	if bl == nil {
		return false
	}
	bl.mu.RLock()
	defer bl.mu.RUnlock()

	normalized := normalizeDomain(domain)

	// Check exact match
	if bl.domains[normalized] {
		return true
	}

	// Check subdomain matches (e.g., if "example.com" is blocked, "ads.example.com" should also be blocked)
	parts := strings.Split(normalized, ".")
	for i := 0; i < len(parts); i++ {
		subdomain := strings.Join(parts[i:], ".")
		if bl.domains[subdomain] {
			return true
		}
	}

	return false
}

// AddDomain adds a domain to the block list.
func (bl *BlockList) AddDomain(domain string) {
	if bl == nil {
		return
	}
	bl.mu.Lock()
	defer bl.mu.Unlock()
	bl.domains[normalizeDomain(domain)] = true
}

// AddDomains adds multiple domains to the block list.
func (bl *BlockList) AddDomains(domains []string) {
	if bl == nil {
		return
	}
	bl.mu.Lock()
	defer bl.mu.Unlock()
	for _, domain := range domains {
		bl.domains[normalizeDomain(domain)] = true
	}
}

// RemoveDomain removes a domain from the block list.
func (bl *BlockList) RemoveDomain(domain string) {
	if bl == nil {
		return
	}
	bl.mu.Lock()
	defer bl.mu.Unlock()
	delete(bl.domains, normalizeDomain(domain))
}

// Clear removes all domains from the block list.
func (bl *BlockList) Clear() {
	if bl == nil {
		return
	}
	bl.mu.Lock()
	defer bl.mu.Unlock()
	bl.domains = make(map[string]bool)
}

// Count returns the number of blocked domains.
func (bl *BlockList) Count() int {
	if bl == nil {
		return 0
	}
	bl.mu.RLock()
	defer bl.mu.RUnlock()
	return len(bl.domains)
}

// GetAll returns all blocked domains as a slice.
func (bl *BlockList) GetAll() []string {
	if bl == nil {
		return nil
	}
	bl.mu.RLock()
	defer bl.mu.RUnlock()
	domains := make([]string, 0, len(bl.domains))
	for domain := range bl.domains {
		domains = append(domains, domain)
	}
	return domains
}

// LoadFromFile parses an adblock list file and adds domains to the block list.
// The file format is: "0.0.0.0 domain1.com domain2.com ..." per line.
func LoadFromFile(blockList *BlockList, filePath string) error {
	if blockList == nil {
		return fmt.Errorf("block list is nil")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	addedCount := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip lines that are entirely comments or empty
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		domains := parseAdblockLine(line)
		if len(domains) > 0 {
			blockList.AddDomains(domains)
			addedCount += len(domains)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	return nil
}

// defaultHTTPTimeout is the timeout for fetching adblock lists from URLs.
const defaultHTTPTimeout = 60 * time.Second

// LoadFromURL fetches an adblock list from a URL and adds domains to the block list.
// The response body is parsed with the same format as LoadFromFile: "0.0.0.0 domain1.com ..." per line.
func LoadFromURL(blockList *BlockList, url string) error {
	if blockList == nil {
		return fmt.Errorf("block list is nil")
	}

	client := &http.Client{Timeout: defaultHTTPTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch URL: status %s", resp.Status)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		domains := parseAdblockLine(line)
		if len(domains) > 0 {
			blockList.AddDomains(domains)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	return nil
}

// parseAdblockLine parses a line from an adblock list.
// Format: "0.0.0.0 domain1.com domain2.com ..." or "127.0.0.1 domain1.com ..."
// Comments starting with # are ignored. If a comment appears in the middle of a line,
// processing stops at that point.
// Returns the list of domains found on the line.
func parseAdblockLine(line string) []string {
	// Remove inline comments (everything after #)
	if commentIdx := strings.Index(line, "#"); commentIdx >= 0 {
		line = line[:commentIdx]
	}

	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	fields := strings.Fields(line)
	if len(fields) < 2 {
		return nil
	}

	// First field should be an IP address (0.0.0.0, 127.0.0.1, etc.)
	firstField := strings.TrimSpace(fields[0])
	if !isBlockIP(firstField) {
		return nil
	}

	// Remaining fields are domains
	domains := make([]string, 0, len(fields)-1)
	for i := 1; i < len(fields); i++ {
		domain := strings.TrimSpace(fields[i])
		// Skip empty fields (shouldn't happen after Fields(), but be safe)
		if domain == "" {
			continue
		}
		// Stop if we hit a comment (shouldn't happen after removing inline comments, but be safe)
		if strings.HasPrefix(domain, "#") {
			break
		}
		domains = append(domains, domain)
	}

	return domains
}

// isBlockIP checks if the given string is a valid blocking IP address.
func isBlockIP(ip string) bool {
	blockIPs := []string{"0.0.0.0", "127.0.0.1", "::", "::1"}
	ip = strings.TrimSpace(ip)
	for _, blockIP := range blockIPs {
		if ip == blockIP {
			return true
		}
	}
	return false
}

// normalizeDomain normalizes a domain name for consistent storage and lookup.
// Removes trailing dots, converts to lowercase, and trims whitespace.
func normalizeDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	domain = strings.ToLower(domain)
	domain = strings.TrimSuffix(domain, ".")
	return domain
}
