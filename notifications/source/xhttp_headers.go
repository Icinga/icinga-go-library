package source

// These headers are used to pass metadata about the request made to the Icinga Notifications API.
// Currently, they are used to convey the version of the rules and the ID of the rules being used.
//
// They should be set in the HTTP request headers when making requests to the Icinga Notifications API,
// and by the Icinga Notifications daemon when processing such requests.
const (
	XIcingaRulesVersion = "X-Icinga-Rules-Version"
	XIcingaRulesId      = "X-Icinga-Rules-Id"
)
