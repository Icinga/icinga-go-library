package notifications

// XIcingaEnableAttributesNegotiation is the HTTP header used to indicate that the client supports attributes negotiation.
//
// Use this header in your HTTP requests to the Icinga Notifications API to indicate that your client
// supports attributes negotiation. Icinga Notifications will then instruct the client to provide missing
// information for events in follow-up requests if the initial request did not contain complete information
// needed to evaluate the rules before it can process or discard the event.
const XIcingaEnableAttributesNegotiation = "X-Icinga-Enable-Attributes-Negotiation"
