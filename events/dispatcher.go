package events

import (
	"fmt"
	"log"

	"github.com/YuriyNasretdinov/social-net/protocol"
	"github.com/YuriyNasretdinov/social-net/session"
)

const (
	EVENT_USER_CONNECTED = iota
	EVENT_USER_DISCONNECTED
	EVENT_ONLINE_USERS_LIST
	EVENT_USER_REPLY
	EVENT_NEW_MESSAGE
	EVENT_NEW_TIMELINE_EVENT
)

type (
	BaseEvent struct {
		Type string
	}

	ControlEvent struct {
		EvType   uint8
		Info     interface{}
		Reply    interface{}
		Listener chan interface{}
	}

	EventUserConnected struct {
		BaseEvent
		protocol.JSUserInfo
	}

	EventUserDisconnected struct {
		BaseEvent
		protocol.JSUserInfo
	}

	EventOnlineUsersList struct {
		BaseEvent
		Users []protocol.JSUserInfo
	}

	EventNewMessage struct {
		BaseEvent
		protocol.Message
	}

	InternalEventNewMessage struct {
		UserFrom uint64
		UserTo   uint64
		Ts       string
		Text     string
	}

	EventNewTimelineStatus struct {
		BaseEvent
		protocol.TimelineMessage
	}

	InternalEventNewTimelineStatus struct {
		UserId   uint64
		UserName string
		Ts       string
		Text     string
	}
)

var (
	EventsFlow = make(chan *ControlEvent, 200)
)

func handleUserConnected(listenerMap map[chan interface{}]*session.SessionInfo, userListeners map[uint64]map[chan interface{}]bool, ev *ControlEvent) {
	evInfo, ok := ev.Info.(*session.SessionInfo)
	if !ok {
		log.Println("VERY BAD: Type assertion failed: ev info is not SessionInfo")
		return
	}

	currentUsers := make([]protocol.JSUserInfo, 0, len(listenerMap))
	for _, info := range listenerMap {
		currentUsers = append(currentUsers, protocol.JSUserInfo{Name: info.Name, Id: fmt.Sprint(info.Id)})
	}

	ouEvent := new(EventOnlineUsersList)
	ouEvent.Type = "EVENT_ONLINE_USERS_LIST"
	ouEvent.Users = currentUsers
	ev.Listener <- ouEvent

	listenerMap[ev.Listener] = evInfo

	if userListeners[evInfo.Id] == nil {
		userListeners[evInfo.Id] = make(map[chan interface{}]bool)
	}

	userListeners[evInfo.Id][ev.Listener] = true

	for listener := range listenerMap {
		if len(listener) >= cap(listener) {
			return
		}

		event := new(EventUserConnected)
		event.Type = "EVENT_USER_CONNECTED"
		event.Name = evInfo.Name
		event.Id = fmt.Sprint(evInfo.Id)
		listener <- event
	}
}

func handleUserDisconnected(listenerMap map[chan interface{}]*session.SessionInfo, userListeners map[uint64]map[chan interface{}]bool, ev *ControlEvent) {
	evInfo, ok := ev.Info.(*session.SessionInfo)
	if !ok {
		log.Println("VERY BAD: Type assertion failed: ev info is not SessionInfo when user disconnects")
		return
	}

	delete(listenerMap, ev.Listener)
	if userListeners[evInfo.Id] != nil {
		delete(userListeners[evInfo.Id], ev.Listener)
		if len(userListeners[evInfo.Id]) == 0 {
			delete(userListeners, evInfo.Id)
		}
	}

	for listener := range listenerMap {
		if len(listener) >= cap(listener) {
			return
		}

		event := new(EventUserDisconnected)
		event.Type = "EVENT_USER_DISCONNECTED"
		event.Name = evInfo.Name
		event.Id = fmt.Sprint(evInfo.Id)
		listener <- event
	}
}

func handleNewMessage(listenerMap map[chan interface{}]*session.SessionInfo, userListeners map[uint64]map[chan interface{}]bool, ev *ControlEvent) {
	sourceEvent, ok := ev.Info.(*InternalEventNewMessage)
	if !ok {
		log.Println("VERY BAD: Type assertion failed: source event is not InternalEventNewMessage in handleNewMessage")
		return
	}

	event := new(EventNewMessage)
	event.Type = "EVENT_NEW_MESSAGE"
	event.Ts = sourceEvent.Ts
	event.Text = sourceEvent.Text

	if userListeners[sourceEvent.UserFrom] != nil {
		for listener := range userListeners[sourceEvent.UserFrom] {
			fromEv := new(EventNewMessage)
			*fromEv = *event
			fromEv.UserFrom = fmt.Sprint(sourceEvent.UserTo)
			fromEv.MsgType = protocol.MSG_TYPE_OUT
			select {
			case listener <- fromEv:
			default:
			}

		}
	}

	if userListeners[sourceEvent.UserTo] != nil {
		for listener := range userListeners[sourceEvent.UserTo] {
			toEv := new(EventNewMessage)
			*toEv = *event
			toEv.UserFrom = fmt.Sprint(sourceEvent.UserFrom)
			toEv.MsgType = protocol.MSG_TYPE_IN
			select {
			case listener <- toEv:
			default:
			}
		}
	}
}

func handleNewTimelineEvent(listenerMap map[chan interface{}]*session.SessionInfo, userListeners map[uint64]map[chan interface{}]bool, ev *ControlEvent) {
	evInfo, ok := ev.Info.(*InternalEventNewTimelineStatus)
	if !ok {
		log.Println("Type assertion failed: evInfo is not InternalEventNewTimelineStatus in handleNewTimelineEvent")
		return
	}

	for listener := range listenerMap {
		userEv := new(EventNewTimelineStatus)
		userEv.Type = "EVENT_NEW_TIMELINE_EVENT"
		userEv.Ts = evInfo.Ts
		userEv.UserId = fmt.Sprint(evInfo.UserId)
		userEv.Text = evInfo.Text
		userEv.UserName = evInfo.UserName

		select {
		case listener <- userEv:
		default:
		}
	}
}

func EventsDispatcher() {
	listenerMap := make(map[chan interface{}]*session.SessionInfo)
	userListeners := make(map[uint64]map[chan interface{}]bool)

	for ev := range EventsFlow {
		if ev.EvType == EVENT_USER_CONNECTED {
			handleUserConnected(listenerMap, userListeners, ev)
		} else if ev.EvType == EVENT_USER_DISCONNECTED {
			handleUserDisconnected(listenerMap, userListeners, ev)
		} else if ev.EvType == EVENT_NEW_MESSAGE {
			handleNewMessage(listenerMap, userListeners, ev)
		} else if ev.EvType == EVENT_NEW_TIMELINE_EVENT {
			handleNewTimelineEvent(listenerMap, userListeners, ev)
		} else if ev.EvType == EVENT_USER_REPLY {
			if _, ok := listenerMap[ev.Listener]; !ok {
				continue
			}

			select {
			case ev.Listener <- ev.Reply:
			default:
			}
		}
	}
}
