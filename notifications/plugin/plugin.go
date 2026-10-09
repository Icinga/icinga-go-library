// Package plugin implements a high-level Icinga Notifications Channel Plugin API.
//
// For own plugins, the [Plugin] interface must be implemented. After configuring your plugin, calling [Run] starts
// a blocking JSON-RPC server that handles incoming requests from the Icinga Notifications process. The plugin can
// also implement the [RPCEndpointReceiver] interface to receive the internally used RPC endpoint, allowing the plugin
// to make RPC calls back to the Icinga Notifications process.
//
// Examples can be found under [cmd/channels] in the Icinga Notifications repository.
//
// [cmd/channels]: https://github.com/Icinga/icinga-notifications/tree/main/cmd/channels
package plugin

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/icinga/icinga-go-library/notifications/event"
	"github.com/icinga/icinga-go-library/notifications/jsonrpc"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-go-library/utils"
)

const (
	MethodGetInfo          = "GetInfo"
	MethodSetConfig        = "SetConfig"
	MethodSendNotification = "SendNotification"
)

// ChildOption describes a config element that is conditionally displayed based on the value of a parent option.
//
// This allows to create a more dynamic and user-friendly configuration interface, where certain options are only
// shown when relevant based on the user's previous selections. Even though the API allows for nested children, the
// UI will only render one level of children or even reject them entirely. Therefore, it is prohibited to define a
// child option with its own children.
type ChildOption struct {
	ConfigOptionCommon

	// ParentValues is a list of parent element values that will trigger the display of this child option.
	//
	// If the parent element is a select element, this list should contain the keys of the parent option that
	// should trigger this child option to be displayed. If the parent element is a checkbox, this list should
	// contain the boolean value (true or false). All other parent element types are not supported for child options
	// and will be rejected by Icinga Notifications Web.
	//
	// If empty, the child option will always be displayed regardless of the parent option's value. This is useful for
	// child options that should always be displayed, but are still logically grouped under a parent element.
	ParentValues []any `json:"parent_values"`
}

// ConfigOptionCommon describes common fields.
type ConfigOptionCommon struct {
	// Element name
	Name string `json:"name"`

	// Element type:
	//
	//  string = text, number = number, bool = checkbox, text = textarea, option = select, options = select[multiple], secret = password
	Type string `json:"type"`

	// Element label map. Locale in the standard format (language_REGION) as key and corresponding label as value.
	// Locale is assumed to be UTF-8 encoded (Without the suffix in the locale)
	//
	//  e.g. {"en_US": "Save", "de_DE": "Speichern"}
	//  An "en_US" locale must be given as a fallback
	Label map[string]string `json:"label"`

	// Element description map. Locale in the standard format (language_REGION) as key and corresponding label as value.
	// Locale is assumed to be UTF-8 encoded (Without the suffix in the locale)
	//
	// When the user moves the mouse pointer over an element in the web UI, a tooltip is displayed with a given message.
	//
	//  e.g. {"en_US": "HTTP request method for the request.", "de_DE": "HTTP-Methode für die Anfrage."}
	//  An "en_US" locale must be given as a fallback
	Help map[string]string `json:"help,omitempty"`

	// Element default: bool for checkbox default value, string for other elements (used as placeholder)
	Default any `json:"default,omitempty"`

	// Set true if this element is required, omit otherwise
	Required bool `json:"required,omitempty"`

	// Options of a select element: key => value.
	// Only required for the type option or options
	//
	//  e.g., map[string]string{
	//			"1":   "January",
	//			"2":  "February",
	//		}
	Options map[string]string `json:"options,omitempty"`

	// Element's min option defines the minimum allowed number value. It can only be used for the type number.
	Min types.Int `json:"min"`

	// Element's max option defines the maximum allowed number value. It can only be used for the type number.
	Max types.Int `json:"max"`
}

// ConfigOption describes a config element.
type ConfigOption struct {
	ConfigOptionCommon

	// Children contains a set of [ChildOption] rendered in the UI only when a specific parent option value is selected.
	//
	// This allows for conditional configuration options based on the user's selection in the parent option.
	// For example, if a parent option is a select element with multiple choices, each choice can have its own
	// set of child options that are displayed only when that choice is selected. A concrete example would be a
	// parent option for selecting HTTP request method (GET, POST, PUT, DELETE), and each method could have its
	// own set of child options for configuring headers, body content, etc.
	//
	// A child option must not have its own children, as this would create a recursive structure that is not supported.
	Children []ChildOption `json:"children,omitempty"`
}

// ConfigOptions describes all ConfigOption entries.
//
// This type became necessary to implement the database.sql.driver.Valuer to marshal it into JSON.
type ConfigOptions []ConfigOption

// Value implements database.sql's driver.Valuer to represent all ConfigOptions as a JSON array.
func (c ConfigOptions) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Info contains channel plugin information.
type Info struct {
	// Type of the channel plugin.
	//
	// Not part of the JSON object. Will be set to the channel plugin file name before database insertion.
	Type string `db:"type" json:"-"`

	// Name of this channel plugin in a human-readable value.
	Name string `db:"name" json:"name"`

	// Version of this channel plugin.
	Version string `db:"version" json:"version"`

	// Author of this channel plugin.
	Author string `db:"author" json:"author"`

	// ConfigAttributes contains multiple ConfigOption(s) as JSON-encoded list.
	ConfigAttributes ConfigOptions `db:"config_attrs" json:"config_attrs"`
}

// TableName implements the contracts.TableNamer interface.
func (i *Info) TableName() string {
	return "available_channel_type"
}

// Contact to receive notifications for the NotificationRequest.
type Contact struct {
	// FullName of a Contact as defined in Icinga Notifications.
	FullName string `json:"full_name"`

	// Addresses of a Contact with a type.
	Addresses []*Address `json:"addresses"`
}

// Address to receive this notification. Each Contact might have multiple addresses.
type Address struct {
	// Type field matches the Info.Type, effectively being the channel plugin file name.
	Type string `json:"type"`

	// Address is the associated Type-specific address, e.g., an email address for type email.
	Address string `json:"address"`
}

// Object which this NotificationRequest is all about, e.g., an Icinga 2 Host or Service object.
type Object struct {
	// Name depending on its source, may be "host!service" when from Icinga 2.
	Name string `json:"name"`

	// Url pointing to this Object, may be to Icinga Web.
	Url string `json:"url"`

	// Tags defining this Object, may be "host" and "service" when from Icinga 2.
	Tags map[string]string `json:"tags"`
}

// Incident of this NotificationRequest, grouping Events for this Object.
type Incident struct {
	// Id is the unique identifier for this Icinga Notifications Incident, allows linking related events.
	Id int64 `json:"id"`

	// Severity of this Incident.
	Severity event.Severity `json:"severity"`

	// IsMuted indicates whether this Incident is being muted with the ongoing event.
	IsMuted bool `json:"muted"`

	// IsRecovered indicates whether this Incident is going to be resolved/recovered with the ongoing event.
	IsRecovered bool `json:"recovered"`
}

// Event indicating this NotificationRequest.
type Event struct {
	// Time when this event occurred, being encoded according to RFC 3339 when passed as JSON.
	Time time.Time `json:"time"`

	// Message of this event, might be a check output when the related Object is an Icinga 2 object.
	Message string `json:"message"`
}

// State is a plugin-defined value that Icinga Notifications stores per contact and incident.
type State struct {
	// Key identifies this state entry.
	//
	// It is a UUID generated by Icinga Notifications and must not be changed by the plugin.
	Key types.UUID `db:"state_key" json:"state_key"`

	// Value is any string of up to 4096 characters that the plugin needs in later notifications,
	// e.g., the ID of a ticket created in an external system.
	Value string `db:"value" json:"value"`
}

// IsZero reports whether s has no Key.
//
// It implements the json.isZeroer interface used by the omitzero option.
func (s State) IsZero() bool { return s.Key.IsZero() }

// NotificationRequest is sent to a channel plugin via [Plugin.SendNotification] to request a notification delivery.
type NotificationRequest struct {
	// Contact to receive this NotificationRequest.
	Contact *Contact `json:"contact"`

	// Object associated with this NotificationRequest, e.g., an Icinga 2 Service Object.
	Object *Object `json:"object"`

	// Incident associated with this NotificationRequest.
	//
	// May be nil when sending non-state notifications and no active incident exists for the given Object.
	Incident *Incident `json:"incident"`

	// Event being responsible for creating this NotificationRequest, e.g., a firing Icinga 2 Service Check.
	Event *Event `json:"event"`

	// State is the plugin's state for this request's contact and incident.
	//
	// If no state has been stored yet, [State.Key] is filled in and [State.Value] is empty.
	// The only way to change the state is to return it in [DeliveryResult.State].
	//
	// Icinga Notifications deletes the state itself once the incident is resolved or the channel is deleted.
	State State `json:"state,omitzero"`
}

// DeliveryResult is the outcome of [Plugin.SendNotification] reported back to Icinga Notifications.
type DeliveryResult struct {
	// If State has a valid Key, it replaces the stored state of the [NotificationRequest].
	//
	// Its Key must match [NotificationRequest.State.Key]. A zero State leaves the stored state unchanged.
	State State `json:"state,omitzero"`

	// Details are shown in the notification history of Icinga Notifications Web in the given order,
	// e.g., a link to the ticket created in an external system.
	//
	// The total size of the details must not exceed 1 MiB. If it does, all details are dropped
	// and not stored in the database.
	Details []DeliveryDetail `json:"details,omitempty"`
}

// DeliveryDetail is a single piece of information about a delivery, shown in Icinga Notifications Web's history view.
type DeliveryDetail struct {
	// Label describes the detail, e.g., "RT Ticket".
	Label string `json:"label"`

	// Text is the text to display, e.g., "#123".
	Text string `json:"text"`

	// URL, if set, renders Text as a hyperlink to it, e.g., "https://rt.example.com/Ticket/Display.html?id=123".
	//
	// It must be an absolute http or https URL. Otherwise, Text is rendered as plain text.
	URL string `json:"url,omitempty"`
}

// MarshalJSON implements the json.Marshaler interface to marshal the DeliveryDetail as a JSON object.
func (d *DeliveryDetail) MarshalJSON() ([]byte, error) {
	if d.Label == "" {
		return nil, fmt.Errorf("label must not be empty")
	}
	if d.Text == "" {
		return nil, fmt.Errorf("text must not be empty")
	}
	type alias DeliveryDetail
	return json.Marshal((*alias)(d))
}

// Plugin defines necessary methods for a channel plugin.
//
// Those methods are being called by the Icinga Notifications process via JSON-RPC. The plugin must implement this
// interface to be able to receive and handle those requests. The plugin can be launched via the [Run] function,
// which will start a JSON-RPC server and block until the plugin is terminated.
type Plugin interface {
	// GetInfo returns the corresponding plugin *Info.
	GetInfo() *Info

	// SetConfig sets the plugin config, returns an error on failure.
	SetConfig(jsonStr json.RawMessage) error

	// SendNotification delivers the notification described by req to its [Contact].
	//
	// A non-nil error means the delivery failed. When the plugin is started via [Run], the error is sent
	// back as a JSON-RPC error response and any [DeliveryResult] returned with it is dropped. On success,
	// the returned [DeliveryResult] may be nil if there is no state update and no details to report.
	SendNotification(req *NotificationRequest) (*DeliveryResult, error)
}

// RPCEndpointReceiver is an interface that can be implemented by a plugin to receive the RPC endpoint used internally.
//
// If a channel plugin implements this interface, the [Run] function will inject the RPC endpoint into the plugin
// before starting the server. This allows the plugin to make RPC calls back to the Icinga Notifications process.
type RPCEndpointReceiver interface {
	ReceiveEndpoint(ctx context.Context, ep *jsonrpc.Endpoint)
}

// PopulateDefaults sets the struct fields from Info.ConfigAttributes where ConfigOption.Default is set.
//
// It should be called from each channel plugin within its Plugin.SetConfig before doing any further configuration.
func PopulateDefaults(typePtr Plugin) error {
	defaults := make(map[string]any)
	for _, confAttr := range typePtr.GetInfo().ConfigAttributes {
		if confAttr.Default != nil {
			defaults[confAttr.Name] = confAttr.Default
		}
	}

	defaultConf, err := json.Marshal(defaults)
	if err != nil {
		return err
	}

	return json.Unmarshal(defaultConf, typePtr)
}

// rpcHandler is a JSON-RPC handler for a channel plugin (on the server/plugin side).
//
// It is used by plugins to handle incoming JSON-RPC requests and dispatch them to the appropriate methods
// of the Plugin interface (see [Run]).
type rpcHandler struct {
	p Plugin
}

func (rh *rpcHandler) Handle(ctx context.Context, conn *jsonrpc.Conn, req *jsonrpc.Request) {
	switch req.Method {
	case MethodGetInfo:
		if err := conn.Reply(ctx, req.ID, rh.p.GetInfo()); err != nil {
			log.Fatalf("failed to send GetInfo response: %v", err)
		}

	case MethodSetConfig:
		if req.Params == nil {
			if err := jsonrpc.ReplyMissingParams(ctx, conn, req.ID); err != nil {
				log.Fatalf("failed to send SetConfig response: %v", err)
			}
			return
		}

		if err := rh.p.SetConfig(*req.Params); err != nil {
			if err := jsonrpc.ReplyError(ctx, conn, req.ID, jsonrpc.CodeInvalidParams, err.Error()); err != nil {
				log.Fatalf("failed to send SetConfig response: %v", err)
			}
			return
		}
		if err := conn.Reply(ctx, req.ID, nil); err != nil {
			log.Fatalf("failed to send SetConfig response: %v", err)
		}

	case MethodSendNotification:
		if req.Params == nil {
			if err := jsonrpc.ReplyMissingParams(ctx, conn, req.ID); err != nil {
				log.Fatalf("failed to send SendNotification response: %v", err)
			}
			return
		}

		var nr NotificationRequest
		if err := json.Unmarshal(*req.Params, &nr); err != nil {
			if err := jsonrpc.ReplyError(ctx, conn, req.ID, jsonrpc.CodeParseError, err.Error()); err != nil {
				log.Fatalf("failed to send SendNotification response: %v", err)
			}
			return
		}

		result, err := rh.p.SendNotification(&nr)
		if err != nil {
			if err := jsonrpc.ReplyError(ctx, conn, req.ID, jsonrpc.CodeInternalError, err.Error()); err != nil {
				log.Fatalf("failed to send SendNotification response: %v", err)
			}
			return
		}
		if err := conn.Reply(ctx, req.ID, result); err != nil {
			log.Fatalf("failed to send SendNotification response: %v", err)
		}

	default:
		if err := jsonrpc.ReplyMethodNotFound(ctx, conn, req.ID); err != nil {
			log.Fatalf("failed to send error response: %v", err)
		}
	}
}

// Run starts the RPC server for a channel plugin.
//
// The RPC server reads requests from stdin, calls the associated RPC method, and writes the responses to stdout.
// It blocks until either the process receives a termination signal (SIGINT or SIGTERM) or the RPC connection is
// closed. If the plugin implements the [RPCEndpointReceiver] interface, the internally used RPC endpoint will be
// injected into the plugin, allowing the plugin to make RPC calls back to the Icinga Notifications process.
//
// Note that this function might also terminate the process unexpectedly if the rpcHandler fails to send a response
// back to Icinga Notifications. This is done to ensure that the plugin does not continue running in an inconsistent
// state after a failed RPC response.
//
// This function should be called last in a channel plugin's main function.
func Run(p Plugin) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	endpoint := jsonrpc.New(ctx, os.Stdin, os.Stdout, &rpcHandler{p: p}, nil)
	if er, ok := p.(RPCEndpointReceiver); ok {
		er.ReceiveEndpoint(ctx, endpoint)
	}

	defer func() { _ = endpoint.Conn().Close() }()

	select {
	case <-ctx.Done():
		return
	case <-endpoint.Done():
		return
	}
}

// FormatMessage formats a NotificationRequest message and adds to the given io.Writer.
//
// The created message is a multi-line message as one might expect it in an email.
func FormatMessage(writer io.Writer, req *NotificationRequest) {
	if req.Event.Message != "" {
		_, _ = fmt.Fprintf(writer, "Message: %s\n\n", req.Event.Message)
	}

	_, _ = fmt.Fprintf(writer, "When: %s\n\n", req.Event.Time.Format("2006-01-02 15:04:05 MST"))
	_, _ = fmt.Fprintf(writer, "Object: %s\n\n", req.Object.Url)
	_, _ = writer.Write([]byte("Tags:\n"))
	for k, v := range utils.IterateOrderedMap(req.Object.Tags) {
		_, _ = fmt.Fprintf(writer, "%s: %s\n", k, v)
	}
}

// FormatSubject returns the formatted subject string.
func FormatSubject(req *NotificationRequest) string {
	if req.Incident != nil {
		return fmt.Sprintf("[#%d] %s is %s", req.Incident.Id, req.Object.Name, req.Incident.Severity)
	}
	return req.Object.Name
}
