// Package commandhandler implements the command handling logic for DNSPlane TUI.
package commandhandler

import (
	"bytes"
	"context"
	"dnsplane/cliutil"
	"dnsplane/data"
	"dnsplane/dnsrecordcache"
	"dnsplane/dnsrecords"
	"dnsplane/dnsservers"
	"dnsplane/resolver"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	tui "github.com/network-plane/planetui"
)

// Function variables for server control
var (
	stopDNSServerFunc      func()
	restartDNSServerFunc   func(string)
	getServerStatusFunc    func() bool
	startGinAPIFunc        func(string)
	getServerListenersFunc func() ServerListenerInfo
)

// ServerListenerInfo describes runtime listener configuration for status output.
type ServerListenerInfo struct {
	DNSProtocol         string
	DNSListeners        []string
	APIEndpoint         string
	APIEnabled          bool
	APIRunning          bool
	ClientSocket        string
	ClientSocketEnabled bool
	ClientTCPEndpoint   string
	ClientTCPEnabled    bool
}

// RegisterServerControlHooks wires runtime control functions for server commands.
func RegisterServerControlHooks(stop func(), restart func(string), status func() bool, startAPI func(string), listeners func() ServerListenerInfo) {
	stopDNSServerFunc = stop
	restartDNSServerFunc = restart
	getServerStatusFunc = status
	startGinAPIFunc = startAPI
	getServerListenersFunc = listeners
}

var captureMu sync.Mutex

type factory struct {
	spec tui.CommandSpec
	run  func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult
}

func (f *factory) Spec() tui.CommandSpec { return f.spec }

func (f *factory) New(tui.CommandRuntime) (tui.Command, error) {
	return &command{spec: f.spec, run: f.run}, nil
}

type command struct {
	spec tui.CommandSpec
	run  func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult
}

func (c *command) Spec() tui.CommandSpec { return c.spec }

func (c *command) Execute(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
	return c.run(rt, input)
}

func legacyRunner(legacy func([]string)) func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult {
	return func(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
		if legacy == nil {
			return tui.CommandResult{
				Status: tui.StatusFailed,
				Error:  &tui.CommandError{Message: "command not available", Severity: tui.SeverityError},
			}
		}
		args := append([]string(nil), input.Raw...)
		output, err := captureLegacyOutput(func() { legacy(args) })
		if err != nil {
			return tui.CommandResult{
				Status: tui.StatusFailed,
				Error:  &tui.CommandError{Err: err, Message: "legacy command execution failed", Severity: tui.SeverityError},
			}
		}
		lines := normalizeLines(output)
		messages := make([]tui.OutputMessage, 0, len(lines))
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			messages = append(messages, tui.OutputMessage{Level: tui.SeverityInfo, Content: line})
		}
		return tui.CommandResult{Status: tui.StatusSuccess, Messages: messages}
	}
}

func captureLegacyOutput(fn func()) (string, error) {
	captureMu.Lock()
	defer captureMu.Unlock()
	if fn == nil {
		return "", nil
	}
	stdout := os.Stdout
	stderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w
	os.Stderr = w
	var buf bytes.Buffer
	done := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(&buf, r)
		_ = r.Close()
		done <- copyErr
	}()
	var runErr error
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				runErr = fmt.Errorf("panic: %v", rec)
			}
		}()
		fn()
	}()
	_ = w.Close()
	copyErr := <-done
	os.Stdout = stdout
	os.Stderr = stderr
	if runErr != nil {
		return buf.String(), runErr
	}
	if copyErr != nil {
		return buf.String(), copyErr
	}
	return buf.String(), nil
}

func normalizeLines(out string) []string {
	if out == "" {
		return nil
	}
	out = strings.ReplaceAll(out, "\r\n", "\n")
	out = strings.TrimRight(out, "\n")
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

func convertRecordMessages(msgs []dnsrecords.Message) []tui.OutputMessage {
	converted := make([]tui.OutputMessage, 0, len(msgs))
	for _, msg := range msgs {
		converted = append(converted, tui.OutputMessage{Level: mapRecordLevel(msg.Level), Content: msg.Text})
	}
	return converted
}

func mapRecordLevel(level dnsrecords.Level) tui.SeverityLevel {
	switch level {
	case dnsrecords.LevelWarn:
		return tui.SeverityWarning
	case dnsrecords.LevelError:
		return tui.SeverityError
	default:
		return tui.SeverityInfo
	}
}

func convertServerMessages(msgs []dnsservers.Message) []tui.OutputMessage {
	converted := make([]tui.OutputMessage, 0, len(msgs))
	for _, msg := range msgs {
		converted = append(converted, tui.OutputMessage{Level: mapServerLevel(msg.Level), Content: msg.Text})
	}
	return converted
}

func mapServerLevel(level dnsservers.Level) tui.SeverityLevel {
	switch level {
	case dnsservers.LevelWarn:
		return tui.SeverityWarning
	case dnsservers.LevelError:
		return tui.SeverityError
	default:
		return tui.SeverityInfo
	}
}

func commandErrorFromRecordErr(err error) *tui.CommandError {
	if err == nil {
		return nil
	}
	severity := tui.SeverityError
	if errors.Is(err, dnsrecords.ErrInvalidArgs) {
		severity = tui.SeverityWarning
	}
	return &tui.CommandError{Err: err, Message: err.Error(), Severity: severity}
}

func commandErrorFromCacheErr(err error) *tui.CommandError {
	if err == nil {
		return nil
	}
	severity := tui.SeverityError
	if errors.Is(err, dnsrecordcache.ErrInvalidArgs) {
		severity = tui.SeverityWarning
	}
	return &tui.CommandError{Err: err, Message: err.Error(), Severity: severity}
}

func commandErrorFromServerErr(err error) *tui.CommandError {
	if err == nil {
		return nil
	}
	severity := tui.SeverityError
	if errors.Is(err, dnsservers.ErrInvalidArgs) {
		severity = tui.SeverityWarning
	}
	return &tui.CommandError{Err: err, Message: err.Error(), Severity: severity}
}

func infoMessages(lines ...string) []tui.OutputMessage {
	msgs := make([]tui.OutputMessage, 0, len(lines))
	for _, line := range lines {
		msgs = append(msgs, tui.OutputMessage{Level: tui.SeverityInfo, Content: line})
	}
	return msgs
}

func warnMessages(lines ...string) []tui.OutputMessage {
	msgs := make([]tui.OutputMessage, 0, len(lines))
	for _, line := range lines {
		msgs = append(msgs, tui.OutputMessage{Level: tui.SeverityWarning, Content: line})
	}
	return msgs
}

func newLegacyFactory(spec tui.CommandSpec, run func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult) tui.CommandFactory {
	if spec.Name == "" {
		panic("command spec must include a name")
	}
	wrapped := run
	if wrapped == nil {
		wrapped = func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult {
			return tui.CommandResult{Status: tui.StatusSuccess}
		}
	}
	return &factory{spec: spec, run: wrapped}
}

func registerContexts() {
	contexts := []struct {
		name        string
		description string
		tags        []string
	}{
		{name: "record", description: "- Record Management", tags: []string{"dns", "records"}},
		{name: "cache", description: "- Cache Management", tags: []string{"cache"}},
		{name: "dns", description: "- DNS Server Management", tags: []string{"dns", "servers"}},
		{name: "server", description: "- Server Management", tags: []string{"server"}},
		{name: "tools", description: "- Diagnostic Tools", tags: []string{"tools", "diagnostics"}},
	}
	for _, ctx := range contexts {
		var opts []tui.ContextOption
		if len(ctx.tags) > 0 {
			opts = append(opts, tui.WithContextTags(ctx.tags...))
		}
		tui.RegisterContext(ctx.name, ctx.description, opts...)
	}
}

func runRecordList() func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult {
	return func(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
		dnsData := data.GetInstance()
		records := dnsData.GetRecords()
		listResult, err := dnsrecords.List(records, input.Raw)
		result := tui.CommandResult{Status: tui.StatusSuccess, Messages: convertRecordMessages(listResult.Messages)}
		if errors.Is(err, dnsrecords.ErrHelpRequested) {
			return result
		}
		if err != nil {
			result.Status = tui.StatusFailed
			result.Error = commandErrorFromRecordErr(err)
			return result
		}
		result.Payload = listResult.Records
		rt.Session().Set("record:last_count", len(listResult.Records))
		renderRecordTable(rt.Output(), listResult.Records)
		if listResult.Detailed {
			renderRecordDetails(rt.Output(), listResult.Records)
		}
		return result
	}
}

func runRecordAdd(allowUpdate bool) func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult {
	return func(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
		dnsData := data.GetInstance()
		if err := dnsData.Initialize(); err != nil {
			return tui.CommandResult{
				Status: tui.StatusFailed,
				Error:  &tui.CommandError{Err: err, Message: err.Error(), Severity: tui.SeverityError},
			}
		}
		records := dnsData.GetRecords()
		updated, msgs, err := dnsrecords.Add(input.Raw, records, allowUpdate)
		result := tui.CommandResult{Status: tui.StatusSuccess, Messages: convertRecordMessages(msgs)}
		if errors.Is(err, dnsrecords.ErrHelpRequested) {
			return result
		}
		if err != nil {
			result.Status = tui.StatusFailed
			result.Error = commandErrorFromRecordErr(err)
			return result
		}
		dnsData.UpdateRecords(updated)
		result.Payload = updated
		return result
	}
}

func runRecordRemove() func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult {
	return func(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
		dnsData := data.GetInstance()
		records := dnsData.GetRecords()
		updated, msgs, err := dnsrecords.Remove(input.Raw, records)
		result := tui.CommandResult{Status: tui.StatusSuccess, Messages: convertRecordMessages(msgs)}
		if errors.Is(err, dnsrecords.ErrHelpRequested) {
			return result
		}
		if err != nil {
			result.Status = tui.StatusFailed
			result.Error = commandErrorFromRecordErr(err)
			return result
		}
		dnsData.UpdateRecords(updated)
		result.Payload = updated
		return result
	}
}

func runRecordClear() func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult {
	return func(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
		if cliutil.IsHelpRequest(input.Raw) {
			msgs := infoMessages(
				"Usage: record clear",
				"Description: Remove all DNS records from memory.",
				"Hint: append '?', 'help', or 'h' after the command to view this usage.",
			)
			return tui.CommandResult{Status: tui.StatusSuccess, Messages: msgs}
		}
		if len(input.Raw) > 0 {
			msgs := append(warnMessages("record clear does not accept arguments."), infoMessages("Usage: record clear")...)
			return tui.CommandResult{Status: tui.StatusFailed, Messages: msgs, Error: &tui.CommandError{Message: "unexpected arguments", Severity: tui.SeverityWarning}}
		}
		dnsData := data.GetInstance()
		dnsData.UpdateRecordsInMemory([]dnsrecords.DNSRecord{})
		return tui.CommandResult{Status: tui.StatusSuccess, Messages: infoMessages("All DNS records have been cleared.")}
	}
}

func runRecordLoad() func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult {
	return func(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
		if cliutil.IsHelpRequest(input.Raw) {
			msgs := infoMessages(
				"Usage: record load",
				"Description: Load DNS records from the default storage file.",
				"Hint: append '?', 'help', or 'h' after the command to view this usage.",
			)
			return tui.CommandResult{Status: tui.StatusSuccess, Messages: msgs}
		}
		if len(input.Raw) > 0 {
			msgs := append(warnMessages("record load does not accept arguments."), infoMessages("Usage: record load")...)
			return tui.CommandResult{Status: tui.StatusFailed, Messages: msgs, Error: &tui.CommandError{Message: "unexpected arguments", Severity: tui.SeverityWarning}}
		}
		dnsData := data.GetInstance()
		records, err := data.LoadDNSRecords()
		if err != nil {
			return tui.CommandResult{
				Status: tui.StatusFailed,
				Error:  &tui.CommandError{Err: err, Message: err.Error(), Severity: tui.SeverityError},
			}
		}
		dnsData.UpdateRecords(records)
		return tui.CommandResult{Status: tui.StatusSuccess, Payload: records, Messages: infoMessages("DNS records loaded.")}
	}
}

func runRecordSave() func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult {
	return func(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
		if cliutil.IsHelpRequest(input.Raw) {
			msgs := infoMessages(
				"Usage: record save",
				"Description: Save current DNS records to the default storage file.",
				"Hint: append '?', 'help', or 'h' after the command to view this usage.",
			)
			return tui.CommandResult{Status: tui.StatusSuccess, Messages: msgs}
		}
		if len(input.Raw) > 0 {
			msgs := append(warnMessages("record save does not accept arguments."), infoMessages("Usage: record save")...)
			return tui.CommandResult{Status: tui.StatusFailed, Messages: msgs, Error: &tui.CommandError{Message: "unexpected arguments", Severity: tui.SeverityWarning}}
		}
		dnsData := data.GetInstance()
		records := dnsData.GetRecords()
		if err := data.SaveDNSRecords(records); err != nil {
			return tui.CommandResult{Status: tui.StatusFailed, Error: &tui.CommandError{Err: err, Message: err.Error(), Severity: tui.SeverityError}}
		}
		return tui.CommandResult{Status: tui.StatusSuccess, Messages: infoMessages("DNS records saved.")}
	}
}

func runCacheList() func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult {
	return func(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
		if cliutil.IsHelpRequest(input.Raw) {
			msgs := infoMessages(
				"Usage: cache list",
				"Description: List all cache entries in memory.",
				"Hint: append '?', 'help', or 'h' after the command to view this usage.",
			)
			return tui.CommandResult{Status: tui.StatusSuccess, Messages: msgs}
		}
		dnsData := data.GetInstance()
		cache := dnsrecordcache.List(dnsData.GetCacheRecords())
		result := tui.CommandResult{Status: tui.StatusSuccess, Payload: cache}
		rt.Session().Set("cache:last_count", len(cache))
		renderCacheTable(rt.Output(), cache)
		if len(cache) == 0 {
			result.Messages = infoMessages("No cache records found.")
		}
		return result
	}
}

func runCacheRemove() func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult {
	return func(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
		dnsData := data.GetInstance()
		cacheRecords := dnsData.GetCacheRecords()
		updated, msgs, err := dnsrecordcache.Remove(input.Raw, cacheRecords)
		result := tui.CommandResult{Status: tui.StatusSuccess, Messages: convertRecordMessages(msgs)}
		if errors.Is(err, dnsrecordcache.ErrHelpRequested) {
			return result
		}
		if err != nil {
			result.Status = tui.StatusFailed
			result.Error = commandErrorFromCacheErr(err)
			return result
		}
		dnsData.UpdateCacheRecordsInMemory(updated)
		result.Payload = updated
		return result
	}
}

func runCacheClear() func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult {
	return func(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
		if cliutil.IsHelpRequest(input.Raw) {
			msgs := infoMessages(
				"Usage: cache clear",
				"Description: Remove every cached DNS entry.",
				"Hint: append '?', 'help', or 'h' after the command to view this usage.",
			)
			return tui.CommandResult{Status: tui.StatusSuccess, Messages: msgs}
		}
		if len(input.Raw) > 0 {
			msgs := append(warnMessages("cache clear does not accept arguments."), infoMessages("Usage: cache clear")...)
			return tui.CommandResult{Status: tui.StatusFailed, Messages: msgs, Error: &tui.CommandError{Message: "unexpected arguments", Severity: tui.SeverityWarning}}
		}
		dnsData := data.GetInstance()
		dnsData.UpdateCacheRecordsInMemory([]dnsrecordcache.CacheRecord{})
		return tui.CommandResult{Status: tui.StatusSuccess, Messages: infoMessages("Cache cleared.")}
	}
}

func runCacheLoad() func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult {
	return func(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
		if cliutil.IsHelpRequest(input.Raw) {
			msgs := infoMessages(
				"Usage: cache load",
				"Description: Load cache records from the default storage file.",
				"Hint: append '?', 'help', or 'h' after the command to view this usage.",
			)
			return tui.CommandResult{Status: tui.StatusSuccess, Messages: msgs}
		}
		if len(input.Raw) > 0 {
			msgs := append(warnMessages("cache load does not accept arguments."), infoMessages("Usage: cache load")...)
			return tui.CommandResult{Status: tui.StatusFailed, Messages: msgs, Error: &tui.CommandError{Message: "unexpected arguments", Severity: tui.SeverityWarning}}
		}
		dnsData := data.GetInstance()
		cache, err := data.LoadCacheRecords()
		if err != nil {
			return tui.CommandResult{
				Status: tui.StatusFailed,
				Error:  &tui.CommandError{Err: err, Message: err.Error(), Severity: tui.SeverityError},
			}
		}
		dnsData.UpdateCacheRecordsInMemory(cache)
		return tui.CommandResult{Status: tui.StatusSuccess, Payload: cache, Messages: infoMessages("Cache records loaded.")}
	}
}

func runCacheSave() func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult {
	return func(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
		if cliutil.IsHelpRequest(input.Raw) {
			msgs := infoMessages(
				"Usage: cache save",
				"Description: Save cache records to the default storage file.",
				"Hint: append '?', 'help', or 'h' after the command to view this usage.",
			)
			return tui.CommandResult{Status: tui.StatusSuccess, Messages: msgs}
		}
		if len(input.Raw) > 0 {
			msgs := append(warnMessages("cache save does not accept arguments."), infoMessages("Usage: cache save")...)
			return tui.CommandResult{Status: tui.StatusFailed, Messages: msgs, Error: &tui.CommandError{Message: "unexpected arguments", Severity: tui.SeverityWarning}}
		}
		dnsData := data.GetInstance()
		cache := dnsData.GetCacheRecords()
		if err := data.SaveCacheRecords(cache); err != nil {
			return tui.CommandResult{Status: tui.StatusFailed, Error: &tui.CommandError{Err: err, Message: err.Error(), Severity: tui.SeverityError}}
		}
		return tui.CommandResult{Status: tui.StatusSuccess, Messages: infoMessages("Cache records saved.")}
	}
}

func runDNSList() func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult {
	return func(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
		if cliutil.IsHelpRequest(input.Raw) {
			msgs := infoMessages(
				"Usage: dns list",
				"Description: Show all configured upstream DNS servers.",
				"Hint: append '?', 'help', or 'h' after the command to view this usage.",
			)
			return tui.CommandResult{Status: tui.StatusSuccess, Messages: msgs}
		}
		dnsData := data.GetInstance()
		servers := dnsData.GetServers()
		listResult := dnsservers.List(servers)
		result := tui.CommandResult{Status: tui.StatusSuccess, Payload: listResult.Servers, Messages: convertServerMessages(listResult.Messages)}
		rt.Session().Set("dns:last_count", len(listResult.Servers))
		renderDNSServerTable(rt.Output(), listResult.Servers)
		return result
	}
}

func runDNSAdd() func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult {
	return func(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
		dnsData := data.GetInstance()
		servers := dnsData.GetServers()
		updated, msgs, err := dnsservers.Add(input.Raw, servers)
		result := tui.CommandResult{Status: tui.StatusSuccess, Messages: convertServerMessages(msgs)}
		if errors.Is(err, dnsservers.ErrHelpRequested) {
			return result
		}
		if err != nil {
			result.Status = tui.StatusFailed
			result.Error = commandErrorFromServerErr(err)
			return result
		}
		dnsData.UpdateServers(updated)
		result.Payload = updated
		return result
	}
}

func runDNSRemove() func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult {
	return func(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
		dnsData := data.GetInstance()
		servers := dnsData.GetServers()
		updated, msgs, err := dnsservers.Remove(input.Raw, servers)
		result := tui.CommandResult{Status: tui.StatusSuccess, Messages: convertServerMessages(msgs)}
		if errors.Is(err, dnsservers.ErrHelpRequested) {
			return result
		}
		if err != nil {
			result.Status = tui.StatusFailed
			result.Error = commandErrorFromServerErr(err)
			return result
		}
		dnsData.UpdateServers(updated)
		result.Payload = updated
		return result
	}
}

func runDNSUpdate() func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult {
	return func(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
		dnsData := data.GetInstance()
		servers := dnsData.GetServers()
		updated, msgs, err := dnsservers.Update(input.Raw, servers)
		result := tui.CommandResult{Status: tui.StatusSuccess, Messages: convertServerMessages(msgs)}
		if errors.Is(err, dnsservers.ErrHelpRequested) {
			return result
		}
		if err != nil {
			result.Status = tui.StatusFailed
			result.Error = commandErrorFromServerErr(err)
			return result
		}
		dnsData.UpdateServers(updated)
		result.Payload = updated
		return result
	}
}

func runDNSClear() func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult {
	return func(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
		if cliutil.IsHelpRequest(input.Raw) {
			msgs := infoMessages(
				"Usage: dns clear",
				"Description: Remove all configured upstream DNS servers.",
				"Hint: append '?', 'help', or 'h' after the command to view this usage.",
			)
			return tui.CommandResult{Status: tui.StatusSuccess, Messages: msgs}
		}
		if len(input.Raw) > 0 {
			msgs := append(warnMessages("dns clear does not accept arguments."), infoMessages("Usage: dns clear")...)
			return tui.CommandResult{Status: tui.StatusFailed, Messages: msgs, Error: &tui.CommandError{Message: "unexpected arguments", Severity: tui.SeverityWarning}}
		}
		dnsData := data.GetInstance()
		dnsData.UpdateServers([]dnsservers.DNSServer{})
		return tui.CommandResult{Status: tui.StatusSuccess, Messages: infoMessages("All DNS servers have been cleared.")}
	}
}

func runDNSLoad() func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult {
	return func(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
		if cliutil.IsHelpRequest(input.Raw) {
			msgs := infoMessages(
				"Usage: dns load",
				"Description: Load DNS servers from the default storage file.",
				"Hint: append '?', 'help', or 'h' after the command to view this usage.",
			)
			return tui.CommandResult{Status: tui.StatusSuccess, Messages: msgs}
		}
		if len(input.Raw) > 0 {
			msgs := append(warnMessages("dns load does not accept arguments."), infoMessages("Usage: dns load")...)
			return tui.CommandResult{Status: tui.StatusFailed, Messages: msgs, Error: &tui.CommandError{Message: "unexpected arguments", Severity: tui.SeverityWarning}}
		}
		dnsData := data.GetInstance()
		servers, err := data.LoadDNSServers()
		if err != nil {
			return tui.CommandResult{
				Status: tui.StatusFailed,
				Error:  &tui.CommandError{Err: err, Message: err.Error(), Severity: tui.SeverityError},
			}
		}
		dnsData.UpdateServers(servers)
		return tui.CommandResult{Status: tui.StatusSuccess, Payload: servers, Messages: infoMessages("DNS servers loaded.")}
	}
}

func runDNSSave() func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult {
	return func(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
		if cliutil.IsHelpRequest(input.Raw) {
			msgs := infoMessages(
				"Usage: dns save",
				"Description: Save DNS server definitions to the default storage file.",
				"Hint: append '?', 'help', or 'h' after the command to view this usage.",
			)
			return tui.CommandResult{Status: tui.StatusSuccess, Messages: msgs}
		}
		if len(input.Raw) > 0 {
			msgs := append(warnMessages("dns save does not accept arguments."), infoMessages("Usage: dns save")...)
			return tui.CommandResult{Status: tui.StatusFailed, Messages: msgs, Error: &tui.CommandError{Message: "unexpected arguments", Severity: tui.SeverityWarning}}
		}
		dnsData := data.GetInstance()
		servers := dnsData.GetServers()
		if err := data.SaveDNSServers(servers); err != nil {
			return tui.CommandResult{Status: tui.StatusFailed, Error: &tui.CommandError{Err: err, Message: err.Error(), Severity: tui.SeverityError}}
		}
		return tui.CommandResult{Status: tui.StatusSuccess, Messages: infoMessages("DNS servers saved.")}
	}
}

func renderRecordTable(out tui.OutputChannel, records []dnsrecords.DNSRecord) {
	if len(records) == 0 {
		return
	}
	rows := make([][]string, 0, len(records))
	for _, record := range records {
		rows = append(rows, []string{record.Name, record.Type, record.Value, fmt.Sprintf("%d", record.TTL)})
	}
	out.WriteTable([]string{"Name", "Type", "Value", "TTL"}, rows)
	tui.EnsureLineBreak(out)
}

func renderRecordDetails(out tui.OutputChannel, records []dnsrecords.DNSRecord) {
	for _, record := range records {
		var details []string
		if !record.AddedOn.IsZero() {
			details = append(details, fmt.Sprintf("Added On: %s", record.AddedOn.Format(time.RFC3339)))
		}
		if !record.UpdatedOn.IsZero() {
			details = append(details, fmt.Sprintf("Updated On: %s", record.UpdatedOn.Format(time.RFC3339)))
		}
		if !record.LastQuery.IsZero() {
			details = append(details, fmt.Sprintf("Last Query: %s", record.LastQuery.Format(time.RFC3339)))
		}
		if record.MACAddress != "" {
			details = append(details, fmt.Sprintf("MAC Address: %s", record.MACAddress))
		}
		if record.CacheRecord {
			details = append(details, "Cache Record: true")
		}
		if len(details) == 0 {
			continue
		}
		out.Info(fmt.Sprintf("%s %s %s %d", record.Name, record.Type, record.Value, record.TTL))
		for _, line := range details {
			out.Info("  " + line)
		}
		out.Info("")
	}
}

func renderCacheTable(out tui.OutputChannel, cache []dnsrecordcache.CacheRecord) {
	if len(cache) == 0 {
		return
	}
	rows := make([][]string, 0, len(cache))
	for _, record := range cache {
		expires := ""
		if !record.Expiry.IsZero() {
			expires = record.Expiry.Format(time.RFC3339)
		}
		rows = append(rows, []string{record.DNSRecord.Name, record.DNSRecord.Type, record.DNSRecord.Value, fmt.Sprintf("%d", record.DNSRecord.TTL), expires})
	}
	out.WriteTable([]string{"Name", "Type", "Value", "TTL", "Expires"}, rows)
	tui.EnsureLineBreak(out)
}

func renderDNSServerTable(out tui.OutputChannel, servers []dnsservers.DNSServer) {
	if len(servers) == 0 {
		return
	}
	rows := make([][]string, 0, len(servers))
	for _, server := range servers {
		rows = append(rows, []string{
			server.Address,
			server.Port,
			fmt.Sprintf("%t", server.Active),
			fmt.Sprintf("%t", server.LocalResolver),
			fmt.Sprintf("%t", server.AdBlocker),
		})
	}
	out.WriteTable([]string{"Address", "Port", "Active", "Local", "AdBlocker"}, rows)
	tui.EnsureLineBreak(out)
}

func runToolsDig() func(tui.CommandRuntime, tui.CommandInput) tui.CommandResult {
	return func(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
		if cliutil.IsHelpRequest(input.Raw) {
			msgs := infoMessages(
				"Usage: tools dig [type] <domain|ip> [type] [@server] [port]",
				"Description: Query all configured DNS servers (or a specific server) for the provided name/IP.",
				"Examples:",
				"  tools dig example.com",
				"  tools dig example.com AAAA",
				"  tools dig AAAA example.com",
				"  tools dig 8.8.8.8",
				"  tools dig PTR 8.8.8.8",
				"  tools dig example.com @8.8.8.8",
				"  tools dig A example.com @8.8.8.8 5353",
				"Hint: append '?', 'help', or 'h' after the command to view this usage.",
			)
			return tui.CommandResult{Status: tui.StatusSuccess, Messages: msgs}
		}

		if len(input.Raw) == 0 {
			msgs := append(warnMessages("tools dig requires a domain or IP address."), infoMessages("Usage: tools dig [type] <domain|ip> [type] [@server] [port]")...)
			return tui.CommandResult{Status: tui.StatusFailed, Messages: msgs, Error: &tui.CommandError{Message: "missing argument", Severity: tui.SeverityWarning}}
		}

		typeToken, queryTarget, serverHost, serverPort, parseErr := parseDigArguments(input.Raw)
		if parseErr != nil {
			return tui.CommandResult{Status: tui.StatusFailed, Messages: warnMessages(parseErr.Error()), Error: &tui.CommandError{Message: parseErr.Error(), Severity: tui.SeverityWarning}}
		}

		if queryTarget == "" {
			msgs := append(warnMessages("tools dig requires a domain or IP address."), infoMessages("Usage: tools dig [type] <domain|ip> [type] [@server] [port]")...)
			return tui.CommandResult{Status: tui.StatusFailed, Messages: msgs, Error: &tui.CommandError{Message: "missing argument", Severity: tui.SeverityWarning}}
		}

		queryType, typeErr := resolveRecordType(typeToken, queryTarget)
		if typeErr != nil {
			return tui.CommandResult{Status: tui.StatusFailed, Messages: warnMessages(typeErr.Error()), Error: &tui.CommandError{Message: typeErr.Error(), Severity: tui.SeverityWarning}}
		}

		queryName, nameErr := normalizeQueryName(queryTarget, queryType)
		if nameErr != nil {
			return tui.CommandResult{Status: tui.StatusFailed, Messages: warnMessages(nameErr.Error()), Error: &tui.CommandError{Message: nameErr.Error(), Severity: tui.SeverityWarning}}
		}

		// Determine DNS servers to query
		var servers []string
		if serverHost != "" {
			if serverPort == "" {
				serverPort = "53"
			}
			servers = []string{net.JoinHostPort(serverHost, serverPort)}
		} else {
			dnsData := data.GetInstance()
			settings := dnsData.GetResolverSettings()
			servers = dnsservers.GetDNSArray(dnsData.GetServers(), true)
			if len(servers) == 0 {
				fallbackIP := strings.TrimSpace(settings.FallbackServerIP)
				fallbackPort := strings.TrimSpace(settings.FallbackServerPort)
				if fallbackIP != "" {
					if fallbackPort == "" {
						fallbackPort = "53"
					}
					servers = append(servers, fmt.Sprintf("%s:%s", fallbackIP, fallbackPort))
				}
			}
			if len(servers) == 0 {
				msgs := infoMessages("No DNS servers configured. Add servers using 'dns add' or set a fallback server.")
				return tui.CommandResult{Status: tui.StatusFailed, Messages: msgs, Error: &tui.CommandError{Message: "no DNS servers configured", Severity: tui.SeverityWarning}}
			}
		}

		// Create DNS client
		client := resolver.NewDNSClient(5 * time.Second)
		question := dns.Question{
			Name:   queryName,
			Qtype:  queryType,
			Qclass: dns.ClassINET,
		}

		// Query all servers in parallel
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		type serverResult struct {
			server   string
			response *dns.Msg
			err      error
			duration time.Duration
		}

		results := make(chan serverResult, len(servers))
		var wg sync.WaitGroup

		for _, server := range servers {
			wg.Add(1)
			go func(srv string) {
				defer wg.Done()
				start := time.Now()
				resp, err := client.Query(ctx, question, srv)
				duration := time.Since(start)
				results <- serverResult{
					server:   srv,
					response: resp,
					err:      err,
					duration: duration,
				}
			}(server)
		}

		go func() {
			wg.Wait()
			close(results)
		}()

		// Collect results
		var allResults []serverResult
		for res := range results {
			allResults = append(allResults, res)
		}

		// Display results
		out := rt.Output()
		out.Info(fmt.Sprintf("Querying %d DNS server(s) for %s %s...\n", len(servers), queryName, dns.TypeToString[queryType]))
		out.Info("")

		rows := make([][]string, 0, len(allResults))
		for _, res := range allResults {
			status := "OK"
			answer := ""
			if res.err != nil {
				status = "ERROR"
				answer = res.err.Error()
			} else if res.response == nil {
				status = "NO RESPONSE"
				answer = "No response received"
			} else if len(res.response.Answer) == 0 {
				status = "NO ANSWER"
				answer = "No answer section"
			} else {
				var answers []string
				for _, rr := range res.response.Answer {
					answers = append(answers, rr.String())
				}
				answer = strings.Join(answers, "; ")
				if res.response.MsgHdr.Authoritative {
					status = "AUTH"
				}
			}

			rows = append(rows, []string{
				res.server,
				status,
				fmt.Sprintf("%.2fms", float64(res.duration.Nanoseconds())/1e6),
				answer,
			})
		}

		if len(rows) > 0 {
			writeSimpleTable(out, []string{"Server", "Status", "Time", "Answer"}, rows)
			tui.EnsureLineBreak(out)
		}

		return tui.CommandResult{Status: tui.StatusSuccess, Payload: allResults}
	}
}

func parseDigArguments(args []string) (typeToken, target, serverHost, serverPort string, err error) {
	if len(args) == 0 {
		err = fmt.Errorf("missing arguments")
		return
	}
	idx := 0
	if t, ok := lookupRecordTypeToken(args[idx]); ok {
		typeToken = strings.ToUpper(strings.TrimSpace(args[idx]))
		if t == 0 {
			err = fmt.Errorf("invalid record type: %s", args[idx])
			return
		}
		idx++
	}
	if idx >= len(args) {
		return
	}
	target = strings.TrimSpace(args[idx])
	idx++
	if typeToken == "" && idx < len(args) {
		if _, ok := lookupRecordTypeToken(args[idx]); ok {
			typeToken = strings.ToUpper(strings.TrimSpace(args[idx]))
			idx++
		}
	}

	for idx < len(args) {
		token := strings.TrimSpace(args[idx])
		switch {
		case token == "":
			// skip
		case token == "@":
			idx++
			if idx < len(args) {
				token = strings.TrimSpace(args[idx])
				token = strings.TrimPrefix(token, "@")
				if token != "" && serverHost == "" {
					if host, port, ok := strings.Cut(token, ":"); ok {
						serverHost = host
						serverPort = port
					} else {
						serverHost = token
					}
				}
			}
		case strings.HasPrefix(token, "@"):
			token = strings.TrimPrefix(token, "@")
			if token != "" {
				if host, port, ok := strings.Cut(token, ":"); ok {
					if serverHost == "" {
						serverHost = host
					}
					if serverPort == "" {
						serverPort = port
					}
				} else if serverHost == "" {
					serverHost = token
				}
			}
		case serverHost != "" && serverPort == "" && isAllDigits(token):
			serverPort = token
		default:
			// ignore additional tokens
		}
		idx++
	}

	return
}

func lookupRecordTypeToken(token string) (uint16, bool) {
	upper := strings.ToUpper(strings.TrimSpace(token))
	if upper == "" {
		return 0, false
	}
	t, ok := dns.StringToType[upper]
	return t, ok
}

func resolveRecordType(typeToken, target string) (uint16, error) {
	if typeToken != "" {
		upper := strings.ToUpper(strings.TrimSpace(typeToken))
		if t, ok := dns.StringToType[upper]; ok {
			return t, nil
		}
		return 0, fmt.Errorf("invalid record type: %s", typeToken)
	}
	clean := strings.TrimSuffix(strings.TrimSpace(target), ".")
	if ip := net.ParseIP(clean); ip != nil {
		return dns.TypePTR, nil
	}
	return dns.TypeA, nil
}

func normalizeQueryName(target string, queryType uint16) (string, error) {
	name := strings.TrimSpace(target)
	if name == "" {
		return "", fmt.Errorf("empty query target")
	}

	clean := strings.TrimSuffix(name, ".")
	if queryType == dns.TypePTR {
		if net.ParseIP(clean) != nil {
			reverse, err := dns.ReverseAddr(clean)
			if err != nil {
				return "", fmt.Errorf("unable to construct PTR query for %s: %w", target, err)
			}
			name = reverse
		}
	}

	if !strings.HasSuffix(name, ".") {
		name += "."
	}
	return name, nil
}

func isAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func writeSimpleTable(out tui.OutputChannel, headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}
	cols := len(headers)
	widths := make([]int, cols)
	for i, header := range headers {
		widths[i] = len(header)
	}
	for _, row := range rows {
		for i := 0; i < cols && i < len(row); i++ {
			if len(row[i]) > widths[i] {
				widths[i] = len(row[i])
			}
		}
	}
	formatParts := make([]string, cols)
	separatorParts := make([]string, cols)
	for i, width := range widths {
		formatParts[i] = fmt.Sprintf("%%-%ds", width)
		sepWidth := width
		if sepWidth > 40 {
			sepWidth = sepWidth / 2
			if sepWidth < len(headers[i]) {
				sepWidth = len(headers[i])
			}
		}
		separatorParts[i] = strings.Repeat("-", sepWidth)
	}
	format := strings.Join(formatParts, "  ")
	separator := strings.Join(separatorParts, "  ")

	out.Info(fmt.Sprintf(format, toInterface(headers, cols)...))
	out.Info(separator)
	for _, row := range rows {
		out.Info(fmt.Sprintf(format, toInterface(row, cols)...))
	}
}

func toInterface(values []string, count int) []interface{} {
	result := make([]interface{}, count)
	for i := 0; i < count; i++ {
		if i < len(values) {
			result[i] = values[i]
		} else {
			result[i] = ""
		}
	}
	return result
}

// RegisterCommands registers all DNS related contexts and commands with the TUI package.
func RegisterCommands() {
	registerContexts()

	commands := []tui.CommandFactory{
		newLegacyFactory(tui.CommandSpec{
			Name:        "stats",
			Summary:     "Display resolver statistics",
			Description: "Shows runtime counters, record totals, and cache statistics for the running resolver.",
			Usage:       "stats",
			Category:    "Monitoring",
			Tags:        []string{"monitoring", "status"},
			Examples: []tui.Example{
				{Description: "Show resolver metrics", Command: "stats"},
			},
		}, legacyRunner(handleStats)),

		newLegacyFactory(tui.CommandSpec{
			Context:     "record",
			Name:        "add",
			Summary:     "Add a DNS record",
			Description: "Adds a DNS record to the in-memory store. Accepts <name> [type] <value> [ttl] syntax.",
			Usage:       "record add <name> [type] <value> [ttl]",
			Category:    "DNS Records",
			Tags:        []string{"records", "create"},
			Args: []tui.ArgSpec{
				{Name: "params", Description: "Name [Type] Value [TTL]", Repeatable: true},
			},
			Examples: []tui.Example{
				{Description: "Add an A record", Command: "record add example.com A 127.0.0.1 3600"},
				{Description: "Add record inferring type", Command: "record add example.com 127.0.0.1"},
			},
		}, runRecordAdd(false)),
		newLegacyFactory(tui.CommandSpec{
			Context:     "record",
			Name:        "remove",
			Summary:     "Remove a DNS record",
			Description: "Deletes a DNS record matching the provided name, type, and value.",
			Usage:       "record remove <name> [type] <value>",
			Category:    "DNS Records",
			Tags:        []string{"records", "delete"},
			Args: []tui.ArgSpec{
				{Name: "params", Description: "Name [Type] Value", Repeatable: true},
			},
			Examples: []tui.Example{{Description: "Remove an A record", Command: "record remove example.com A 127.0.0.1"}},
		}, runRecordRemove()),
		newLegacyFactory(tui.CommandSpec{
			Context:     "record",
			Name:        "update",
			Summary:     "Update an existing record",
			Description: "Adds or updates a DNS record depending on whether it already exists.",
			Usage:       "record update <name> [type] <value> [ttl]",
			Category:    "DNS Records",
			Tags:        []string{"records", "update"},
			Args: []tui.ArgSpec{
				{Name: "params", Description: "Name [Type] Value [TTL]", Repeatable: true},
			},
			Examples: []tui.Example{{Description: "Update TTL for record", Command: "record update example.com A 127.0.0.1 120"}},
		}, runRecordAdd(true)),
		newLegacyFactory(tui.CommandSpec{
			Context:     "record",
			Name:        "list",
			Summary:     "List DNS records",
			Description: "Displays configured DNS records with optional detail mode and filtering.",
			Usage:       "record list [details|d] [filter]",
			Category:    "DNS Records",
			Tags:        []string{"records", "list"},
			Args: []tui.ArgSpec{
				{Name: "mode", Description: "Use 'details' or 'd' for verbose output"},
				{Name: "filter", Description: "Optional filter by name or type", Required: false},
			},
			Examples: []tui.Example{
				{Description: "List records", Command: "record list"},
				{Description: "Show detailed records", Command: "record list details"},
			},
		}, runRecordList()),
		newLegacyFactory(tui.CommandSpec{
			Context:     "record",
			Name:        "clear",
			Summary:     "Clear all DNS records",
			Description: "Removes every DNS record from the in-memory store.",
			Usage:       "record clear",
			Category:    "DNS Records",
			Tags:        []string{"records", "clear"},
			Examples:    []tui.Example{{Description: "Clear records", Command: "record clear"}},
		}, runRecordClear()),
		newLegacyFactory(tui.CommandSpec{
			Context:     "record",
			Name:        "load",
			Summary:     "Load DNS records from disk",
			Description: "Reloads DNS records from the default storage file.",
			Usage:       "record load",
			Category:    "DNS Records",
			Tags:        []string{"records", "load"},
			Examples:    []tui.Example{{Description: "Load records", Command: "record load"}},
		}, runRecordLoad()),
		newLegacyFactory(tui.CommandSpec{
			Context:     "record",
			Name:        "save",
			Summary:     "Save DNS records to disk",
			Description: "Persists current DNS records to the default storage file.",
			Usage:       "record save",
			Category:    "DNS Records",
			Tags:        []string{"records", "save"},
			Examples:    []tui.Example{{Description: "Save records", Command: "record save"}},
		}, runRecordSave()),

		newLegacyFactory(tui.CommandSpec{
			Context:     "cache",
			Name:        "list",
			Summary:     "List cache entries",
			Description: "Displays cached DNS entries currently held in memory.",
			Usage:       "cache list",
			Category:    "Cache",
			Tags:        []string{"cache", "list"},
			Examples:    []tui.Example{{Description: "List cache", Command: "cache list"}},
		}, runCacheList()),
		newLegacyFactory(tui.CommandSpec{
			Context:     "cache",
			Name:        "remove",
			Summary:     "Remove cache entry",
			Description: "Removes a cached DNS record matching the provided criteria.",
			Usage:       "cache remove <name> [type] <value>",
			Category:    "Cache",
			Tags:        []string{"cache", "delete"},
			Args:        []tui.ArgSpec{{Name: "params", Description: "Name [Type] Value", Repeatable: true}},
		}, runCacheRemove()),
		newLegacyFactory(tui.CommandSpec{
			Context:     "cache",
			Name:        "clear",
			Summary:     "Clear cache",
			Description: "Empties all cached DNS entries.",
			Usage:       "cache clear",
			Category:    "Cache",
			Tags:        []string{"cache", "clear"},
		}, runCacheClear()),
		newLegacyFactory(tui.CommandSpec{
			Context:     "cache",
			Name:        "load",
			Summary:     "Load cache from disk",
			Description: "Loads cache entries from the default storage file.",
			Usage:       "cache load",
			Category:    "Cache",
			Tags:        []string{"cache", "load"},
		}, runCacheLoad()),
		newLegacyFactory(tui.CommandSpec{
			Context:     "cache",
			Name:        "save",
			Summary:     "Save cache to disk",
			Description: "Persists cache entries to the default storage file.",
			Usage:       "cache save",
			Category:    "Cache",
			Tags:        []string{"cache", "save"},
		}, runCacheSave()),

		newLegacyFactory(tui.CommandSpec{
			Context:     "dns",
			Name:        "add",
			Summary:     "Add upstream DNS server",
			Description: "Adds an upstream DNS server definition.",
			Usage:       "dns add <address> [port]",
			Category:    "Upstream Servers",
			Tags:        []string{"dns", "servers", "add"},
		}, runDNSAdd()),
		newLegacyFactory(tui.CommandSpec{
			Context:     "dns",
			Name:        "remove",
			Summary:     "Remove upstream DNS server",
			Description: "Removes an upstream DNS server definition.",
			Usage:       "dns remove <address> [port]",
			Category:    "Upstream Servers",
			Tags:        []string{"dns", "servers", "remove"},
		}, runDNSRemove()),
		newLegacyFactory(tui.CommandSpec{
			Context:     "dns",
			Name:        "update",
			Summary:     "Update upstream DNS server",
			Description: "Updates an existing upstream DNS server definition.",
			Usage:       "dns update <address> [port]",
			Category:    "Upstream Servers",
			Tags:        []string{"dns", "servers", "update"},
		}, runDNSUpdate()),
		newLegacyFactory(tui.CommandSpec{
			Context:     "dns",
			Name:        "list",
			Summary:     "List upstream DNS servers",
			Description: "Displays configured upstream DNS servers.",
			Usage:       "dns list",
			Category:    "Upstream Servers",
			Tags:        []string{"dns", "servers", "list"},
		}, runDNSList()),
		newLegacyFactory(tui.CommandSpec{
			Context:     "dns",
			Name:        "clear",
			Summary:     "Clear upstream DNS servers",
			Description: "Removes every upstream DNS server definition.",
			Usage:       "dns clear",
			Category:    "Upstream Servers",
			Tags:        []string{"dns", "servers", "clear"},
		}, runDNSClear()),
		newLegacyFactory(tui.CommandSpec{
			Context:     "dns",
			Name:        "load",
			Summary:     "Load upstream DNS servers",
			Description: "Loads upstream DNS server definitions from disk.",
			Usage:       "dns load",
			Category:    "Upstream Servers",
			Tags:        []string{"dns", "servers", "load"},
		}, runDNSLoad()),
		newLegacyFactory(tui.CommandSpec{
			Context:     "dns",
			Name:        "save",
			Summary:     "Save upstream DNS servers",
			Description: "Persists upstream DNS server definitions to disk.",
			Usage:       "dns save",
			Category:    "Upstream Servers",
			Tags:        []string{"dns", "servers", "save"},
		}, runDNSSave()),

		newLegacyFactory(tui.CommandSpec{
			Context:     "server",
			Name:        "start",
			Summary:     "Start server component",
			Description: "Starts DNS or API server components.",
			Usage:       "server start <dns|api>",
			Category:    "Server",
			Tags:        []string{"server", "start"},
			Args:        []tui.ArgSpec{{Name: "component", Description: "Component to start", Required: false}},
		}, legacyRunner(handleServerStart)),
		newLegacyFactory(tui.CommandSpec{
			Context:     "server",
			Name:        "stop",
			Summary:     "Stop server component",
			Description: "Stops DNS or API server components.",
			Usage:       "server stop <dns|api>",
			Category:    "Server",
			Tags:        []string{"server", "stop"},
			Args:        []tui.ArgSpec{{Name: "component", Description: "Component to stop", Required: false}},
		}, legacyRunner(handleServerStop)),
		newLegacyFactory(tui.CommandSpec{
			Context:     "server",
			Name:        "status",
			Summary:     "Show server status",
			Description: "Displays listener details for DNS, API, and CLI clients.",
			Usage:       "server status [dns|api|client]",
			Category:    "Server",
			Tags:        []string{"server", "status"},
			Args:        []tui.ArgSpec{{Name: "component", Description: "Component to inspect", Required: false}},
		}, legacyRunner(handleServerStatus)),
		newLegacyFactory(tui.CommandSpec{
			Context:     "server",
			Name:        "configure",
			Summary:     "Configure server settings",
			Description: "Updates resolver settings or displays current configuration.",
			Usage:       "server configure [setting] [value]",
			Category:    "Server",
			Tags:        []string{"server", "configure"},
			Args: []tui.ArgSpec{
				{Name: "setting", Description: "Setting name", Required: false},
				{Name: "value", Description: "Setting value", Required: false},
			},
		}, legacyRunner(handleServerConfigure)),
		newLegacyFactory(tui.CommandSpec{
			Context:     "server",
			Name:        "load",
			Summary:     "Load server settings",
			Description: "Loads resolver settings from disk.",
			Usage:       "server load",
			Category:    "Server",
			Tags:        []string{"server", "load"},
		}, legacyRunner(handleServerLoad)),
		newLegacyFactory(tui.CommandSpec{
			Context:     "server",
			Name:        "save",
			Summary:     "Save server settings",
			Description: "Persists resolver settings to disk.",
			Usage:       "server save",
			Category:    "Server",
			Tags:        []string{"server", "save"},
		}, legacyRunner(handleServerSave)),

		newLegacyFactory(tui.CommandSpec{
			Context:     "tools",
			Name:        "dig",
			Summary:     "Query DNS servers",
			Description: "Queries all configured DNS servers for a domain or IP address and displays responses from each server.",
			Usage:       "tools dig [type] <domain|ip> [type] [@server] [port]",
			Category:    "Tools",
			Tags:        []string{"tools", "dig", "query", "diagnostics"},
			Args: []tui.ArgSpec{
				{Name: "args", Description: "Query arguments (domain/IP, type, @server, port)", Repeatable: true},
			},
			Examples: []tui.Example{
				{Description: "Query A record", Command: "tools dig example.com"},
				{Description: "Query AAAA record", Command: "tools dig example.com AAAA"},
				{Description: "Query PTR record", Command: "tools dig 8.8.8.8 PTR"},
				{Description: "Query specific server", Command: "tools dig example.com @8.8.8.8"},
				{Description: "Query with custom port", Command: "tools dig example.com @8.8.8.8 5353"},
			},
		}, runToolsDig()),
	}

	for _, cmd := range commands {
		tui.RegisterCommand(cmd)
	}
}

// Server commands rely on function variables.
func handleServerLoad(args []string) {
	if cliutil.IsHelpRequest(args) {
		printServerLoadUsage()
		return
	}
	if len(args) > 0 {
		fmt.Println("server load does not accept arguments.")
		printServerLoadUsage()
		return
	}
	dnsData := data.GetInstance()
	settings := data.LoadSettings()
	dnsData.UpdateSettings(settings)
	fmt.Println("Server settings loaded.")
}

func handleServerSave(args []string) {
	if cliutil.IsHelpRequest(args) {
		printServerSaveUsage()
		return
	}
	if len(args) > 0 {
		fmt.Println("server save does not accept arguments.")
		printServerSaveUsage()
		return
	}
	dnsData := data.GetInstance()
	data.SaveSettings(dnsData.Settings)
	fmt.Println("Server settings saved.")
}

func handleServerStart(args []string) {
	if cliutil.IsHelpRequest(args) {
		printServerStartUsage()
		printServerComponentHint()
		return
	}
	if len(args) == 0 {
		printServerStartUsage()
		printServerComponentHint()
		return
	}
	dnsData := data.GetInstance()
	settings := dnsData.Settings
	startCommands := map[string]func(){
		"dns": func() {
			if restartDNSServerFunc != nil {
				restartDNSServerFunc(settings.DNSPort)
			}
			fmt.Println("DNS server started.")
		},
		"api": func() {
			if startGinAPIFunc != nil {
				startGinAPIFunc(settings.RESTPort)
			}
			fmt.Println("API server started.")
		},
	}
	component := strings.ToLower(args[0])
	if cmd, ok := startCommands[component]; ok {
		cmd()
	} else {
		fmt.Printf("Unknown component to start: %s\n", args[0])
		printServerStartUsage()
		printServerComponentHint()
	}
}

func handleServerStop(args []string) {
	if cliutil.IsHelpRequest(args) {
		printServerStopUsage()
		printServerComponentHint()
		return
	}
	if len(args) == 0 {
		printServerStopUsage()
		printServerComponentHint()
		return
	}
	stopCommands := map[string]func(){
		"dns": func() {
			if stopDNSServerFunc != nil {
				stopDNSServerFunc()
			}
			fmt.Println("DNS server stopped.")
		},
		"api": func() {
			fmt.Println("API server stop not implemented yet.")
		},
	}
	component := strings.ToLower(args[0])
	if cmd, ok := stopCommands[component]; ok {
		cmd()
	} else {
		fmt.Printf("Unknown component to stop: %s\n", args[0])
		printServerStopUsage()
		printServerComponentHint()
	}
}

func handleServerStatus(args []string) {
	if cliutil.IsHelpRequest(args) {
		printServerStatusUsage()
		return
	}
	if len(args) > 1 {
		fmt.Println("server status accepts at most one argument.")
		printServerStatusUsage()
		return
	}
	component := "all"
	var original string
	if len(args) == 1 {
		original = strings.TrimSpace(args[0])
		if original != "" {
			component = strings.ToLower(original)
		}
	}

	info := ServerListenerInfo{DNSProtocol: "udp"}
	if getServerListenersFunc != nil {
		info = getServerListenersFunc()
		if info.DNSProtocol == "" {
			info.DNSProtocol = "udp"
		}
	}

	settings := data.GetInstance().GetResolverSettings()
	if len(info.DNSListeners) == 0 {
		info.DNSListeners = []string{fmt.Sprintf("0.0.0.0:%s", settings.DNSPort)}
	}
	if info.APIEndpoint == "" {
		info.APIEndpoint = fmt.Sprintf("0.0.0.0:%s", settings.RESTPort)
	}
	info.APIEnabled = info.APIEnabled || info.APIRunning

	dnsStatus := "stopped"
	if getServerStatusFunc != nil && getServerStatusFunc() {
		dnsStatus = "running"
	}

	formatEndpoint := func(endpoint string, enabled bool) string {
		value := strings.TrimSpace(endpoint)
		if value == "" {
			value = "not configured"
		}
		if !enabled {
			if value == "not configured" {
				return "disabled"
			}
			return fmt.Sprintf("%s (disabled)", value)
		}
		return value
	}

	printDNS := func() {
		fmt.Println("DNS Listener:")
		fmt.Printf("  Protocol: %s\n", strings.ToUpper(info.DNSProtocol))
		fmt.Printf("  Address:  %s\n", strings.Join(info.DNSListeners, ", "))
		fmt.Printf("  Status:   %s\n", dnsStatus)
	}

	printAPI := func() {
		fmt.Println("API Listener:")
		fmt.Printf("  Endpoint: %s\n", formatEndpoint(info.APIEndpoint, info.APIEnabled || info.APIRunning))
		state := "disabled"
		if info.APIEnabled || info.APIRunning {
			if info.APIRunning {
				state = "running"
			} else {
				state = "stopped"
			}
		}
		fmt.Printf("  Status:   %s\n", state)
	}

	printClients := func() {
		fmt.Println("Client Access:")
		fmt.Printf("  UNIX Socket: %s\n", formatEndpoint(info.ClientSocket, info.ClientSocketEnabled))
		fmt.Printf("  TCP:         %s\n", formatEndpoint(info.ClientTCPEndpoint, info.ClientTCPEnabled))
	}

	switch component {
	case "", "all", "dns":
		printDNS()
		if component == "dns" {
			return
		}
		fmt.Println()
		printAPI()
		fmt.Println()
		printClients()
	case "api":
		printAPI()
	case "client", "clients":
		printClients()
	default:
		display := original
		if display == "" {
			display = component
		}
		fmt.Printf("Unknown component: %s\n", display)
		printServerStatusUsage()
	}
}

func handleServerConfigure(args []string) {
	dnsData := data.GetInstance()
	settings := dnsData.Settings
	if cliutil.IsHelpRequest(args) {
		printServerConfigureUsage()
		return
	}
	if len(args) == 0 {
		fmt.Println("Current Server Configuration:")
		fmt.Printf("DNS Port: %s\n", settings.DNSPort)
		fmt.Printf("API Port: %s\n", settings.RESTPort)
		fmt.Printf("Fallback Server IP: %s\n", settings.FallbackServerIP)
		fmt.Printf("Fallback Server Port: %s\n", settings.FallbackServerPort)
		return
	}
	if len(args) < 2 {
		fmt.Println("server configure requires both setting and value.")
		printServerConfigureUsage()
		return
	}
	if len(args) > 2 {
		fmt.Println("server configure accepts exactly two arguments.")
		printServerConfigureUsage()
		return
	}
	setting := strings.ToLower(args[0])
	value := args[1]
	switch setting {
	case "dns_port":
		settings.DNSPort = value
		fmt.Printf("DNS Port set to %s\n", value)
	case "api_port":
		settings.RESTPort = value
		fmt.Printf("API Port set to %s\n", value)
	case "fallback_ip":
		settings.FallbackServerIP = value
		fmt.Printf("Fallback Server IP set to %s\n", value)
	case "fallback_port":
		settings.FallbackServerPort = value
		fmt.Printf("Fallback Server Port set to %s\n", value)
	default:
		fmt.Printf("Unknown setting: %s\n", setting)
		printServerConfigureUsage()
		return
	}
	dnsData.UpdateSettings(settings)
	fmt.Println("Server configuration updated.")
}

// Stats command
func handleStats(args []string) {
	if cliutil.IsHelpRequest(args) {
		printStatsUsage()
		return
	}
	if len(args) > 0 {
		fmt.Println("stats does not accept arguments.")
		printStatsUsage()
		return
	}
	dnsData := data.GetInstance()
	fmt.Println("Server start time:", dnsData.Stats.ServerStartTime)
	fmt.Println("Server Up Time:", serverUpTimeFormat(dnsData.Stats.ServerStartTime))
	fmt.Println()
	fmt.Println("Total Records:", len(dnsData.GetRecords()))
	fmt.Println("Total DNS Servers:", len(dnsData.GetServers()))
	fmt.Println("Total Cache Records:", len(dnsData.GetCacheRecords()))
	fmt.Println()
	fmt.Println("Total queries received:", dnsData.Stats.TotalQueries)
	fmt.Println("Total queries answered:", dnsData.Stats.TotalQueriesAnswered)
	fmt.Println("Total cache hits:", dnsData.Stats.TotalCacheHits)
	fmt.Println("Total queries forwarded:", dnsData.Stats.TotalQueriesForwarded)
}

// Helper for formatting uptime
func serverUpTimeFormat(startTime time.Time) string {
	duration := time.Since(startTime)
	days := duration / (24 * time.Hour)
	duration -= days * 24 * time.Hour
	hours := duration / time.Hour
	duration -= hours * time.Hour
	minutes := duration / time.Minute
	duration -= minutes * time.Minute
	seconds := duration / time.Second
	if days > 0 {
		return fmt.Sprintf("%d days, %d hours, %d minutes, %d seconds", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%d hours, %d minutes, %d seconds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%d minutes, %d seconds", minutes, seconds)
	}
	return fmt.Sprintf("%d seconds", seconds)
}

func printServerStartUsage() {
	fmt.Println("Usage: server start <dns|api>")
	fmt.Println("Description: Start the specified server component.")
	printHelpAliasesHint()
}

func printServerStopUsage() {
	fmt.Println("Usage: server stop <dns|api>")
	fmt.Println("Description: Stop the specified server component.")
	printHelpAliasesHint()
}

func printServerStatusUsage() {
	fmt.Println("Usage: server status [dns|api|client]")
	fmt.Println("Description: Show listener details for DNS, API, and CLI clients. Defaults to all when omitted.")
	printHelpAliasesHint()
}

func printServerComponentHint() {
	fmt.Println("Available components: dns, api")
}

func printServerConfigureUsage() {
	fmt.Println("Usage: server configure <dns_port|api_port|fallback_ip|fallback_port> <value>")
	fmt.Println("Description: Update a server configuration setting. Run without arguments to view current settings.")
	printHelpAliasesHint()
}

func printServerLoadUsage() {
	fmt.Println("Usage: server load")
	fmt.Println("Description: Load server settings from the default storage file.")
	printHelpAliasesHint()
}

func printServerSaveUsage() {
	fmt.Println("Usage: server save")
	fmt.Println("Description: Save current server settings to the default storage file.")
	printHelpAliasesHint()
}

func printStatsUsage() {
	fmt.Println("Usage: stats")
	fmt.Println("Description: Display runtime statistics for the resolver.")
	printHelpAliasesHint()
}

func printHelpAliasesHint() {
	fmt.Println("Hint: append '?', 'help', or 'h' after the command to view this usage.")
}
