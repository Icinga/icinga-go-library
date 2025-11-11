# Icinga Go Library

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
