package plugins

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"

	"github.com/alexandre-normand/slackscot"
	"github.com/alexandre-normand/slackscot/actions"
	"github.com/alexandre-normand/slackscot/config"
	"github.com/alexandre-normand/slackscot/plugin"
)

const (
	// QuoterPluginName is the name of the quoter plugin
	QuoterPluginName = "quoter"
)

// Quoter holds plugin data for the quoter plugin
type Quoter struct {
	*slackscot.Plugin
	quoteConfigs   []quoteConfig
	triggerRegexps map[string]*regexp.Regexp
}

// quoteConfig contains config for a set of quotes
type quoteConfig struct {
	Triggers  []string
	Frequency float64
	Responses []string
}

// NewQuoter creates a new instance of the plugin
func NewQuoter(config *config.PluginConfig) (*slackscot.Plugin, error) {
	// Viper requires sub-trees to be maps, so a map-like struct is required
	// for unmarshalling the PluginConfig. (see spf13/viper#171)
	qConfig := struct {
		QuoteConfigs []quoteConfig
	}{}
	err := config.Unmarshal(&qConfig)
	if err != nil {
		return nil, fmt.Errorf("[%s] Unable to unmarshal config: %w", QuoterPluginName, err)
	}

	// TODO: perform more validation
	// Ensure responses is not empty, otherwise trying to get a random int32
	// in the range 0 to 0 will panic. Having missing triggers or frequency is
	// still ok, just can't be triggered.
	for _, c := range qConfig.QuoteConfigs {
		if len(c.Responses) == 0 {
			return nil, fmt.Errorf("[%s] Error loading QuoteConfig, missing or 0-length responses", QuoterPluginName)
		}
	}

	q := new(Quoter)
	q.quoteConfigs = qConfig.QuoteConfigs
	q.triggerRegexps = make(map[string]*regexp.Regexp)

	pluginBuilder := plugin.New(QuoterPluginName)

	for _, c := range q.quoteConfigs {
		// Hear Action for ambient conversation
		pluginBuilder = pluginBuilder.WithHearAction(actions.NewHearAction().
			Hidden().
			WithMatcher(q.matcher(c.Triggers, c.Frequency)).
			WithAnswerer(q.answerer(c.Responses)).
			Build())

		// Command for bot mentions and DMs
		// Slackscot does not route direct mentions and direct messages to
		// Hear Actions, so creating a parallel Command is necessary.
		// Matcher is configured to have frequency of 1, i.e. 100% chance.
		pluginBuilder = pluginBuilder.WithCommand(actions.NewCommand().
			WithMatcher(q.matcher(c.Triggers, 1)).
			WithUsage(strings.Join(c.Triggers, "|")).
			WithDescription("Get a quotable quote").
			WithAnswerer(q.answerer(c.Responses)).
			Build())
	}

	q.Plugin = pluginBuilder.Build()
	return q.Plugin, nil
}

// matcher makes a Matcher, that decides whether to send an answer
func (q *Quoter) matcher(triggers []string, frequency float64) func(m *slackscot.IncomingMessage) bool {
	return func(m *slackscot.IncomingMessage) bool {
		// Ignore bot messages to prevent infinite loops from this bot
		// hearing its own help command output. In theory the slackscot
		// message routing should already ignore messages from its own userID
		// but somehow I've observed it getting stuck in a loop on DMs.
		user, err := q.UserInfoFinder.GetUserInfo(m.User)
		if err != nil {
			q.Logger.Printf("[%s] Error getting user info for user [%s]: %w\n", QuoterPluginName, m.User, err)
		}
		if user.IsBot {
			return false
		}

		// Check if at least one of the triggers has been hit
		found := false
		for _, trigger := range triggers {
			exp, err := q.getTriggerRegexp(trigger)
			if err != nil {
				q.Logger.Printf("[%s] Error getting regexp for trigger [%s]: %w\n", QuoterPluginName, trigger, err)
			}

			if exp.MatchString(m.NormalizedText) {
				found = true
			}
		}
		if !found {
			return false
		}

		// For Commands, frequency is set to 1 as the response is always
		// expected. In that case, don't bother with randomGen.
		if frequency >= 0.99 {
			return true
		}

		// Use the message's timestamp to seed rand, to get consistent decision
		// when messages are edited. This timestamp is stable across edits.
		ts, err := parseFloatTimestampStringToInt(m.Timestamp)
		if err != nil {
			q.Logger.Printf("[%s] Skipping message [%v] due to error converting timestamp string to int: %w\n", QuoterPluginName, m, err)
		}
		randomGen := rand.New(rand.NewSource(ts))

		return randomGen.Float64() < frequency
	}
}

// answerer makes an Answerer, that picks an answer
func (q *Quoter) answerer(responses []string) func(m *slackscot.IncomingMessage) *slackscot.Answer {
	return func(m *slackscot.IncomingMessage) *slackscot.Answer {
		// Use the message's timestamp to seed rand, to get consistent decision
		// when messages are edited. This timestamp is stable across edits.
		ts, err := parseFloatTimestampStringToInt(m.Timestamp)
		if err != nil {
			q.Logger.Printf("[%s] Skipping message [%v] due to error converting timestamp string to int: %w\n", QuoterPluginName, m, err)
		}
		randomGen := rand.New(rand.NewSource(ts))

		i := randomGen.Int31n(int32(len(responses)))
		return &slackscot.Answer{
			Text: fmt.Sprintf("%s", responses[i]),
		}
	}
}

// getTriggerRegexp returns an existing Regexp if cached, and compiles it
// and stores it otherwise.
func (q *Quoter) getTriggerRegexp(trigger string) (*regexp.Regexp, error) {
	if exp, ok := q.triggerRegexps[trigger]; ok {
		return exp, nil
	}

	// Make a case-insensitive match with word boundaries
	exp, err := regexp.Compile(fmt.Sprintf("(?i)\\b%s\\b", regexp.QuoteMeta(trigger)))
	if err != nil {
		return nil, err
	}
	q.triggerRegexps[trigger] = exp
	return exp, nil
}

func parseFloatTimestampStringToInt(timestamp string) (int64, error) {
	ts, err := strconv.ParseFloat(timestamp, 64)
	if err != nil {
		return 0, err
	}
	return int64(ts * 1000000), nil
}
