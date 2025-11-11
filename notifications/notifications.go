// Package notifications contains subpackages related to Icinga Notifications.
//
// The event package defines the central Event type used to describe Icinga Notifications events.
//
// When implementing a new Icinga Notifications [Channel Plugins] to allow notifications be sent over a new way, take a
// look at the plugin package. Internally, this uses the rpc package, which, however, should not be relevant for
// mere channel plugin implementations.
//
// When implementing a new Icinga Notifications Source to submit events to Icinga Notifications' [HTTP API], consider
// the source package.
//
// [Channel Plugins]: https://icinga.com/docs/icinga-notifications/latest/doc/10-Channels/
// [HTTP API]: https://icinga.com/docs/icinga-notifications/latest/doc/20-HTTP-API/
package notifications
