// Package config holds pico-lsp settings and implements the LSP configuration
// contract: values are pulled from the pico8 section when the client supports
// workspace/configuration, with initializationOptions.pico8 as the fallback.
package config

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
	"go.lsp.dev/protocol"
)

const (
	// Section is the LSP configuration section owned by pico-lsp.
	Section = "pico8"

	DefaultAPIVersion   = "0.2.7"
	DefaultTypeChecking = "basic"
	DefaultMaxProblems  = 100

	DefaultUndefinedGlobal         = "warning"
	DefaultUnusedLocal             = "off"
	DefaultUnreachableCode         = "hint"
	DefaultDuplicateTableField     = "hint"
	DefaultCallbackPrecedence      = "hint"
	DefaultCallbackParams          = "warning"
	DefaultNoStdlib                = "hint"
	DefaultShadowedLocal           = "off"
	DefaultEmptyBlock              = "off"
	DefaultGlobalInFunction        = "off"
	DefaultIncludeOutsideWorkspace = "warning"
	DefaultIncludeUnusualType      = "hint"
	DefaultIncludeDuplicate        = "off"

	SeverityOff     = "off"
	SeverityHint    = "hint"
	SeverityInfo    = "info"
	SeverityWarning = "warning"
	SeverityError   = "error"

	TypeCheckingOff    = "off"
	TypeCheckingBasic  = "basic"
	TypeCheckingStrict = "strict"
)

// Settings is the pico8 configuration section.
type Settings struct {
	APIVersion   string `json:"apiVersion"`
	TypeChecking string `json:"typeChecking"`
	InlayHints   bool   `json:"inlayHints"`
	// MaxProblems is the diagnostic cap. Zero disables truncation.
	MaxProblems int                `json:"maxProblems"`
	Limits      LimitsSettings     `json:"limits"`
	Format      FormatSettings     `json:"format"`
	Diagnostics DiagnosticSettings `json:"diagnostics"`
	Lint        LintSettings       `json:"lint"`
}

// LimitsSettings controls cart budget UI.
type LimitsSettings struct {
	CodeLens    bool `json:"codeLens"`
	Diagnostics bool `json:"diagnostics"`
}

// FormatSettings controls formatter behavior.
type FormatSettings struct {
	MaxBlankLines int `json:"maxBlankLines"`
}

// LintSettings controls optional PICO-8 lint checks.
type LintSettings struct {
	APIRanges bool `json:"apiRanges"`
}

// DiagnosticSettings overrides diagnostic severities.
type DiagnosticSettings struct {
	UndefinedGlobal         string `json:"undefinedGlobal"`
	UnusedLocal             string `json:"unusedLocal"`
	UnreachableCode         string `json:"unreachableCode"`
	DuplicateTableField     string `json:"duplicateTableField"`
	CallbackPrecedence      string `json:"callbackPrecedence"`
	CallbackParams          string `json:"callbackParams"`
	NoStdlib                string `json:"noStdlib"`
	ShadowedLocal           string `json:"shadowedLocal"`
	EmptyBlock              string `json:"emptyBlock"`
	GlobalInFunction        string `json:"globalInFunction"`
	IncludeOutsideWorkspace string `json:"includeOutsideWorkspace"`
	IncludeUnusualType      string `json:"includeUnusualType"`
	IncludeDuplicate        string `json:"includeDuplicate"`
}

// Default returns the documented default settings.
func Default() Settings {
	return Settings{
		APIVersion:   DefaultAPIVersion,
		TypeChecking: DefaultTypeChecking,
		InlayHints:   false,
		MaxProblems:  DefaultMaxProblems,
		Limits: LimitsSettings{
			CodeLens:    true,
			Diagnostics: true,
		},
		Format: FormatSettings{
			MaxBlankLines: 2,
		},
		Diagnostics: DiagnosticSettings{
			UndefinedGlobal:         DefaultUndefinedGlobal,
			UnusedLocal:             DefaultUnusedLocal,
			UnreachableCode:         DefaultUnreachableCode,
			DuplicateTableField:     DefaultDuplicateTableField,
			CallbackPrecedence:      DefaultCallbackPrecedence,
			CallbackParams:          DefaultCallbackParams,
			NoStdlib:                DefaultNoStdlib,
			ShadowedLocal:           DefaultShadowedLocal,
			EmptyBlock:              DefaultEmptyBlock,
			GlobalInFunction:        DefaultGlobalInFunction,
			IncludeOutsideWorkspace: DefaultIncludeOutsideWorkspace,
			IncludeUnusualType:      DefaultIncludeUnusualType,
			IncludeDuplicate:        DefaultIncludeDuplicate,
		},
		Lint: LintSettings{APIRanges: false},
	}
}

// Manager stores the active settings and handles configuration pulls. It is
// safe for concurrent use.
type Manager struct {
	log  *slog.Logger
	mu   sync.RWMutex
	cfg  Settings
	init Settings
	pull bool
}

// New returns a Manager populated with defaults.
func New(log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	settings := Default()
	return &Manager{
		log:  log,
		cfg:  settings,
		init: settings,
	}
}

// Settings returns a copy of the active settings.
func (m *Manager) Settings() Settings {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

// SetPullSupported records whether the client supports workspace/configuration.
func (m *Manager) SetPullSupported(supported bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pull = supported
}

// SupportsPull reports whether the client supports workspace/configuration.
func (m *Manager) SupportsPull() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pull
}

// ApplyInitializationOptions applies initializationOptions.pico8. Missing or
// malformed values keep the current defaults; invalid enum values fall back to
// their defaults individually.
func (m *Manager) ApplyInitializationOptions(options protocol.LSPAny) {
	raw, ok, err := pico8Value(options)
	if err != nil {
		m.log.Warn("ignoring invalid initializationOptions", "error", err)
		return
	}
	if !ok {
		return
	}

	next, err := decode(raw, Default())
	if err != nil {
		m.log.Warn("ignoring invalid initializationOptions.pico8", "error", err)
		return
	}
	m.storeInitialization(next)
}

// ApplyDidChange applies settings pushed by workspace/didChangeConfiguration
// when the client does not support configuration pulls.
func (m *Manager) ApplyDidChange(settings protocol.LSPAny) {
	raw, ok, err := pico8Value(settings)
	if err != nil {
		m.log.Warn("ignoring invalid didChangeConfiguration settings", "error", err)
		return
	}
	if !ok {
		return
	}

	next, err := decode(raw, m.initializationSettings())
	if err != nil {
		m.log.Warn("ignoring invalid didChangeConfiguration.pico8", "error", err)
		return
	}
	m.store(next)
}

// Pull requests the pico8 section from the client. It is a no-op when the
// client does not support configuration pulls.
func (m *Manager) Pull(ctx context.Context, client protocol.Client) error {
	if !m.SupportsPull() {
		return nil
	}
	if client == nil {
		return fmt.Errorf("config: nil client")
	}

	section := Section
	values, err := client.Configuration(ctx, &protocol.ConfigurationParams{
		Items: []protocol.ConfigurationItem{{Section: &section}},
	})
	if err != nil {
		return fmt.Errorf("config: workspace/configuration: %w", err)
	}

	next := m.initializationSettings()
	if len(values) > 0 && !isJSONNull(values[0]) {
		next, err = decode(values[0], next)
		if err != nil {
			return fmt.Errorf("config: decode %q section: %w", Section, err)
		}
		if len(values) > 1 {
			m.log.Warn("client returned more than one configuration value", "section", Section, "count", len(values))
		}
	}

	m.store(next)
	return nil
}

func (m *Manager) initializationSettings() Settings {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.init
}

func (m *Manager) storeInitialization(next Settings) {
	next = normalize(next, m.log)

	m.mu.Lock()
	m.init = next
	m.cfg = next
	m.mu.Unlock()

	m.logSettings("configuration updated from initializationOptions", next)
}

func (m *Manager) store(next Settings) {
	next = normalize(next, m.log)

	m.mu.Lock()
	m.cfg = next
	m.mu.Unlock()

	m.logSettings("configuration updated", next)
}

func (m *Manager) logSettings(message string, settings Settings) {
	m.log.Debug(message,
		"apiVersion", settings.APIVersion,
		"typeChecking", settings.TypeChecking,
		"inlayHints", settings.InlayHints,
		"maxProblems", settings.MaxProblems,
	)
}

// pico8Value extracts the pico8 member from an LSP JSON value.
func pico8Value(raw protocol.LSPAny) (value jsontext.Value, ok bool, err error) {
	if isJSONNull(raw) {
		return nil, false, nil
	}

	var envelope struct {
		Pico8 jsontext.Value `json:"pico8"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, false, err
	}
	if isJSONNull(envelope.Pico8) {
		return nil, false, nil
	}
	return envelope.Pico8, true, nil
}

// partialSettings records only the fields a client actually supplied. A JSON
// null leaves the pointer nil, so it means "no override" rather than zeroing
// the corresponding setting.
type partialSettings struct {
	APIVersion   *string                    `json:"apiVersion"`
	TypeChecking *string                    `json:"typeChecking"`
	InlayHints   *bool                      `json:"inlayHints"`
	MaxProblems  *int                       `json:"maxProblems"`
	Limits       *partialLimitsSettings     `json:"limits"`
	Format       *partialFormatSettings     `json:"format"`
	Diagnostics  *partialDiagnosticSettings `json:"diagnostics"`
	Lint         *partialLintSettings       `json:"lint"`
}

type partialLimitsSettings struct {
	CodeLens    *bool `json:"codeLens"`
	Diagnostics *bool `json:"diagnostics"`
}

type partialFormatSettings struct {
	MaxBlankLines *int `json:"maxBlankLines"`
}

type partialLintSettings struct {
	APIRanges *bool `json:"apiRanges"`
}

type partialDiagnosticSettings struct {
	UndefinedGlobal         *string `json:"undefinedGlobal"`
	UnusedLocal             *string `json:"unusedLocal"`
	UnreachableCode         *string `json:"unreachableCode"`
	DuplicateTableField     *string `json:"duplicateTableField"`
	CallbackPrecedence      *string `json:"callbackPrecedence"`
	CallbackParams          *string `json:"callbackParams"`
	NoStdlib                *string `json:"noStdlib"`
	ShadowedLocal           *string `json:"shadowedLocal"`
	EmptyBlock              *string `json:"emptyBlock"`
	GlobalInFunction        *string `json:"globalInFunction"`
	IncludeOutsideWorkspace *string `json:"includeOutsideWorkspace"`
	IncludeUnusualType      *string `json:"includeUnusualType"`
	IncludeDuplicate        *string `json:"includeDuplicate"`
}

func (p partialSettings) apply(base Settings) Settings {
	if p.APIVersion != nil {
		base.APIVersion = *p.APIVersion
	}
	if p.TypeChecking != nil {
		base.TypeChecking = *p.TypeChecking
	}
	if p.InlayHints != nil {
		base.InlayHints = *p.InlayHints
	}
	if p.MaxProblems != nil {
		base.MaxProblems = *p.MaxProblems
	}
	if p.Limits != nil {
		if p.Limits.CodeLens != nil {
			base.Limits.CodeLens = *p.Limits.CodeLens
		}
		if p.Limits.Diagnostics != nil {
			base.Limits.Diagnostics = *p.Limits.Diagnostics
		}
	}
	if p.Format != nil && p.Format.MaxBlankLines != nil {
		base.Format.MaxBlankLines = *p.Format.MaxBlankLines
	}
	if p.Lint != nil && p.Lint.APIRanges != nil {
		base.Lint.APIRanges = *p.Lint.APIRanges
	}
	if p.Diagnostics != nil {
		if p.Diagnostics.UndefinedGlobal != nil {
			base.Diagnostics.UndefinedGlobal = *p.Diagnostics.UndefinedGlobal
		}
		if p.Diagnostics.UnusedLocal != nil {
			base.Diagnostics.UnusedLocal = *p.Diagnostics.UnusedLocal
		}
		if p.Diagnostics.UnreachableCode != nil {
			base.Diagnostics.UnreachableCode = *p.Diagnostics.UnreachableCode
		}
		if p.Diagnostics.DuplicateTableField != nil {
			base.Diagnostics.DuplicateTableField = *p.Diagnostics.DuplicateTableField
		}
		if p.Diagnostics.CallbackPrecedence != nil {
			base.Diagnostics.CallbackPrecedence = *p.Diagnostics.CallbackPrecedence
		}
		if p.Diagnostics.CallbackParams != nil {
			base.Diagnostics.CallbackParams = *p.Diagnostics.CallbackParams
		}
		if p.Diagnostics.NoStdlib != nil {
			base.Diagnostics.NoStdlib = *p.Diagnostics.NoStdlib
		}
		if p.Diagnostics.ShadowedLocal != nil {
			base.Diagnostics.ShadowedLocal = *p.Diagnostics.ShadowedLocal
		}
		if p.Diagnostics.EmptyBlock != nil {
			base.Diagnostics.EmptyBlock = *p.Diagnostics.EmptyBlock
		}
		if p.Diagnostics.GlobalInFunction != nil {
			base.Diagnostics.GlobalInFunction = *p.Diagnostics.GlobalInFunction
		}
		if p.Diagnostics.IncludeOutsideWorkspace != nil {
			base.Diagnostics.IncludeOutsideWorkspace = *p.Diagnostics.IncludeOutsideWorkspace
		}
		if p.Diagnostics.IncludeUnusualType != nil {
			base.Diagnostics.IncludeUnusualType = *p.Diagnostics.IncludeUnusualType
		}
		if p.Diagnostics.IncludeDuplicate != nil {
			base.Diagnostics.IncludeDuplicate = *p.Diagnostics.IncludeDuplicate
		}
	}
	return base
}

func decode(raw jsontext.Value, base Settings) (Settings, error) {
	if isJSONNull(raw) {
		return base, nil
	}

	var partial partialSettings
	if err := json.Unmarshal(raw, &partial); err != nil {
		return base, err
	}
	return partial.apply(base), nil
}

func isJSONNull(raw []byte) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null"))
}

// warnedAPIVersions makes unsupported apiVersion warnings one-time per value.
var warnedAPIVersions sync.Map

// normalize validates enum-like fields. Invalid values fall back to defaults
// and produce a warning; unknown JSON keys were already ignored by the decoder.
func normalize(s Settings, log *slog.Logger) Settings {
	d := Default()

	if s.APIVersion == "" {
		s.APIVersion = d.APIVersion
	}
	if s.APIVersion != d.APIVersion {
		if _, seen := warnedAPIVersions.LoadOrStore(s.APIVersion, struct{}{}); !seen {
			log.Warn("unsupported pico8.apiVersion", "value", s.APIVersion, "default", d.APIVersion)
		}
		s.APIVersion = d.APIVersion
	}

	switch s.TypeChecking {
	case TypeCheckingOff, TypeCheckingBasic, TypeCheckingStrict:
	case "":
		s.TypeChecking = d.TypeChecking
	default:
		log.Warn("invalid pico8.typeChecking", "value", s.TypeChecking, "default", d.TypeChecking)
		s.TypeChecking = d.TypeChecking
	}

	if s.MaxProblems < 0 {
		s.MaxProblems = d.MaxProblems
	}
	if s.Format.MaxBlankLines < 0 {
		s.Format.MaxBlankLines = d.Format.MaxBlankLines
	}

	s.Diagnostics.UndefinedGlobal = normalizeSeverity(s.Diagnostics.UndefinedGlobal, d.Diagnostics.UndefinedGlobal, "undefinedGlobal", log)
	s.Diagnostics.UnusedLocal = normalizeSeverity(s.Diagnostics.UnusedLocal, d.Diagnostics.UnusedLocal, "unusedLocal", log)
	s.Diagnostics.UnreachableCode = normalizeSeverity(s.Diagnostics.UnreachableCode, d.Diagnostics.UnreachableCode, "unreachableCode", log)
	s.Diagnostics.DuplicateTableField = normalizeSeverity(s.Diagnostics.DuplicateTableField, d.Diagnostics.DuplicateTableField, "duplicateTableField", log)
	s.Diagnostics.CallbackPrecedence = normalizeSeverity(s.Diagnostics.CallbackPrecedence, d.Diagnostics.CallbackPrecedence, "callbackPrecedence", log)
	s.Diagnostics.CallbackParams = normalizeSeverity(s.Diagnostics.CallbackParams, d.Diagnostics.CallbackParams, "callbackParams", log)
	s.Diagnostics.NoStdlib = normalizeSeverity(s.Diagnostics.NoStdlib, d.Diagnostics.NoStdlib, "noStdlib", log)
	s.Diagnostics.ShadowedLocal = normalizeSeverity(s.Diagnostics.ShadowedLocal, d.Diagnostics.ShadowedLocal, "shadowedLocal", log)
	s.Diagnostics.EmptyBlock = normalizeSeverity(s.Diagnostics.EmptyBlock, d.Diagnostics.EmptyBlock, "emptyBlock", log)
	s.Diagnostics.GlobalInFunction = normalizeSeverity(s.Diagnostics.GlobalInFunction, d.Diagnostics.GlobalInFunction, "globalInFunction", log)
	s.Diagnostics.IncludeOutsideWorkspace = normalizeSeverity(s.Diagnostics.IncludeOutsideWorkspace, d.Diagnostics.IncludeOutsideWorkspace, "includeOutsideWorkspace", log)
	s.Diagnostics.IncludeUnusualType = normalizeSeverity(s.Diagnostics.IncludeUnusualType, d.Diagnostics.IncludeUnusualType, "includeUnusualType", log)
	s.Diagnostics.IncludeDuplicate = normalizeSeverity(s.Diagnostics.IncludeDuplicate, d.Diagnostics.IncludeDuplicate, "includeDuplicate", log)

	return s
}

func normalizeSeverity(value, def, name string, log *slog.Logger) string {
	switch value {
	case SeverityOff, SeverityHint, SeverityInfo, SeverityWarning, SeverityError:
		return value
	case "":
		return def
	default:
		log.Warn("invalid pico8 diagnostic severity", "setting", name, "value", value, "default", def)
		return def
	}
}
