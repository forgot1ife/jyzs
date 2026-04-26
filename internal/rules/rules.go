package rules

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
)

type RuleSet struct {
	Rules []Rule `json:"rules"`
}

type Rule struct {
	Name       string      `json:"name"`
	RecordType string      `json:"record_type"`
	Match      Match       `json:"match"`
	Extractors []Extractor `json:"extractors"`
}

type Match struct {
	Method       string `json:"method"`
	HostContains string `json:"host_contains"`
	PathContains string `json:"path_contains"`
	URLRegex     string `json:"url_regex"`
}

type Extractor struct {
	Field   string `json:"field"`
	Source  string `json:"source"` // request_body | response_body
	Kind    string `json:"kind"`   // gjson | regex
	Pattern string `json:"pattern"`
	Group   int    `json:"group,omitempty"`
}

func LoadRuleSet(path string) (*RuleSet, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var rs RuleSet
	if err := json.Unmarshal(b, &rs); err != nil {
		return nil, err
	}
	for i := range rs.Rules {
		if rs.Rules[i].Name == "" {
			return nil, fmt.Errorf("rule[%d] missing name", i)
		}
		if rs.Rules[i].RecordType == "" {
			rs.Rules[i].RecordType = "generic"
		}
	}
	return &rs, nil
}

func (rs *RuleSet) MatchRules(req *http.Request) []Rule {
	matched := make([]Rule, 0, len(rs.Rules))
	for _, r := range rs.Rules {
		if r.Matches(req) {
			matched = append(matched, r)
		}
	}
	return matched
}

func (r Rule) Matches(req *http.Request) bool {
	if r.Match.Method != "" && !strings.EqualFold(r.Match.Method, req.Method) {
		return false
	}
	if r.Match.HostContains != "" && !strings.Contains(strings.ToLower(req.Host), strings.ToLower(r.Match.HostContains)) {
		return false
	}
	if r.Match.PathContains != "" && !strings.Contains(strings.ToLower(req.URL.Path), strings.ToLower(r.Match.PathContains)) {
		return false
	}
	if r.Match.URLRegex != "" {
		pat, err := regexp.Compile(r.Match.URLRegex)
		if err != nil {
			return false
		}
		if !pat.MatchString(req.URL.String()) {
			return false
		}
	}
	return true
}

func (r Rule) Extract(requestBody []byte, responseBody []byte) map[string]any {
	out := map[string]any{}
	for _, ex := range r.Extractors {
		var source []byte
		switch ex.Source {
		case "request_body":
			source = requestBody
		case "response_body":
			source = responseBody
		default:
			continue
		}
		if len(source) == 0 {
			continue
		}

		switch ex.Kind {
		case "gjson":
			v := extractByGJSON(source, ex.Pattern)
			if v != nil {
				out[ex.Field] = v
			}
		case "regex":
			v := extractByRegex(source, ex.Pattern, ex.Group)
			if v != nil {
				out[ex.Field] = v
			}
		}
	}
	return out
}

func extractByGJSON(source []byte, path string) any {
	result := gjson.GetBytes(source, path)
	if !result.Exists() {
		return nil
	}
	if result.IsArray() {
		arr := result.Array()
		values := make([]any, 0, len(arr))
		for _, x := range arr {
			values = append(values, x.Value())
		}
		return values
	}
	return result.Value()
}

func extractByRegex(source []byte, pattern string, group int) any {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	m := re.FindSubmatch(source)
	if len(m) == 0 {
		return nil
	}
	if group <= 0 {
		group = 1
	}
	if group >= len(m) {
		return nil
	}
	return string(m[group])
}

