package plugins

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/alexandre-normand/slackscot"
	"github.com/alexandre-normand/slackscot/actions"
	"github.com/alexandre-normand/slackscot/config"
	"github.com/alexandre-normand/slackscot/plugin"
)

const (
	// SchoolcodePluginName is the plugin name
	SchoolcodePluginName = "schoolcode"
	maxResults           = 10
)

// school codes are always 4 digits, so match on that
var schoolcodeRegex = regexp.MustCompile("^find school (\\d{4})")

// re2 doesn't support the (?!re) syntax, so hack it by matching on leading
// non-digit char
var schoolstringRegex = regexp.MustCompile("^find school ([\\D].*)")

// Schoolcode holds data for the schoolcode plugin
type Schoolcode struct {
	*slackscot.Plugin
	answerOptions []slackscot.AnswerOption
}

// NewSchoolcode creates a new instance of the schoolcode plugin
func NewSchoolcode(config *config.PluginConfig) (*slackscot.Plugin, error) {
	// Unmarshal the PluginConfig
	sConfig := struct {
		ThreadedReplies bool
	}{}
	err := config.Unmarshal(&sConfig)
	if err != nil {
		return nil, fmt.Errorf("[%s] Unable to unmarshal config: %w", SchoolcodePluginName, err)
	}

	s := new(Schoolcode)
	// if enabled, send all replies in thread (except when in DM)
	if sConfig.ThreadedReplies {
		s.answerOptions = append(s.answerOptions, slackscot.AnswerInThread())
	}

	s.Plugin = plugin.New(SchoolcodePluginName).
		WithCommand(actions.NewCommand().
			WithMatcher(func(m *slackscot.IncomingMessage) bool {
				return schoolcodeRegex.MatchString(m.NormalizedText)
			}).
			WithUsage("find school 1234").
			WithDescription("Find school name for school code 1234").
			WithAnswerer(s.schoolcodeAnswerer).
			Build()).
		WithCommand(actions.NewCommand().
			WithMatcher(func(m *slackscot.IncomingMessage) bool {
				return schoolstringRegex.MatchString(m.NormalizedText)
			}).
			WithUsage("find school <string>").
			WithDescription("Find school name containing <string> or with exact initials <string>").
			WithAnswerer(s.schoolstringAnswerer).
			Build()).
		Build()

	return s.Plugin, nil
}

func (s *Schoolcode) schoolcodeAnswerer(m *slackscot.IncomingMessage) *slackscot.Answer {
	matches := schoolcodeRegex.FindStringSubmatch(m.NormalizedText)
	if len(matches) < 2 {
		s.Logger.Printf("[%s] Missing regexp match on message [%v]", SchoolcodePluginName, m)
	}
	code := matches[1]

	schoolname, ok := schoolcodeData[code]
	var answerText string
	switch ok {
	case false:
		answerText = fmt.Sprintf("No school found with school code %s.", code)
	case true:
		answerText = fmt.Sprintf("`%s` %s", code, schoolname)
	}
	return &slackscot.Answer{
		Text:    answerText,
		Options: s.answerOptions,
	}
}

func (s *Schoolcode) schoolstringAnswerer(m *slackscot.IncomingMessage) *slackscot.Answer {
	matches := schoolstringRegex.FindStringSubmatch(m.NormalizedText)
	if len(matches) < 2 {
		s.Logger.Printf("[%s] Missing regexp match on message [%v]", SchoolcodePluginName, m)
	}
	// trim is necessary as trailing whitespace is picked up by the regexp
	query := strings.ToUpper(strings.TrimSpace(matches[1]))

	// Naive linear search
	results := []string{}
	for code, name := range schoolcodeData {
		initials, err := getInitials(name)
		if err != nil {
			s.Logger.Printf("[%s] Error getting name initials on school code [%s]: %w", SchoolcodePluginName, code, err)
		}
		if strings.Contains(name, query) || initials == query {
			results = append(results, fmt.Sprintf("`%s` %s", code, name))
		}
	}

	var answerText string
	switch l := len(results); {
	case l == 0:
		answerText = fmt.Sprintf("No schools found with string %s.", query)
	case l > maxResults:
		answerText = fmt.Sprintf("There are more than %d results. Please try a more specific query.", maxResults)
	default:
		answerText = fmt.Sprintf("%s", strings.Join(results, "\n"))
	}
	return &slackscot.Answer{
		Text:    answerText,
		Options: s.answerOptions,
	}
}

func getInitials(input string) (string, error) {
	var initials strings.Builder
	for _, s := range strings.Fields(input) {
		// TODO: extract first rune and WriteRune instead
		err := initials.WriteByte(s[0])
		if err != nil {
			return "", err
		}
	}
	return initials.String(), nil
}
