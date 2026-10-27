# Icinga Go Library

## 0.10.0 (2026-10-29)

This Icinga Go Library release is made for Icinga Notifications v1.0.0 and Icinga DB 1.6.0.

The `notifications` package was reworked with several breaking changes.
Besides that, this release adds TLS configuration options, extends the database package, and fixes configuration loading.

Icinga Notifications related changes:

* notifications/event: Rework events with relations, a UUID-based `ID`, a `Summary`, non-state events, and shared validation. #199, #202, #203, #212, #216, #232, #240, #247, #254, #263, #266, #267, #276, #277
* notifications/event: Reduce severities to `ok`, `warning`, `error`, and `critical`. #268
* notifications/source: Support the v1 HTTP API with attribute negotiation, configurable relations, and a unified response format. #201, #205, #217, #222, #229, #241, #242, #243, #257, #258, #262, #271, #272
* notifications/source: Unix domain socket and HTTPS transports. #224
* notifications/plugin: Replace the custom `rpc` package with the JSON-RPC 2.0 compliant `jsonrpc` package, allowing any standard JSON-RPC library to be used for channel plugins. #226
* notifications/plugin: Version `NotificationRequest`. Plugins announce supported versions via `supported_request_versions` in `GetInfo`. #269
* notifications/plugin: Allow `SendNotification` to return delivery results and to store channel state, keyed by Icinga Notifications. #231, #259
* notifications/plugin: Extend `NotificationRequest` and `Incident` for improved notifications, e.g., `Body`, `Summary`, `muted`, and `recovered`. #228, #263
* notifications/plugin: Remove `FormatMessage`, `FormatSubject`, and the incident `url` in favor of the source's `icingaweb2_url` option. #239, #263
* notifications/plugin: Conditional configuration options via `ConfigOption.Children`. #250, #270

Other changes:

* config: TLS server configuration options and CRL-based client certificate revocation checks. #204, #207, #219
* config: Fix password file validation for YAML and zero values being overwritten by defaults. #227, #274
* database: Add `TxOptions` to `DB.ExecTx`, column exclusion for `DB.BuildUpsertStmt`, `OnSuccessApplyAndSendTo`, and retries for `YieldAll`. #189, #212, #215, #221
* database: Fix MySQL driver log messages being wrapped in square brackets. #261
* redis: Add `XDelOnAllConsumersAck`. #212
* types: Support SQL NULL and JSON for `UUID`. #235
* utils: Rename `Ellipsize` to `EllipsizeRunes` and add `EllipsizeBytes`. #244
* com: Add `ErrgroupReceiveWrap`. #188

## 0.9.0 (2026-03-04)

This Icinga Go Library release introduces streaming deletions/upserts for databases,
adds support for password files next to password fields for configurations,
and includes some further housekeeping.

* database: Stream functionality for deletions and upserts. #16
* redis: Remove obsolete workaround against reuse of broken connections. #148
* config: Password files next to password fields. #180
* Address golangci-lint issues. #182
* Test downstream Icinga Go applications against latest Icinga Go Library. #183
* Bump dependencies. #177, #178, #179, #181, #184

## 0.8.2 (2025-12-15)

This release of the Icinga Go Library updates dependencies to ensure that the latest changes are available for the upcoming Icinga DB release.

* Bump dependencies. #169, #170, #171, #173, #174, #175

## 0.8.1 (2025-11-17)

This Icinga Go Library release changes the Notifications Source base URL parameter name from `api-base-url` to `url`.
The motivation behind this rewording is to have an identical naming between Icinga DB v1.5.0 and Icinga for Kubernetes.

* notifications: Rename Source `api-base-url` to `url`. #168

## 0.8.0 (2025-11-12)

This Icinga Go Library release is made for Icinga DB v1.5.0 and Icinga Notifications v0.2.0.

Most notably, the new `notifications` package provides components for implementing Icinga Notifications sources and channels.

* notifications: Add a package that helps with implementing sources and channels for Icinga Notifications. #145, #160, #161
* redis: Disable client maint notifications. #162
* logging: Allow to provide custom core factory func. #152
* retry: Remove ResetTimeout function. #142 
* Documentation: Add README.md, CHANGELOG.md, and some API docs. #164, #167
* New test. #33
* Bump dependencies.

## 0.7.2 (2025-06-17)

This IGL release simply updates dependencies to ensure the latest changes are available for the upcoming Icinga DB release.

* Bump dependencies. #138, #139, #140

## 0.7.1 (2025-06-02)

* backoff: Fix internal shifting overflow. #136
* Address golangci-lint issues. #137
* Bump dependencies. #134

## 0.7.0 (2025-05-27)

* database: Introduce DB#InsertObtainID() function. #64
* database,redis: More verbosity in OnRetryableError. #124
* config: Unset TLS Key environment variable. #125
* logging: Include error field in journald message, Fix key encoding of
* all fields for journaldCore. #126
* Import Icinga Notifications internal/utils. #127
* database,redis: No connection timeout after successful connection. #131
* backoff: Ensure bounds and introduce default value. #133
* Bump dependencies. #122, #123, #128, #129, #130
* New tests. #27, #32, #34, #36, #38, #46
