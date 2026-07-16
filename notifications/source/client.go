package source

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/icinga/icinga-go-library/notifications"
	"github.com/icinga/icinga-go-library/notifications/event"
	"github.com/icinga/icinga-go-library/types"
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
	httpClient http.Client

	endpoints struct {
		ProcessEvent string
		Incidents    string
		Health       string
	}
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
	// Clear any query parameters from the base URL not to interfere with the filter query parameter used in requests.
	baseUrl.RawQuery = ""

	return &Client{
		httpClient: http.Client{
			Transport: &basicAuthTransport{
				RoundTripper: http.DefaultTransport,
				username:     cfg.Username,
				password:     cfg.Password,
				userAgent:    clientName,
			},
		},
		endpoints: struct {
			ProcessEvent string
			Incidents    string
			Health       string
		}{
			ProcessEvent: baseUrl.JoinPath("/process-event").String(),
			Incidents:    baseUrl.JoinPath("/incidents").String(),
			Health:       baseUrl.JoinPath("/health").String(),
		},
	}, nil
}

// ErrAttrsNegotiation implies missing attributes.
var ErrAttrsNegotiation = stderrors.New("attribute negotiation required")

// CheckHealth performs a health check against the Icinga Notifications /health API endpoint.
//
// It returns nil if the health check is successful (HTTP 200 OK), or an error if the health check
// fails or the response status code is not 200. This also includes invalid credentials, which will
// return an ErrUnauthorizedRequest error.
func (client *Client) CheckHealth(ctx context.Context) error {
	//nolint:bodyclose // False positive, drainBody is called in the defer statement below.
	resp, err := client.doRequest(ctx, http.MethodGet, client.endpoints.Health, nil, nil, nil)
	if err != nil {
		return errors.Wrap(err, "cannot GET health from API")
	}
	defer drainBody(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("unexpected response from health API, status %q (%d): %q",
			resp.Status, resp.StatusCode, readLimitedBody(resp.Body))
	}
	return nil
}

// ProcessEvent submits an event.Event to the Icinga Notifications /process-event API endpoint.
//
// When rejectIncomplete is set and the returned error is ErrAttrsNegotiation,
// the first return parameter is an array of required attributes.
func (client *Client) ProcessEvent(ctx context.Context, ev *event.Event, rejectIncomplete bool) ([]string, error) {
	body, err := json.Marshal(ev)
	if err != nil {
		return nil, errors.Wrap(err, "cannot encode event to JSON")
	}

	//nolint:bodyclose // False positive, drainBody is called in the defer statement below.
	resp, err := client.doRequest(
		ctx,
		http.MethodPost,
		client.endpoints.ProcessEvent,
		nil,
		bytes.NewReader(body),
		map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
			notifications.XIcingaRejectIfRelationsIncomplete: fmt.Sprintf("%t", rejectIncomplete),
		},
	)
	if err != nil {
		return nil, errors.Wrap(err, "cannot POST HTTP request to process event")
	}
	defer drainBody(resp.Body)

	if resp.StatusCode == http.StatusUnprocessableEntity {
		var attributeNegotiationResp struct{ Attributes []string }

		if err := json.NewDecoder(resp.Body).Decode(&attributeNegotiationResp); err != nil {
			return nil, errors.Wrap(err, "cannot decode attribute negotiation from process event response")
		}

		return attributeNegotiationResp.Attributes, ErrAttrsNegotiation
	}

	if resp.StatusCode >= http.StatusOK && resp.StatusCode <= 299 {
		return nil, nil // Successfully processed the event.
	}

	if resp.StatusCode == http.StatusNotAcceptable {
		return nil, nil // Superfluous state change event.
	}

	return nil, errors.Errorf("unexpected response from process event API, status %q (%d): %q",
		resp.Status, resp.StatusCode, readLimitedBody(resp.Body))
}

// GetIncidents retrieves the current incidents of this source from the Icinga Notifications /incidents API endpoint.
//
// The filter parameter is used to filter the incidents returned by the API. It must be a non-nil value that can
// be marshaled to JSON. If filter is nil, an error is returned. Please refer to the Icinga Notifications API doc
// for details on the filter syntax and supported fields.
func (client *Client) GetIncidents(ctx context.Context, filter any) ([]Incident, error) {
	if filter == nil {
		return nil, errors.New("filter parameter must be non-nil")
	}

	//nolint:bodyclose // False positive, drainBody is called in the defer statement below.
	resp, err := client.doRequest(ctx, http.MethodGet, client.endpoints.Incidents, filter, nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "cannot GET incidents from API")
	}
	defer drainBody(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("unexpected response from incidents API, status %q (%d): %q",
			resp.Status, resp.StatusCode, readLimitedBody(resp.Body))
	}

	var incidents []Incident
	if err := json.NewDecoder(resp.Body).Decode(&incidents); err != nil {
		return nil, errors.Wrap(err, "cannot decode incidents response")
	}
	return incidents, nil
}

// ModifyIncidents modifies the incidents that match the provided filter with the given attributes.
//
// The filter parameter is used to select which incidents to modify. It must be a non-nil value that can be
// marshaled to JSON. If filter is nil, an error is returned. Please refer to the Icinga Notifications API
// doc for details on the filter syntax and supported fields.
func (client *Client) ModifyIncidents(ctx context.Context, attrs ModifiableIncidentAttrs, filter any) error {
	if filter == nil {
		return errors.New("filter parameter must be non-nil")
	}

	if err := attrs.Validate(); err != nil {
		return err
	}

	body, err := json.Marshal(attrs)
	if err != nil {
		return errors.Wrap(err, "cannot encode modified incident attributes to JSON")
	}

	//nolint:bodyclose // False positive, drainBody is called in the defer statement below.
	resp, err := client.doRequest(
		ctx,
		http.MethodPost,
		client.endpoints.Incidents,
		filter,
		bytes.NewReader(body),
		map[string]string{
			"Content-Type": "application/json",
		},
	)
	if err != nil {
		return errors.Wrap(err, "cannot POST modified incident attributes to API")
	}
	defer drainBody(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("unexpected response from modify incidents API, status %q (%d): %q",
			resp.Status, resp.StatusCode, readLimitedBody(resp.Body))
	}
	return nil
}

// ErrUnauthorizedRequest indicates that the request was unauthorized, typically due to invalid credentials.
var ErrUnauthorizedRequest = stderrors.New("unauthorized request")

// doRequest is a helper function that performs an HTTP request to the specified endpoint with the given method, filter,
// body, and headers.
//
// The filter parameter is marshaled to JSON and added as a query parameter to the endpoint URL. If filter is nil,
// no filter is applied. The body parameter is an [io.Reader] that provides the request body. It can be nil for
// requests that do not require a body. The headers parameter is a map of additional HTTP headers to include in the
// request. It can be nil if no additional headers are needed.
//
// The function returns the HTTP response and any error encountered during the request. If the response status code
// is 401 (Unauthorized), the function returns [ErrUnauthorizedRequest] and the response body is drained and closed
// automatically. The caller is responsible for closing the response body in all other cases.
func (client *Client) doRequest(
	ctx context.Context,
	method,
	endpoint string,
	filter any,
	body io.Reader,
	headers map[string]string,
) (*http.Response, error) {
	if filter != nil {
		filterBytes, err := json.Marshal(filter)
		if err != nil {
			return nil, errors.Wrap(err, "cannot encode filter to JSON")
		}
		endpoint = fmt.Sprintf("%s?filter=%s", endpoint, url.QueryEscape(string(filterBytes)))
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create HTTP request")
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	resp, err := client.httpClient.Do(req) // #nosec G704 -- SSRF impossible, trusted user input
	if err != nil {
		return nil, errors.Wrap(err, "cannot do HTTP request")
	}

	if resp.StatusCode == http.StatusUnauthorized {
		defer drainBody(resp.Body)
		return nil, errors.Wrap(ErrUnauthorizedRequest, readLimitedBody(resp.Body))
	}
	return resp, nil
}

// Incident represents a single incident object as returned by the Icinga Notifications /incidents API endpoint.
type Incident struct {
	IsMuted    bool              `json:"is_muted"`
	ObjectTags map[string]string `json:"object_tags"`
	Severity   event.Severity    `json:"severity"`
}

// ModifiableIncidentAttrs represents the attributes of an incident that can be modified via the /incident endpoint.
//
// The Message field is a string that can be set to update the incident's message. The Close field is a boolean
// that can only be set to true to close the incident. Setting Close to false is not allowed and will result in
// a validation error.
type ModifiableIncidentAttrs struct {
	Message types.String `json:"message,omitzero"`
	Close   types.Bool   `json:"close,omitzero"`
}

// Validate checks if the ModifiableIncidentAttrs struct has valid values for its fields.
func (attrs *ModifiableIncidentAttrs) Validate() error {
	if attrs.Close.Valid && !attrs.Close.Bool {
		return errors.New("invalid value for 'close': must be true if set")
	}
	if !attrs.Close.Valid && !attrs.Message.Valid {
		return errors.New("at least one of 'message' or 'close' must be set")
	}
	return nil
}

// readLimitedBody reads from the provided [io.ReadCloser] up to 1<<16 bytes and returns it as a string.
//
// If an error occurs during reading, the error message is appended to the returned string.
func readLimitedBody(body io.ReadCloser) string {
	var buf bytes.Buffer
	_, err := io.Copy(&buf, &io.LimitedReader{R: body, N: 1 << 16}) // Limit the error message length to avoid memory exhaustion.
	if err != nil {
		buf.WriteString(" Read Error: ")
		buf.WriteString(err.Error())
	}
	return strings.TrimSpace(buf.String())
}

// drainBody reads and discards the remaining data from the provided [io.ReadCloser] and closes it.
//
// This is necessary to allow the underlying HTTP connection to be reused for future requests.
func drainBody(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, body)
	_ = body.Close()
}
