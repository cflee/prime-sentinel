# prime-sentinel

A reboot of omega-sentinel, using the [alexandre-normand/slackscot](https://github.com/alexandre-normand/slackscot) Slack bot framework.

## Overview

The Slackscot framework is relatively new, and was chosen over other Slack-only and cross-platform options, for its rather unusual inbuilt support for noticing message updates (edits) and updating/deleting the bot's responses as appropriate.

> Note: If a message that was chosen by the RNG is edited to add _new_ trigger(s), a new response will be sent, which will naturally appear at the bottom of the channel or thread.
>
> If this gets really irritating, can try adjusting `maxAgeHandledMessages` to a shorter period from the default 24 hours, so that at least the message being edited and the bottom of the channel/thread won't be so far apart, and it might be less surprising to the humans.
> If it appears to be sending new responses instead of updating existing ones (e.g. editing a chosen message without adding new triggers causes new responses to be sent), might need to increase `responseCacheSize` and/or decrease `maxAgeHandledMessages`.

Internally, Slackscot uses the [nlopes/slack](https://github.com/nlopes/slack) client library to connect using a Slack App [Bot User](https://api.slack.com/bot-users) token to the WebSockets-based [Real Time Messaging API](https://api.slack.com/rtm).
Using the RTM API instead of the [Events API](https://api.slack.com/events-api) (receiving incoming webhooks from Slack based on subscriptions) is necessary to work around silly inbound IP geolocation restrictions.

## Usage/Setup

Some or all of these steps may not be necessary if you have a Slack app or bot token on hand.

1. [Create a Slack App](https://api.slack.com/apps/new).
    1. You can change the name later, but the workspace can't be modified later. The main impact of workplace choice is that if you no longer have access to this workspace, you won't be able to administer the app.
1. From the [list of your Slack Apps](https://api.slack.com/apps), select your app.
1. Either using the "Add features and functionality" box on the Basic Information page, or the **Bot Users** link on the sidebar, configure the Bot User settings inside the app.
    1. **Display name**, despite misleading explanatory text, follows the same rules as human user display names, so spaces work fine.
    1. **Always Show My Bot as Online** should be Off, so that the bot only displays online when it's connected on the RTM API. This way you know when the bot is offline for some reason, and you don't get frustrated at a non-response.
1. Select **Install App** to add it to the Slack Workspace. This is a little strange given that the app was explicitly created within a workspace, but it's probabyl necessary to explicitly authorise this App to the Workspace.
1. Collect the **Bot User OAuth Access Token** from the Install App page. This is the "bot token" you need for the config file.
1. `go run .`
1. Now you can interact with the bot over Direct Messages, or optionally add it to some channel(s).

## Configuration

### CLI flags

By default, `config.yml` in the working directory is read as a config file.
The path to config file may be overridden using the cli flag `--config <path>`.

All other options are configured using the config file or env vars.

### Config file

These config bits are actually all from Slackscot, but their README doesn't document it very thoroughly, so we repeat it here.

A sample config file is provided at `config.yml.example`.
All formats that [spf13/viper](https://github.com/spf13/viper) supports should be permitted, so JSON, TOML, YAML should be supported as well, but YAML is used by default as it supports comments.

| Config key | Description |
|---|---|
| `token` | String, Slack OAuth Access Token |
| `debug` | Boolean, enable debug level logging from Slackscot. (default `false`) |
| `responseCacheSize` | Int, number of entries permitted in response cache (default `5000`) |
| `maxAgeHandledMessage` | Int, max age of messages in seconds beyond which message updates are ignored. (default `86400` or 24 hours) |
| `userInfoCacheSize` | Int, number of entries permitted in the user info LRU cache (default `0`, which disables this cache and always fetches from Slack) |
| `timeLocation` | String, time zone location for [time.LoadLocation](https://godoc.org/time#LoadLocation) (`UTC`, `Local` or IANA Time Zone database string). This is used for the scheduler to kick off scheduled actions. (default `"Local"`) |
| `replyBehavior.threadedReplies` | Boolean, forces answers to be in a thread. (default `false`) |
| `replyBehavior.broadcastThreadedReplies` | Boolean, if answers are in a thread, forces the answer to be broadcasted to the channel. (default `false`) |
| `plugins` | Map of strings (plugin names), for plugin configuration. |

If `replyBehavior.threadedReplies` and `replyBehavior.broadcast` are set to false, plugins are permitted to _opt-in_ their answers to threaded replies and broadcasted threaded replies.
It's recommended to leave both at false.

### Env vars

Config file values may be overridden by env vars. Viper's AutomaticEnv is enabled, so uppercasing the key name should work, e.g. `DEBUG=true`.

On top of the automatic mapping, some additional env vars are mapped for convenience:

| Env var | Config key |
|---|---|
| `SLACK_TOKEN` | `token` |

## Plugins

### Quoter

Quoter listens to all incoming messages to see if they match one or more words defined in `triggers`, and sends an answer randomly picked from `responses`, with `frequency` probability.
Multiple sets of triggers-responses-frequency can be defined.

Quoter will always answer with a random response if mentioned directly with a trigger word, e.g. `@<Prime Sentinel> devops`.

Quoter uses the original timestamp of the message (`ts`) to seed the random number generator in deciding whether to respond and which response to use, so that edited messages that still contain the trigger(s) post-edit won't have the responses vanish or change unexpectedly.

Configuration is made under the plugins key in `config.yml`:

```yaml
# ...
plugins:
  quoter:
    quoteconfigs:
      - triggers:
          - "devops"
          - "devsecops"
        frequency: 0.3
        responses:
          - "DevOps is not about tools."
          - "SHIFT LEEEEFFFTTT!!"
```

For each quoteconfig, all of these are mandatory:

| Key | Description |
|---|---|
| `triggers` | List of strings, words that can trigger a response. This will be compiled into a [regexp](https://godoc.org/regexp) `(?i)\b_____\b`, where the underscores are the trigger strings, with regex metacharacters escaped away. |
| `frequency` | Float, probability that a response will be triggered, e.g. 0.3 is 30% chance, 1.0 is 100% confirm plus chop will trigger |
| `responses` | List of strings, candidate responses. Emojis, including custom emojis, can be used here. |

### Schoolcode

Schoolcode listens to commands directed at it, and tries to find schools or school codes.

```
find school 1234 - find school name for given 4-digit school code
find school <string> - find school names that either contain this string or have initials exactly matching this string, if there are at most 10 results. string must not start with digit
```

The school codes are hard-coded in the plugin code because the [School Information Service](https://sis.moe.gov.sg) doesn't seem to expose the list of schools as an API, so HTML scraping is necessary.
SIS is also missing certain categories of school codes (e.g. 61xx series for MOE Kindergarten).

```yaml
# ...
plugins:
  schoolcode:
    threadedReplies: false
```

| Key | Description |
|---|---|
| `threadedReplies` | Boolean, response will be added as a thread reply if true. (default `false`) |