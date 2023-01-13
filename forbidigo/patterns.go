package forbidigo

import (
	"fmt"
	"regexp"
	"regexp/syntax"
	"strings"

	"gopkg.in/yaml.v2"
)

// Pattern matches code that is not supposed to be used.
type Pattern struct {
	re, pkgRe *regexp.Regexp

	// Pattern is the regular expression string that is used for matching.
	// It gets matched against the literal source code text or the expanded
	// text, depending on the mode in which the analyzer runs.
	Pattern string `yaml:"pattern"`

	// Package is a regular expression for the full package path of
	// an imported item. Ignored unless the analyzer is configured to
	// determine that information.
	Package string `yaml:"package"`

	// Msg gets printed in addition to the normal message if a match is
	// found.
	Msg string `yaml:"msg"`
}

// A YAMLPattern pattern in a YAML string may be represented either by a string
// (the traditional regular expression syntax) or a struct (for more complex
// patterns).
type YAMLPattern Pattern

func (p *YAMLPattern) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try struct first. It's unlikely that a regular expression string
	// is valid YAML for a struct.
	var pattern Pattern
	if err := unmarshal(&pattern); err != nil {
		errStr := err.Error()
		// Didn't work, try plain string.
		var ptrn string
		if err := unmarshal(&ptrn); err != nil {
			return fmt.Errorf("pattern is neither a regular expression string (%s) nor a Pattern struct (%s)", err.Error(), errStr)
		}
		p.Pattern = ptrn
	} else {
		*p = YAMLPattern(pattern)
	}
	return ((*Pattern)(p)).validate()
}

var _ yaml.Unmarshaler = &YAMLPattern{}

// parse accepts a regular expression or, if the string starts with { or contains a line break, a
// JSON or YAML representation of a Pattern.
func parse(ptrn string) (*Pattern, error) {
	pattern := &Pattern{}

	if strings.HasPrefix(strings.TrimSpace(ptrn), "{") ||
		strings.Contains(ptrn, "\n") {
		// Embedded JSON or YAML. We can decode both with the YAML decoder.
		if err := yaml.UnmarshalStrict([]byte(ptrn), pattern); err != nil {
			return nil, fmt.Errorf("parsing as JSON or YAML failed: %v", err)
		}
	} else {
		pattern.Pattern = ptrn
	}

	if err := pattern.validate(); err != nil {
		return nil, err
	}
	return pattern, nil
}

func (p *Pattern) validate() error {
	ptrnRe, err := regexp.Compile(p.Pattern)
	if err != nil {
		return fmt.Errorf("unable to compile source code pattern `%s`: %s", p.Pattern, err)
	}
	re, err := syntax.Parse(p.Pattern, syntax.Perl)
	if err != nil {
		return fmt.Errorf("unable to parse source code pattern `%s`: %s", p.Pattern, err)
	}
	msg := extractComment(re)
	if msg != "" {
		p.Msg = msg
	}
	p.re = ptrnRe

	if p.Package != "" {
		pkgRe, err := regexp.Compile(p.Package)
		if err != nil {
			return fmt.Errorf("unable to compile package pattern `%s`: %s", p.Package, err)
		}
		p.pkgRe = pkgRe
	}

	return nil
}

// Traverse the leaf submatches in the regex tree and extract a comment, if any
// is present.
func extractComment(re *syntax.Regexp) string {
	for _, sub := range re.Sub {
		subStr := sub.String()
		if strings.HasPrefix(subStr, "#") {
			return strings.TrimSpace(strings.TrimPrefix(sub.String(), "#"))
		}
		if len(sub.Sub) > 0 {
			if comment := extractComment(sub); comment != "" {
				return comment
			}
		}
	}
	return ""
}
