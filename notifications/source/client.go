package source

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/icinga/icinga-go-library/notifications"
	"github.com/icinga/icinga-go-library/notifications/event"
	"github.com/icinga/icinga-go-library/types"
	"github.com/pkg/errors"
)

const (
	// ResponseStatusSuccess indicates a successful response from the Icinga Notifications API.
	ResponseStatusSuccess = "success"
	// ResponseStatusError indicates an error response from the Icinga Notifications API.
	ResponseStatusError = "error"
)

var (
	// ErrAttrsNegotiation implies missing attributes.
	ErrAttrsNegotiation = stderrors.New("attribute negotiation required")

	// ErrUnauthorizedRequest indicates that the request was unauthorized, typically due to invalid credentials.
	ErrUnauthorizedRequest = stderrors.New("unauthorized request")

	// ErrReadPartialResp indicates that a partial response was received from the API, which may require special handling.
	ErrReadPartialResp = stderrors.New("read partial response")
)

// clientTransport is a http.RoundTripper to be used in NewClient.
type clientTransport struct {
	http.RoundTripper

	// userAgent for the User-Agent request header.
	userAgent string

	// username and password are sent as HTTP basic authentication if the username is not empty.
	username string
	password string
}

// RoundTrip implements http.RoundTripper.
func (ct *clientTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", ct.userAgent)
	if ct.username != "" {
		req.SetBasicAuth(ct.username, ct.password)
	}
	return ct.RoundTripper.RoundTrip(req)
}

// Client provides a common interface to interact with the Icinga Notifications API.
//
// It stores the configuration for the API endpoint and holds a reusable HTTP client for requests. To create a Client,
// the NewClient function should be used.
type Client struct {
	httpClient http.Client

	endpoints struct {
		ProcessEvent        string
		Incidents           string
		NotificationHistory string
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

	transport := http.DefaultTransport.(*http.Transport).Clone() //nolint:forcetypeassert
	switch baseUrl.Scheme {
	case schemeHttp:
		// Nothing to do here.

	case schemeHttps:
		cfg.TlsOptions.Enable = true
		tlsConfig, err := cfg.TlsOptions.MakeConfig(baseUrl.Hostname())
		if err != nil {
			return nil, errors.Wrap(err, "unable to create TLS config")
		}
		transport.TLSClientConfig = tlsConfig

	case schemeUnix:
		// Extract the socket path used for lower level connection and use a dummy HTTP URL instead.
		socketPath := baseUrl.Path
		baseUrl = &url.URL{Scheme: "http", Host: "localhost:5680"}
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			dialer := &net.Dialer{}
			return dialer.DialContext(ctx, "unix", socketPath)
		}

	default:
		return nil, errors.Errorf("unsupported notifications scheme %q", baseUrl.Scheme)
	}

	return &Client{
		httpClient: http.Client{
			Transport: &clientTransport{
				RoundTripper: transport,
				userAgent:    clientName,
				username:     cfg.Username,
				password:     cfg.Password,
			},
		},
		endpoints: struct {
			ProcessEvent        string
			Incidents           string
			NotificationHistory string
		}{
			ProcessEvent:        baseUrl.JoinPath("/process-event").String(),
			Incidents:           baseUrl.JoinPath("/incidents").String(),
			NotificationHistory: baseUrl.JoinPath("/notification-history").String(),
		},
	}, nil
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
		0,
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

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil // Successfully processed the event.
	}

	if resp.StatusCode == http.StatusUnprocessableEntity {
		var attributeNegotiationResp struct{ Attributes []string }

		if err := json.NewDecoder(resp.Body).Decode(&attributeNegotiationResp); err != nil {
			return nil, errors.Wrap(err, "cannot decode attribute negotiation from process event response")
		}

		return attributeNegotiationResp.Attributes, ErrAttrsNegotiation
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
	incidentsCh, errCh := client.YieldIncidents(ctx, filter)
	var incidents []Incident

	for incident := range incidentsCh {
		incidents = append(incidents, incident)
	}
	if err := <-errCh; err != nil {
		return nil, err
	}
	return incidents, nil
}

// YieldIncidents retrieves the current incidents of this source from the Icinga Notifications /incidents endpoint
// and yields them as a stream of [Incident] objects.
//
// The filter parameter is used to filter the incidents returned by the API. It must be a non-nil value that can
// be marshaled to JSON. If filter is nil, an error is returned. Please refer to the Icinga Notifications API doc
// for details on the filter syntax and supported fields.
//
// The function returns a channel of [Incident] objects and a channel of errors. The caller should read from both
// channels until they are closed. If an error occurs during the request or while decoding the response, it will
// be sent to the error channel. Also, this might send [ErrReadPartialResp] to the error channel when something
// went wrong midstream in Icinga Notifications, and that will be the last thing sent to the error channel, after
// which no more incidents will be sent to the incident channel.
//
// If you want to collect all incidents into a slice, consider using [Client.GetIncidents] instead, which internally
// uses this function and collects the incidents for you.
func (client *Client) YieldIncidents(ctx context.Context, filter any) (<-chan Incident, <-chan error) {
	return client.yield[Incident](ctx, client.endpoints.Incidents, filter, 0, false)
}

// ModifyIncidents modifies the incidents that match the provided filter with the given attributes.
//
// The filter parameter is used to select which incidents to modify. It must be a non-nil value that can be
// marshaled to JSON. If filter is nil, an error is returned. Please refer to the Icinga Notifications API
// doc for details on the filter syntax and supported fields.
//
// If some matching incidents couldn't be modified, the function returns a [ModifyError] containing the details
// of the incidents that failed to be modified. The caller can inspect that error to determine and act up the
// erroneous incidents if needed.
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
		0,
		bytes.NewReader(body),
		map[string]string{
			"Content-Type": "application/json",
		},
	)
	if err != nil {
		return errors.Wrap(err, "cannot POST modified incident attributes to API")
	}
	defer drainBody(resp.Body)

	if resp.StatusCode != http.StatusAccepted {
		return errors.Errorf("unexpected response from modify incidents API, status %q (%d): %q",
			resp.Status, resp.StatusCode, readLimitedBody(resp.Body))
	}

	var results []ModifiedIncidentResp
	for dec := json.NewDecoder(resp.Body); dec.More(); {
		var modifiedResp Response[ModifiedIncidentResp]
		if err := dec.Decode(&modifiedResp); err != nil {
			return errors.Wrap(err, "cannot decode modified incident response from API")
		}
		if modifiedResp.Status != ResponseStatusSuccess {
			results = append(results, modifiedResp.Result)
		}
	}
	if len(results) > 0 {
		return errors.WithStack(&ModifyError{results: results})
	}
	return nil
}

// GetNotificationHistory retrieves the notification history from the API for the specified time range.
//
// The since parameter is a Unix timestamp in milliseconds that specifies the start of the time range for which
// to retrieve notification history entries. It must be a positive integer. If since is less than or equal to zero,
// an error is returned.
func (client *Client) GetNotificationHistory(ctx context.Context, filter any, since int64) ([]NotificationHistory, error) {
	notificationHistoryCh, errCh := client.YieldNotificationHistory(ctx, filter, since)

	var notificationHistoryEntries []NotificationHistory
	for entry := range notificationHistoryCh {
		notificationHistoryEntries = append(notificationHistoryEntries, entry)
	}
	if err := <-errCh; err != nil {
		return nil, err
	}

	return notificationHistoryEntries, nil
}

// YieldNotificationHistory retrieves the notification history from the API for the specified time range and yields
// them as a stream of [NotificationHistory] objects.
//
// The since parameter is a Unix timestamp in milliseconds that specifies the start of the time range for which
// to retrieve notification history entries. It must be a positive integer. If since is less than or equal to zero,
// an error is returned.
//
// The function returns a channel of [NotificationHistory] objects and a channel of errors. The caller should read
// from both channels until they are closed. If an error occurs during the request or while decoding the response,
// it will be sent to the error channel. Also, this might send [ErrReadPartialResp] to the error channel when it
// receives a notification history entry with a non-zero Error field from the API, which indicates that something
// went wrong after streaming the first chunk of the response body.
//
// If you want to collect all notification history entries into a slice, consider using [Client.GetNotificationHistory]
// instead, which internally uses this function and collects the entries for you.
func (client *Client) YieldNotificationHistory(ctx context.Context, filter any, since int64) (<-chan NotificationHistory, <-chan error) {
	return client.yield[NotificationHistory](ctx, client.endpoints.NotificationHistory, filter, since, true)
}

// yield is a generic helper function that retrieves data from the specified API endpoint and yields it as a stream of
// type T objects.
//
// The filter parameter is used to filter the data returned by the API. It must be a non-nil value that can
// be marshaled to JSON. If filter is nil, an error is returned. Please refer to the Icinga Notifications API doc
// for details on the filter syntax and supported fields.
//
// The function returns a channel of type T objects and a channel of errors. The caller should read from both
// channels until they are closed. If an error occurs during the request or while decoding the response, it will
// be sent to the error channel.
func (client *Client) yield[T any](ctx context.Context, endpoint string, filter any, since int64, requireSince bool) (<-chan T, <-chan error) {
	ch := make(chan T)
	errCh := make(chan error, 1)

	go func() {
		defer close(ch)
		defer close(errCh)

		if requireSince && since <= 0 {
			errCh <- errors.New("since parameter must be a positive integer")
			return
		}

		//nolint:bodyclose // False positive, drainBody is called in the defer statement below.
		resp, err := client.doRequest(ctx, http.MethodGet, endpoint, filter, since, nil, nil)
		if err != nil {
			errCh <- errors.Wrapf(err, "cannot GET data from API endpoint %q", endpoint)
			return
		}
		defer drainBody(resp.Body)

		if resp.StatusCode != http.StatusAccepted {
			errCh <- errors.Errorf("unexpected response from API %q, status %q (%d): %q",
				endpoint, resp.Status, resp.StatusCode, readLimitedBody(resp.Body))
			return
		}

		for dec := json.NewDecoder(resp.Body); dec.More(); {
			if !decodeAndDispatchResponseItem(ctx, dec, ch, errCh) {
				return // An error occurred, and it has been sent to errCh, so we stop processing further.
			}
		}
	}()

	return ch, errCh
}

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
	since int64,
	body io.Reader,
	headers map[string]string,
) (*http.Response, error) {
	if since > 0 {
		endpoint = fmt.Sprintf("%s?since=%d", endpoint, since)
	}
	if filter != nil {
		filterBytes, err := json.Marshal(filter)
		if err != nil {
			return nil, errors.Wrap(err, "cannot encode filter to JSON")
		}
		separator := "?"
		if since > 0 {
			separator = "&"
		}
		endpoint = fmt.Sprintf("%s%sfilter=%s", endpoint, separator, url.QueryEscape(string(filterBytes)))
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

// ErrorState represents an error state returned by the Icinga Notifications API as a Response.Result
// when the Response.Status is "error".
type ErrorState struct {
	Error string `json:"error,omitempty"`
}

// Incident represents an incident as returned by the Icinga Notifications /incidents API endpoint as a Response.Result
// when the Response.Status is "success".
type Incident struct {
	IsMuted    bool              `json:"is_muted"`
	ObjectTags map[string]string `json:"object_tags,omitempty"`
	Severity   event.Severity    `json:"severity,omitempty"`
}

// Response represents a generic response from the Icinga Notifications API.
//
// The Status field indicates whether the request was successful or resulted in an error. The Result field contains
// the actual response data, which can be of any type T. If the request was successful, Result will contain the
// expected data, otherwise, contains the error state information.
type Response[T any] struct {
	Status string `json:"status"` // success or error
	Result T      `json:"result"`
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

// ModifiedIncidentResp represents the response for a modified incident from the Icinga Notifications API as a
// Response.Result in both success and error cases. However, if Response.Status is "error", the ErrorState field
// will be populated with the error details, otherwise, it's omitted entirely.
//
// It contains the ObjectTags of the modified incident.
type ModifiedIncidentResp struct {
	ObjectTags map[string]string `json:"object_tags,omitempty"`
	ErrorState
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

// decodeAndDispatchResponseItem decodes a single response item from the JSON decoder and sends it to the provided channel.
//
// If the response item indicates success, it decodes the result into the specified type T and sends it to the ch
// channel, otherwise it decodes the error state and sends an error to the errCh channel. If the context is canceled
// before sending the result, it sends the context error to the errCh channel instead.
//
// The function returns true if the result was successfully sent to the ch channel, and false if an error was sent to
// the errCh channel.
func decodeAndDispatchResponseItem[T any](ctx context.Context, dec *json.Decoder, ch chan<- T, errCh chan<- error) bool {
	var respRes Response[json.RawMessage]
	if err := dec.Decode(&respRes); err != nil {
		errCh <- errors.Wrap(err, "cannot decode response from API")
		return false
	}

	switch respRes.Status {
	case ResponseStatusSuccess:
		var resultT T
		if err := json.Unmarshal(respRes.Result, &resultT); err != nil {
			errCh <- errors.Wrap(err, "cannot decode result from API")
			return false
		}

		select {
		case ch <- resultT:
			return true

		case <-ctx.Done():
			errCh <- ctx.Err()
			return false
		}

	default:
		var errState ErrorState
		if err := json.Unmarshal(respRes.Result, &errState); err != nil {
			errCh <- errors.Wrap(err, "cannot decode error state from API")
			return false
		}
		errCh <- errors.Wrap(ErrReadPartialResp, errState.Error)
		return false
	}
}

// ModifyError represents an error that occurred while modifying incidents via the /incident endpoint.
type ModifyError struct {
	results []ModifiedIncidentResp
}

// Error implements the error interface for ModifyError.
func (e *ModifyError) Error() string {
	return fmt.Sprintf("failed to modify %d incidents", len(e.results))
}

// Results returns the slice of [ModifiedIncidentResp] that contains the details of the incidents that failed to be modified.
func (e *ModifyError) Results() []ModifiedIncidentResp { return e.results }
