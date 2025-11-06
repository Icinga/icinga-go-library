// Package source implements an Icinga Notifications source to send events to the Icinga Notifications API.
//
// To create a new source, start by creating a new [Client]. Then, for each event you want to forward to Icinga
// Notifications, call [Client.ProcessEvent]. This method forwards the event to the Icinga Notifications API and
// reports further information, such as outdated rules.
package source
