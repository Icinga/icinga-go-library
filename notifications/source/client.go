package source

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/icinga/icinga-go-library/notifications/event"
	"github.com/pkg/errors"
)

// basicAuthTransport is an http.RoundTripper that adds basic authentication and a User-Agent header to HTTP requests.
type basicAuthTransport struct {
	http.RoundTripper // RoundTripper is the underlying HTTP transport to use for making requests.

	// username and password are set as HTTP basic authentication.
	username string
	password string
	// userAgent is used to set the User-Agent header.
	userAgent string
}

// RoundTrip adds basic authentication headers to the request and executes the HTTP request.
func (b *basicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(b.username, b.password)
	// As long as our round tripper is used for the client, the User-Agent header below
	// overrides any other value set by the user.
	req.Header.Set("User-Agent", b.userAgent)

	return b.RoundTripper.RoundTrip(req)
}

// Client provides a common interface to interact with the Icinga Notifications API.
//
// It stores the configuration for the API endpoint and holds a reusable HTTP client for requests. To create a Client,
// the NewClient function should be used.
type Client struct {
	httpClient           http.Client
	processEventEndpoint string
}

// NewClient creates a new Client instance with the provided configuration.
//
// The clientName argument is used as the User-Agent header in HTTP requests sent by this Client and should represent
// the project using this client, e.g., "Icinga DB v1.5.0".
//
// It may return an error if the API base URL cannot be parsed.
func NewClient(cfg Config, clientName string) (*Client, error) {
	baseUrl, err := url.Parse(cfg.Url)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse API base URL")
	}

	processEventEndpoint := baseUrl.JoinPath("/process-event").String()

	return &Client{
		httpClient: http.Client{
			Transport: &basicAuthTransport{
				RoundTripper: http.DefaultTransport,
				username:     cfg.Username,
				password:     cfg.Password,
				userAgent:    clientName,
			},
		},
		processEventEndpoint: processEventEndpoint,
	}, nil
}

// RulesInfo holds information about the event rules for a specific source.
//
// A Client can fetch RulesInfo from the Icinga Notifications API via Client.ProcessEvent.
//
// The Version represents the current rules version for this Client. This value should be stored by a caller and set to
// [event.Event.EventVersion] when being passed to Client.ProcessEvent, as described there. This allows detecting
// outdated source rules.
//
// Rules is a map of rule IDs to object filter expressions. These object filter expressions are source-specific. For
// example, Icinga DB expects an SQL query.
type RulesInfo struct {
	// Version of the event rules fetched from the API.
	Version string

	// Rules is a map of rule IDs to their corresponding object filter expression.
	Rules map[string]string
}

// ErrRulesOutdated implies that the rules version between the submitted event and Icinga Notifications mismatches.
var ErrRulesOutdated = stderrors.New("rules version is outdated")

// ProcessEvent submits an event.Event to the Icinga Notifications /process-event API endpoint.
//
// For a successful submission, this method returns (nil, nil).
//
// It may return an ErrRulesOutdated error, implying that the provided ruleVersion does not match the current rules
// version in the Icinga Notifications daemon. Only in this case, it will also return the current rules specific to this
// source and their version as RulesInfo, allowing to retry submitting an event after reevaluating it.
//
// For the Event to be submitted, Event.RulesVersion and Event.RuleIds should be set. If no appropriate values are
// known, they can be left empty. This will return ErrRulesOutdated - unless there are no rules configured that need to
// be evaluated by the source -, which can be handled as described above to retrieve information and resubmit the event.
//
// If the request fails or the response is not as expected, it returns an error.
func (client *Client) ProcessEvent(ctx context.Context, ev *event.Event) (*RulesInfo, error) {
	body, err := json.Marshal(ev)
	if err != nil {
		return nil, errors.Wrap(err, "cannot encode event to JSON")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.processEventEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, errors.Wrap(err, "cannot create HTTP request")
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	resp, err := client.httpClient.Do(req) // #nosec G704 -- SSRF impossible, trusted user input
	if err != nil {
		return nil, errors.Wrap(err, "cannot POST HTTP request to process event")
	}

	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusPreconditionFailed {
		// Indicates that the rules version is outdated and the body should contain the current rules and their version.
		// So, we read the body to extract the rules and return an ErrRulesOutdated error, so the caller can retry
		// the event submission after it has reevaluated them.
		var rulesInfo RulesInfo
		if err := json.NewDecoder(resp.Body).Decode(&rulesInfo); err != nil {
			return nil, errors.Wrap(err, "cannot decode new rules from process event response")
		}

		return &rulesInfo, ErrRulesOutdated
	}

	if resp.StatusCode >= http.StatusOK && resp.StatusCode <= 299 {
		return nil, nil // Successfully processed the event.
	}

	if resp.StatusCode == http.StatusNotAcceptable {
		return nil, nil // Superfluous state change event.
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, &io.LimitedReader{R: resp.Body, N: 1 << 16}) // Limit the error message length to avoid memory exhaustion.

	return nil, errors.Errorf("unexpected response from process event API, status %q (%d): %q",
		resp.Status, resp.StatusCode, strings.TrimSpace(buf.String()))
}
