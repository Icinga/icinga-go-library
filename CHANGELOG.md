# Icinga Go Library

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
