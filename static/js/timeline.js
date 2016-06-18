var DEFAULT_TIMELINE_LIMIT = 10

function SetUpTimelinePage() {
	addEv("timeline_link", "click", function(ev) {
		changeLocation("Timeline", "/timeline/")
		return false
	})

	addEv("timeline_msg", 'keydown', function(ev) {
		if (ev.keyCode == 13) {
			addToTimeline(ev.target.value)
			ev.target.value = ''
		}
	})
}

function addToTimeline(msgText) {
	sendReq("REQUEST_ADD_TO_TIMELINE", {Text: msgText}, function(reply) {
		console.log("Add to timeline: ", reply)
	})
}

function createTimelineEl(msg) {
	var div = document.createElement('div')
	div.className = 'timeline_event'

	var userNameEl = document.createElement('div')
	userNameEl.className = 'timeline_username'
	userNameEl.appendChild(document.createTextNode(msg.UserName))

	var messageEl = document.createElement('div')
	messageEl.className = 'timeline_msg'

	messageEl.appendChild(document.createTextNode(msg.Text))

	div.appendChild(userNameEl)
	div.appendChild(createTsEl(msg.Ts))
	div.appendChild(messageEl)

	return div
}

function showTimelineResponse(reply) {
	var len = reply.Messages.length
	var el = document.getElementById("timeline_texts")

	var minTs

	for (var i = 0; i < len; i++) {
		var msg = reply.Messages[i]
		el.appendChild(createTimelineEl(msg))
		minTs = msg.Ts

		// do not show 'extra' message because it would otherwise be duplicate
		if (i >= DEFAULT_TIMELINE_LIMIT - 1) break
	}

	if (len > DEFAULT_TIMELINE_LIMIT) {
		var div = document.createElement('div')
		var textEl = document.createTextNode('...')
		div.appendChild(textEl)
		div.id = 'timeline_show_more'
		el.appendChild(div)
		addEv(div.id, 'click', function() {
			sendReq(
				"REQUEST_GET_TIMELINE",
				{
					DateEnd: minTs,
					Limit: DEFAULT_TIMELINE_LIMIT + 1,
				},
				function (reply) {
					div.parentNode.removeChild(div)
					showTimelineResponse(reply)
				}
			)
		})
	}
}

function onNewTimelineEvent(msg) {
	var el = document.getElementById("timeline_texts")
	el.insertBefore(createTimelineEl(msg), el.firstChild)
}

function ShowTimeline() {
	sendReq(
		"REQUEST_GET_TIMELINE",
		{Limit: DEFAULT_TIMELINE_LIMIT + 1},
		showTimelineResponse
	)
}
