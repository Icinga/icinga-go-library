package source

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"strings"

	"github.com/icinga/icinga-go-library/notifications/event"
	"github.com/pkg/errors"
)

// ErrRulesOutdated implies that the rules version between Icinga DB and Icinga Notifications mismatches.
var ErrRulesOutdated = fmt.Errorf("rules version is outdated")

// basicAuthTransport is an http.RoundTripper that adds basic authentication and a User-Agent header to HTTP requests.
type basicAuthTransport struct {
	http.RoundTripper // RoundTripper is the underlying HTTP transport to use for making requests.

	Username   string
	Password   string
	ClientName string // ClientName is used to set the User-Agent header.
}

// RoundTrip adds basic authentication headers to the request and executes the HTTP request.
func (b *basicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(b.Username, b.Password)
	// As long as our round tripper is used for the client, the User-Agent header below
	// overrides any other value set by the user.
	req.Header.Set("User-Agent", b.ClientName)

	return b.RoundTripper.RoundTrip(req)
}

// Client provides a common interface to interact with the Icinga Notifications API.
// It holds the configuration for the API endpoint and the HTTP client used to make requests.
type Client struct {
	cfg Config // cfg holds base API endpoint URL and authentication details.

	client http.Client // HTTP client used for making requests to the Icinga Notifications API.

	processEventEndpoint string // ProcessEventEndpoint holds the URL for the process event endpoint.
}

// NewClient creates a new Client instance with the provided configuration.
//
// The projectName is used to set the User-Agent header in HTTP requests sent by this client and should be
// set to the name of the project using this client (e.g., "Icinga DB v1.5.0").
//
// It may return an error if the API base URL or Icinga Web 2 base URL cannot be parsed.
func NewClient(cfg Config, projectName string) (*Client, error) {
	client := &Client{
		cfg: cfg,
		client: http.Client{
			//Timeout: cfg.Timeout, // Uncomment once Timeout is (should be?) user configurable.
			Transport: &basicAuthTransport{
				RoundTripper: http.DefaultTransport,
				Username:     cfg.Username,
				Password:     cfg.Password,
				ClientName:   projectName,
			},
		},
	}

	baseUrl, err := url.Parse(cfg.ApiBaseUrl)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse API base URL")
	}

	client.processEventEndpoint = baseUrl.JoinPath("/process-event").String()

	return client, nil
}

// ProcessEvent submits an event to the Icinga Notifications /process-event API endpoint.
//
// It serializes the event into JSON and sends it as a POST request to the process event endpoint.
//
// Event.RulesVersion and Event.RuleIds must be set. When no information is available, set them to an empty string and
// an empty []int64, respectively.
//
// It may return an ErrRulesOutdated error, implying that the provided ruleVersion does not match the current rules
// version in Icinga Notifications daemon. In this case, it will also return the current rules specific to your source
// and their version, so you can retry the event submission after re-evaluating them. This way, no extra HTTP request
// is needed to fetch the rules, as Icinga Notifications will respond with the newest ones whenever it detects that
// you're using an outdated event rules config.
//
// If the request fails or the response is not as expected, it returns an error; otherwise, it returns nil.
func (c *Client) ProcessEvent(ctx context.Context, ev *event.Event) (*RulesInfo, error) {
	body, err := json.Marshal(ev)
	if err != nil {
		return nil, errors.Wrap(err, "cannot encode event to JSON")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.processEventEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, errors.Wrap(err, "cannot create HTTP request")
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	resp, err := c.client.Do(req)
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
	_, _ = io.Copy(&buf, &io.LimitedReader{R: resp.Body, N: 1 << 20}) // Limit the error message length to avoid memory exhaustion.

	return nil, errors.Errorf("unexpected response from process event API, status %q (%d): %q",
		resp.Status, resp.StatusCode, strings.TrimSpace(buf.String()))
}

// RulesInfo holds information about the event rules for a specific source.
type RulesInfo struct {
	Version string              // Version of the event rules fetched from the API.
	Rules   map[string]RuleResp // Rules is a map of rule IDs to their corresponding RuleResp objects.
}

// Iter returns an iterator over the rules in the RulesInfo.
// It yields each RuleResp object until all rules have been processed or the yield function returns false.
func (r *RulesInfo) Iter() iter.Seq[RuleResp] {
	return func(yield func(RuleResp) bool) {
		for _, rule := range r.Rules {
			if !yield(rule) {
				break
			}
		}
	}
}

// RuleResp represents a response object for a rule in the Icinga Notifications API.
type RuleResp struct {
	Id               string // Id is the unique identifier of the rule.
	Name             string // Name is the name of the rule.
	ObjectFilterExpr string // ObjectFilterExpr is the object filter expression of the rule, if any.
}
