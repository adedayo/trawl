package narrative

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/adedayo/trawl/config/narratives"
	"github.com/adedayo/trawl/pkg/store"
)

// Entry is one curated narrative. It is the unit of the content files in
// config/narratives and the unit the lookup returns.
//
// Every text field may contain {{placeholder}} slots (see Placeholders). The
// slots are filled by plain string substitution; there is no logic in content.
type Entry struct {
	// Level is one of the Level* constants. Key identifies the entry within the
	// level: a signal identifier, a weakness class, a check family, a service
	// class, a secret type (or "<family>.*" for a whole family), or the fixed
	// keys "cve" and "generic".
	Level string `json:"level"`
	Key   string `json:"key"`
	// Label is a short human noun for the thing the entry is about, used to
	// fill {{service}} and {{secretType}} (for example "remote desktop service").
	Label string `json:"label,omitempty"`

	WhyItMatters       string   `json:"whyItMatters"`
	HowItCouldBeAbused string   `json:"howItCouldBeAbused"`
	WhatGoodLooksLike  string   `json:"whatGoodLooksLike"`
	WhatToDo           []string `json:"whatToDo"`
	HowRescanConfirms  string   `json:"howRescanConfirms"`
}

// contentFile is the on-disk shape of one document in config/narratives.
type contentFile struct {
	Version string  `json:"version"`
	Entries []Entry `json:"entries"`
}

// Placeholders lists the {{slots}} content may use.
//
//	{{service}}    label of the service class, else the class itself, else "service"
//	{{secretType}} label of the secret-type entry, else the raw type, else "secret"
//	{{cve}}        the CVE identifier, else "this vulnerability"
//	{{title}}      the finding title, else "this finding"
//	{{check}}      the check family, else "this check"
var Placeholders = []string{"service", "secretType", "cve", "title", "check"}

var placeholderPattern = regexp.MustCompile(`\{\{\s*([A-Za-z]+)\s*\}\}`)

// Keys that are fixed for the templates that are not looked up by name.
const (
	KeyCandidate = "candidate"
	KeyCVE       = "cve"
	KeyGeneric   = "generic"
)

// validLevels is the set of levels an entry may declare.
var validLevels = map[string]bool{
	LevelSignal: true, LevelWeaknessClass: true, LevelCheck: true,
	LevelServiceClass: true, LevelSecretType: true, LevelCandidate: true,
	LevelCVE: true, LevelGeneric: true,
}

// Registry is a loaded, validated narrative set. It is immutable after
// construction and safe for concurrent use. It implements Explainer.
type Registry struct {
	version string
	entries map[string]map[string]*Entry // level -> key -> entry
}

var _ Explainer = (*Registry)(nil)

// Load builds the registry from the narrative content embedded in the binary.
func Load() (*Registry, error) {
	return LoadFrom(narratives.JSON()...)
}

// LoadFrom builds a registry from one or more content documents. All documents
// must declare the same version, level and key pairs must be unique across
// them, every entry must be complete, and a generic fallback must exist, so a
// registry that loads can explain every finding.
func LoadFrom(data ...[]byte) (*Registry, error) {
	if len(data) == 0 {
		return nil, errors.New("narrative: no content documents supplied")
	}
	r := &Registry{entries: map[string]map[string]*Entry{}}
	for i, doc := range data {
		var file contentFile
		dec := json.NewDecoder(bytes.NewReader(doc))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&file); err != nil {
			return nil, fmt.Errorf("narrative: document %d: %w", i+1, err)
		}
		if strings.TrimSpace(file.Version) == "" {
			return nil, fmt.Errorf("narrative: document %d: version is required", i+1)
		}
		if r.version == "" {
			r.version = file.Version
		} else if r.version != file.Version {
			return nil, fmt.Errorf("narrative: document %d declares version %q but earlier documents declare %q; a set has one version",
				i+1, file.Version, r.version)
		}
		for j := range file.Entries {
			entry := file.Entries[j]
			if err := validateEntry(&entry); err != nil {
				return nil, fmt.Errorf("narrative: document %d entry %d: %w", i+1, j+1, err)
			}
			byKey := r.entries[entry.Level]
			if byKey == nil {
				byKey = map[string]*Entry{}
				r.entries[entry.Level] = byKey
			}
			if _, dup := byKey[entry.Key]; dup {
				return nil, fmt.Errorf("narrative: duplicate entry %s %q", entry.Level, entry.Key)
			}
			byKey[entry.Key] = &entry
		}
	}
	if r.entries[LevelGeneric][KeyGeneric] == nil {
		return nil, errors.New("narrative: no generic entry; a registry without a fallback cannot explain every finding")
	}
	return r, nil
}

// normalizeKey makes keys case-insensitive in lookup. Signal identifiers are
// upper case by convention; everything else is lower case.
func normalizeKey(level, key string) string {
	key = strings.TrimSpace(key)
	if level == LevelSignal {
		return strings.ToUpper(key)
	}
	return strings.ToLower(key)
}

func validateEntry(e *Entry) error {
	if !validLevels[e.Level] {
		return fmt.Errorf("unknown level %q", e.Level)
	}
	e.Key = normalizeKey(e.Level, e.Key)
	if e.Key == "" {
		return errors.New("key is required")
	}
	if e.Level == LevelCandidate && e.Key != KeyCandidate {
		return fmt.Errorf("the candidate level has the single key %q, got %q", KeyCandidate, e.Key)
	}
	if e.Level == LevelCVE && e.Key != KeyCVE {
		return fmt.Errorf("the cve level has the single key %q, got %q", KeyCVE, e.Key)
	}
	if e.Level == LevelGeneric && e.Key != KeyGeneric {
		return fmt.Errorf("the generic level has the single key %q, got %q", KeyGeneric, e.Key)
	}
	where := fmt.Sprintf("%s %q", e.Level, e.Key)
	required := map[string]string{
		"whyItMatters":       e.WhyItMatters,
		"howItCouldBeAbused": e.HowItCouldBeAbused,
		"whatGoodLooksLike":  e.WhatGoodLooksLike,
		"howRescanConfirms":  e.HowRescanConfirms,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s: %s is required", where, name)
		}
	}
	if len(e.WhatToDo) == 0 {
		return fmt.Errorf("%s: whatToDo needs at least one step", where)
	}
	for _, step := range e.WhatToDo {
		if strings.TrimSpace(step) == "" {
			return fmt.Errorf("%s: whatToDo contains an empty step", where)
		}
	}
	for _, text := range e.texts() {
		for _, m := range placeholderPattern.FindAllStringSubmatch(text, -1) {
			if !knownPlaceholder(m[1]) {
				return fmt.Errorf("%s: unknown placeholder {{%s}}", where, m[1])
			}
		}
	}
	return nil
}

// texts returns every text field of the entry, for validation and linting.
func (e *Entry) texts() []string {
	out := []string{e.WhyItMatters, e.HowItCouldBeAbused, e.WhatGoodLooksLike, e.HowRescanConfirms}
	return append(out, e.WhatToDo...)
}

func knownPlaceholder(name string) bool {
	for _, p := range Placeholders {
		if p == name {
			return true
		}
	}
	return false
}

// Version is the content version of the narrative set, recorded in the view so
// a reader can tell which wording they were shown.
func (r *Registry) Version() string { return r.version }

// Keys returns the sorted keys present at a level.
func (r *Registry) Keys(level string) []string {
	keys := make([]string, 0, len(r.entries[level]))
	for key := range r.entries[level] {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Entry returns a copy of the entry at level and key.
func (r *Registry) Entry(level, key string) (Entry, bool) {
	e, ok := r.entries[level][normalizeKey(level, key)]
	if !ok {
		return Entry{}, false
	}
	cp := *e
	cp.WhatToDo = append([]string(nil), e.WhatToDo...)
	return cp, true
}

// Resolve reports which entry would supply the wording for in, most specific
// first:
//
//	SignalID -> WeaknessClass -> Check -> ServiceClass -> SecretType -> CVE -> generic
//
// A secret type is matched exactly first, then by family ("aws.access_key"
// falls back to "aws.*"). The order is a single table so that changing the
// precedence is a one-line change with a test that names it.
func (r *Registry) Resolve(in Input) (level, key string, ok bool) {
	for _, step := range lookupOrder {
		for _, candidate := range step.candidates(in) {
			if candidate == "" {
				continue
			}
			if _, found := r.entries[step.level][normalizeKey(step.level, candidate)]; found {
				return step.level, normalizeKey(step.level, candidate), true
			}
		}
	}
	return "", "", false
}

type lookupStep struct {
	level      string
	candidates func(Input) []string
}

var lookupOrder = []lookupStep{
	{LevelSignal, func(in Input) []string { return []string{in.SignalID} }},
	{LevelCheck, func(in Input) []string { return []string{in.Check} }},
	{LevelWeaknessClass, func(in Input) []string { return []string{in.WeaknessClass} }},
	{LevelServiceClass, func(in Input) []string { return []string{in.ServiceClass} }},
	{LevelSecretType, func(in Input) []string {
		t := strings.ToLower(strings.TrimSpace(in.SecretType))
		if t == "" {
			return nil
		}
		out := []string{t}
		if family, _, found := strings.Cut(t, "."); found {
			out = append(out, family+".*")
		}
		return out
	}},
	{LevelCandidate, func(in Input) []string {
		if in.Source == store.SourceCandidate {
			return []string{KeyCandidate}
		}
		return nil
	}},
	{LevelCVE, func(in Input) []string {
		if strings.TrimSpace(in.CVE) == "" {
			return nil
		}
		return []string{KeyCVE}
	}},
	{LevelGeneric, func(Input) []string { return []string{KeyGeneric} }},
}

// Explain renders the explanation for in. It is deterministic: the same input
// always yields the same output, and nothing is persisted.
func (r *Registry) Explain(in Input) Explanation {
	level, key, _ := r.Resolve(in) // the generic entry always exists
	entry := r.entries[level][key]
	subst := r.substituter(in, entry)

	unassessed := !in.CheckState.Assessed() && strings.TrimSpace(string(in.CheckState)) != ""

	steps := make([]string, 0, len(entry.WhatToDo)+3)
	for _, step := range entry.WhatToDo {
		steps = append(steps, subst(step))
	}

	out := Explanation{
		Level:              level,
		LevelKey:           key,
		Version:            r.version,
		HowItCouldBeAbused: subst(entry.HowItCouldBeAbused),
		HowRescanConfirms:  subst(entry.HowRescanConfirms),
		WhyItMatters:       subst(entry.WhyItMatters),
		WhatGoodLooksLike:  subst(entry.WhatGoodLooksLike),
	}

	if in.Catalogue != nil {
		out.References = append([]string(nil), in.Catalogue.References...)
		out.Standards = standardsFrom(in.Catalogue.Tags)
		steps = appendRemediation(steps, in.Catalogue.Remediation)
	}
	if out.References == nil {
		out.References = []string{}
	}

	if unassessed {
		out.CoverageNote = coverageNote(in)
		out.WhatWeFound = whatWeFoundUnassessed(in)
		out.WhyItMatters = unassessedWhyPrefix + " " + out.WhyItMatters
		out.WhatGoodLooksLike = unassessedGoodPrefix + " " + out.WhatGoodLooksLike
		steps = append([]string{unassessedStep1, unassessedStep2}, steps...)
	} else {
		out.WhatWeFound = whatWeFound(in)
	}
	out.WhatToDo = steps
	out.ExploitationStatus = exploitationStatus(in)
	return out
}

// Fixed wording used when the control was not assessed. It lives in code, not
// content, because the rule it serves (never imply an unassessed control is in
// place) must not be editable away by a content change.
const (
	unassessedWhyPrefix  = "This control could not be assessed, so what follows explains why it matters in general. It is not a statement about your setup."
	unassessedGoodPrefix = "Because this control could not be assessed, Trawl cannot say whether it is in place. For reference, the target state is described as follows."
	unassessedStep1      = "Find out why the check did not conclude, using the reason recorded for it, and run it again so the control can be assessed."
	unassessedStep2      = "If the next assessment confirms a weakness, continue with the steps below."
)

func (r *Registry) substituter(in Input, entry *Entry) func(string) string {
	service := firstNonEmpty(in.ServiceClass)
	if e := r.entries[LevelServiceClass][normalizeKey(LevelServiceClass, in.ServiceClass)]; e != nil && e.Label != "" {
		service = e.Label
	}
	if service == "" {
		service = "service"
	}

	secret := strings.TrimSpace(in.SecretType)
	if entry.Level == LevelSecretType && entry.Label != "" {
		secret = entry.Label
	} else if secret != "" {
		t := strings.ToLower(secret)
		if e := r.entries[LevelSecretType][t]; e != nil && e.Label != "" {
			secret = e.Label
		} else if family, _, found := strings.Cut(t, "."); found {
			if e := r.entries[LevelSecretType][family+".*"]; e != nil && e.Label != "" {
				secret = e.Label
			}
		}
	}
	if secret == "" {
		secret = "secret"
	}

	values := map[string]string{
		"service":    service,
		"secretType": secret,
		"cve":        firstNonEmpty(strings.TrimSpace(in.CVE), "this vulnerability"),
		"title":      firstNonEmpty(titleOf(in), "this finding"),
		"check":      firstNonEmpty(strings.TrimSpace(in.Check), "this check"),
	}
	return func(text string) string {
		// A single pass: substituted values are never rescanned, so input text
		// that happens to contain a slot cannot expand into anything.
		return placeholderPattern.ReplaceAllStringFunc(text, func(m string) string {
			name := placeholderPattern.FindStringSubmatch(m)[1]
			if v, ok := values[name]; ok {
				return v
			}
			return m
		})
	}
}

func titleOf(in Input) string {
	if in.Catalogue != nil && strings.TrimSpace(in.Catalogue.Title) != "" {
		return strings.TrimSpace(in.Catalogue.Title)
	}
	return strings.TrimSpace(in.Title)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// whatWeFound is built from the catalogue (or the finding's own title) and then
// this occurrence's detail and evidence. It is the only section that carries
// occurrence data.
func whatWeFound(in Input) string {
	var paras []string
	title := titleOf(in)
	if title != "" {
		paras = append(paras, endSentence(title))
	}
	desc := ""
	if in.Catalogue != nil {
		desc = strings.TrimSpace(in.Catalogue.Description)
	}
	if desc != "" {
		paras = append(paras, desc)
	}
	paras = append(paras, occurrenceParagraphs(in, "In this case: ", "Evidence recorded: ", desc)...)
	if len(paras) == 0 {
		return "Trawl raised a finding of this kind."
	}
	return strings.Join(paras, "\n\n")
}

// whatWeFoundUnassessed says what was being checked without asserting what was
// found, because nothing was.
func whatWeFoundUnassessed(in Input) string {
	paras := []string{"Trawl could not assess this control, so it draws no conclusion about whether the control is in place."}
	if title := titleOf(in); title != "" {
		paras = append(paras, "The condition this check looks for is: "+endSentence(title))
	}
	paras = append(paras, occurrenceParagraphs(in, "Last detail recorded: ", "Last evidence recorded: ", "")...)
	return strings.Join(paras, "\n\n")
}

func occurrenceParagraphs(in Input, detailLead, evidenceLead, alreadySaid string) []string {
	var paras []string
	detail := strings.TrimSpace(in.Detail)
	if detail != "" && detail != strings.TrimSpace(alreadySaid) && detail != titleOf(in) {
		paras = append(paras, detailLead+endSentence(detail))
	}
	if evidence := strings.TrimSpace(in.Evidence); evidence != "" {
		paras = append(paras, evidenceLead+evidence)
	}
	return paras
}

func endSentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	switch s[len(s)-1] {
	case '.', '!', '?':
		return s
	}
	return s + "."
}

// coverageNote is set when the assessment behind the finding did not conclude.
func coverageNote(in Input) string {
	var b strings.Builder
	b.WriteString("This control could not be assessed. ")
	switch in.CheckState {
	case store.CoverageNotChecked:
		b.WriteString("The check did not run, so Trawl cannot say whether the control is in place or missing.")
	case store.CoverageCheckFailed:
		b.WriteString("The check ran but could not reach a conclusion, so Trawl cannot say whether the control is in place or missing.")
	default:
		b.WriteString("The assessment state was not one Trawl recognizes, so it is treated as not assessed and no conclusion is drawn.")
	}
	if reason := strings.TrimSpace(in.CheckReason); reason != "" {
		b.WriteString(" Reason recorded: ")
		b.WriteString(endSentence(reason))
	}
	return b.String()
}

// standardsFrom keeps the catalogue tags that name a compliance standard and
// drops the topical ones (transport-security, pki and so on).
func standardsFrom(tags []string) []string {
	var out []string
	for _, tag := range tags {
		if standardTags[tag] {
			out = append(out, tag)
		}
	}
	return out
}

var standardTags = map[string]bool{
	"ncsc-mailcheck": true,
	"cis-controls":   true,
	"nist-800-81":    true,
	"pci-dss":        true,
}

// appendRemediation adds the catalogue's technical remediation as a final step
// unless an existing step already says the same thing.
func appendRemediation(steps []string, remediation string) []string {
	remediation = strings.TrimSpace(remediation)
	if remediation == "" {
		return steps
	}
	norm := normalizeText(remediation)
	for _, step := range steps {
		s := normalizeText(step)
		if s == norm || strings.Contains(s, norm) || strings.Contains(norm, s) {
			return steps
		}
	}
	return append(steps, remediation)
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func normalizeText(s string) string {
	return strings.TrimSpace(nonAlnum.ReplaceAllString(strings.ToLower(s), " "))
}

// Exploitation wording. Never a probability, a percentage or a score.

const (
	kevCatalogue   = "the public exploited-vulnerabilities catalogue"
	exploitUnknown = "Whether this weakness is being used in real attacks could not be established."
)

func exploitationStatus(in Input) string {
	hasCVE := strings.TrimSpace(in.CVE) != ""
	if !hasCVE && in.KEVState == "" && in.EPSSState == "" {
		return ""
	}
	var parts []string

	switch store.EnrichmentState(in.KEVState) {
	case store.EnrichmentOK:
		if in.KEVListed {
			parts = append(parts, fmt.Sprintf("This weakness is known to be used in real attacks (checked against %s%s).",
				kevCatalogue, onDate(in.KEVCheckedAt)))
		} else {
			parts = append(parts, fmt.Sprintf("This weakness was not listed in %s%s. That does not mean it is not being used in attacks, only that it was not listed at that time.",
				kevCatalogue, onDate(in.KEVCheckedAt)))
		}
	case store.EnrichmentNotFound:
		parts = append(parts, fmt.Sprintf("No entry for this weakness was found in %s%s. That does not rule out use in real attacks, only that it was not listed at that time.",
			kevCatalogue, onDate(in.KEVCheckedAt)))
	default: // not_checked, check_failed, empty or unrecognized
		parts = append(parts, exploitUnknown)
	}

	if in.EPSSHigh && epssUsable(in.EPSSState) {
		parts = append(parts, fmt.Sprintf("Public attack-forecasting data (the Exploit Prediction Scoring System) also rates the likelihood of this weakness being used in attacks as elevated%s.",
			onDate(in.EPSSCheckedAt)))
	}
	return strings.Join(parts, " ")
}

func epssUsable(state string) bool {
	switch store.EnrichmentState(state) {
	case store.EnrichmentNotChecked, store.EnrichmentCheckFailed, store.EnrichmentNotFound:
		return false
	}
	return true
}

// onDate renders " on January 2, 2006" in UTC, or says the date was not
// recorded. The date is the point: a status without one cannot be audited.
func onDate(checkedAt string) string {
	checkedAt = strings.TrimSpace(checkedAt)
	if checkedAt == "" {
		return "; the date of that check was not recorded"
	}
	t, err := time.Parse(time.RFC3339, checkedAt)
	if err != nil {
		return "; the date of that check was not recorded"
	}
	return " on " + t.UTC().Format("January 2, 2006")
}
