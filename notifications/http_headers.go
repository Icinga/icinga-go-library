package notifications

// XIcingaRejectIfRelationsIncomplete is a custom HTTP header that can be used by sources to indicate that Icinga
// Notifications should reject the request if the event's relations are incomplete.
//
// By default, Icinga Notifications will attempt to process events even if their relations are incomplete,
// which may not be desirable in some cases. If a source sets this header to "true", Icinga Notifications
// rejects the request with all the missing relations in the response body, allowing the source to retry
// with the complete set of relations.
const XIcingaRejectIfRelationsIncomplete = "X-Icinga-Reject-If-Relations-Incomplete"
