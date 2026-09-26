package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/lasseh/taillight-tui/internal/tui/client"
	"github.com/lasseh/taillight-tui/internal/tui/component"
)

// notifySeverityMax is the max severity that triggers a toast notification.
// Events with severity <= this value will show a toast (0=emerg, 3=err).
const notifySeverityMax = 3

// notifyMaxAge is the cutoff for triggering toasts on backfill events.
// Events older than this are silently skipped — otherwise reconnecting
// to a busy stream would flood the screen with old alerts.
const notifyMaxAge = 30 * time.Second

// syslogToast returns the toast rule for a syslog-shaped feed: alert on
// severity err and worse.
func syslogToast(feed string) func(client.SrvlogEvent) (component.Toast, bool) {
	return func(e client.SrvlogEvent) (component.Toast, bool) {
		if e.Severity > notifySeverityMax || time.Since(e.ReceivedAt) > notifyMaxAge {
			return component.Toast{}, false
		}
		return component.Toast{
			Title:   fmt.Sprintf("[%s] %s", strings.ToUpper(e.SeverityLabel), e.Hostname),
			Message: e.Message,
			Feed:    feed,
			Level:   e.Severity,
			Time:    e.ReceivedAt,
		}, true
	}
}

// applogToast alerts on FATAL and ERROR applog events.
func applogToast(e client.AppLogEvent) (component.Toast, bool) {
	if (e.Level != "FATAL" && e.Level != "ERROR") || time.Since(e.ReceivedAt) > notifyMaxAge {
		return component.Toast{}, false
	}
	level := 3 // default to "err" level for toast color
	if e.Level == "FATAL" {
		level = 0
	}
	return component.Toast{
		Title:   fmt.Sprintf("[%s] %s", e.Level, e.Service),
		Message: e.Msg,
		Feed:    "applog",
		Level:   level,
		Time:    e.ReceivedAt,
	}, true
}
