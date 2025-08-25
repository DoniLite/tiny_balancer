package core

import (
	"context"

	goevents "github.com/DoniLite/go-events"
)

var EventBus *goevents.EventFactory

var (
	ServerStartedEvent     *goevents.Event
	ErrorDroppedEvent     *goevents.Event
	CertManagerActionEvent *goevents.Event
)

func init() {
	EventBus = goevents.NewEventBus()

	ServerStartedEvent = EventBus.CreateEvent("server_started")
	ErrorDroppedEvent = EventBus.CreateEvent("error_dropped")
	CertManagerActionEvent = EventBus.CreateEvent("cert_manager_action")
}

func AddEventHandler(event *goevents.Event, handler goevents.EventHandler) {
	EventBus.On(event, handler)
}

func onCertManagerEvent(ctx context.Context, event string, data map[string]any) error {
	EventBus.Emit(CertManagerActionEvent, &goevents.EventData{Message: event, Payload: data})
	return nil
}

func SubscribeToEvent(handler goevents.EventHandler, events ...*goevents.Event) {
	EventBus.Subscribe(handler, events...)
}