package calsync

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strings"
	"time"

	"net/http"

	"github.com/atreya2011/slack-google-oauth2-test/internal/pkg/config"
	"github.com/atreya2011/slack-google-oauth2-test/internal/pkg/logger"
	"github.com/slack-go/slack"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	auth "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

// CalSync is the structure of calsync type
type CalSync struct {
	Log      *log.Logger
	OAuth2   *OAuth2
	Slack    *Slack
	accounts map[string]oauth2.Token
}

// Slack contains slack api related fields
type Slack struct {
	API           *slack.Client
	SlashCommand  *slack.SlashCommand
	SigningSecret string
}

// OAuth2 contains oauth2 related information
type OAuth2 struct {
	Config *oauth2.Config
	Token  *oauth2.Token
}

// New returns a new instance of command
func New(c *config.Config) *CalSync {
	logger, err := logger.New("calsync.txt")
	if err != nil {
		log.Fatal(err)
	}
	// create new slack.Client instance
	api := slack.New(c.SlackToken)
	accounts := make(map[string]oauth2.Token)
	return &CalSync{
		Log: logger,
		OAuth2: &OAuth2{
			Config: &oauth2.Config{},
			Token:  &oauth2.Token{},
		},
		Slack: &Slack{
			API:           api,
			SlashCommand:  &slack.SlashCommand{},
			SigningSecret: c.SlackSigningSecret,
		},
		accounts: accounts,
	}
}

// HandleSlashCommand handles the slash command post request from slack
func (cs *CalSync) HandleSlashCommand(w http.ResponseWriter, r *http.Request) {
	// check if the post request has signing secret for authentication
	verifier, err := slack.NewSecretsVerifier(r.Header, cs.Slack.SigningSecret)
	if err != nil {
		cs.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	r.Body = ioutil.NopCloser(io.TeeReader(r.Body, &verifier))

	// STEP 1. parse the slash command post message and store the paramters
	sc, err := slack.SlashCommandParse(r)
	if err != nil {
		cs.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err = verifier.Ensure(); err != nil {
		cs.Log.Println(err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// store the slash command data in struct
	cs.Slack.SlashCommand = &sc

	// STEP 2:
	if sc.Text == "connect" {
		authURL, err := cs.getAuthURL()
		if err != nil {
			cs.Log.Println(err)
		}

		msg := fmt.Sprintf("Go to this url to authorize calsync: %s", authURL)
		_, err = cs.Slack.API.PostEphemeral(
			cs.Slack.SlashCommand.ChannelID,
			cs.Slack.SlashCommand.UserID,
			slack.MsgOptionText(msg, false),
		)
		if err != nil {
			cs.Log.Println(err)
		}
	}
	if strings.Contains(sc.Text, "get") {
		if len(strings.Split(sc.Text, " ")) > 1 {
			email := strings.Split(sc.Text, " ")[1]
			token, ok := cs.accounts[fmt.Sprintf("%s:%s", sc.UserID, email)]
			if ok {
				cs.OAuth2.Token = &token
				cs.getEvents()
			} else {
				cs.Log.Println(err)
			}
		}
	}
}

// handle redirect
func (cs *CalSync) HandleRedirect(w http.ResponseWriter, r *http.Request) {
	// check if the state string returned from oauth2 endpoint is the same one we sent
	if r.FormValue("state") != "state-token" {
		log.Println("invalid auth state")
	}
	// exchange the authorization code for the token
	t, err := cs.OAuth2.Config.Exchange(context.Background(), r.FormValue("code"))
	if err != nil {
		log.Println(err)
	} else {
		cs.OAuth2.Token = t
	}
	slackURL := fmt.Sprintf(
		"https://bbsakura.slack.com/app_redirect?channel=%s",
		cs.Slack.SlashCommand.ChannelID,
	)
	http.Redirect(w, r, slackURL, http.StatusTemporaryRedirect)
	email, err := cs.getUserEmail()
	if err != nil {
		cs.Log.Println(err)
	}
	cs.accounts[fmt.Sprintf("%s:%s", cs.Slack.SlashCommand.UserID, email)] = *cs.OAuth2.Token
	_, err = cs.Slack.API.PostEphemeral(
		cs.Slack.SlashCommand.ChannelID,
		cs.Slack.SlashCommand.UserID,
		slack.MsgOptionText(fmt.Sprintf("%s: account connected!", email), false),
	)
	if err != nil {
		cs.Log.Println(err)
	}
}

// GetAuthURL returns auth url to receive consent from user
func (cs *CalSync) getAuthURL() (authURL string, err error) {
	// If modifying these scopes, delete your previously saved token.json.
	b, err := ioutil.ReadFile("credentials.json")
	if err != nil {
		err = fmt.Errorf("unable to read client secret file: %v", err)
		return
	}
	cs.OAuth2.Config, err = google.ConfigFromJSON(
		b,
		calendar.CalendarEventsReadonlyScope,
		auth.UserinfoEmailScope,
		auth.UserinfoProfileScope,
	)
	if err != nil {
		err = fmt.Errorf("unable to parse client secret file to config: %v", err)
		return
	}
	authURL = cs.OAuth2.Config.AuthCodeURL("state-token")
	return
}

func (cs *CalSync) getUserEmail() (email string, err error) {
	ctx := context.Background()
	// create new auth service
	ts := cs.OAuth2.Config.TokenSource(ctx, cs.OAuth2.Token)
	authService, err := auth.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return
	}
	// get user info
	user, err := authService.Userinfo.V2.Me.Get().Do()
	if err != nil {
		return
	}
	email = user.Email
	return
}

func (cs *CalSync) getEvents() {
	ctx := context.Background()
	ts := cs.OAuth2.Config.TokenSource(ctx, cs.OAuth2.Token)
	srv, err := calendar.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		cs.Log.Printf("unable to initialize Calendar client: %v", err)
	}
	t := time.Now().Format(time.RFC3339)
	events, err := srv.Events.List("primary").ShowDeleted(false).
		SingleEvents(true).TimeMin(t).MaxResults(10).OrderBy("startTime").Do()
	if err != nil {
		cs.Log.Fatalf("Unable to retrieve next ten of the user's events: %v", err)
	}
	var evts []string
	evts = append(evts, "Upcoming Events:")
	if len(events.Items) == 0 {
		cs.Slack.API.PostEphemeral(cs.Slack.SlashCommand.ChannelID, cs.Slack.SlashCommand.UserID, slack.MsgOptionText("No upcoming events found!", false))
	} else {
		for _, item := range events.Items {
			date := item.Start.DateTime
			if date == "" {
				date = item.Start.Date
			}
			msg := fmt.Sprintf("%v (%v)\n", item.Summary, date)
			evts = append(evts, msg)
		}
		cs.Slack.API.PostEphemeral(cs.Slack.SlashCommand.ChannelID, cs.Slack.SlashCommand.UserID, slack.MsgOptionText(strings.Join(evts, "\n"), false))
	}
}
