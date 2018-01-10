var DEFAULT_TIMELINE_LIMIT = 10

loaders["timeline"] = function() {
	var parts = window.location.pathname.split('/')

	if (parts[2] == 'hash') {
		ShowHash(decodeURIComponent(parts[3]))
	} else {
		ShowTimeline()
	}
}

function SetUpTimelinePage() {
	addEv("timeline_link", "click", function(ev) {
		changeLocation("Timeline", "/timeline/")
		return false
	})

	addEv("timeline_msg", 'keydown', function(ev) {
		if (ev.keyCode == 13) {
			addToTimeline(ev.target.value)
			ev.target.value = ''
			return false
		}
	})

	addEv("send_timeline_msg", "click", function(ev) {
		var el = document.getElementById("timeline_msg")
		addToTimeline(el.value)
		el.value = ''
	})
}

function addToTimeline(msgText) {
	sendReq("REQUEST_ADD_TO_TIMELINE", {Text: msgText}, function(reply) {
		console.log("Add to timeline: ", reply)
	})
}

function createTimelineEl(msg) {
	var hashRegex = /(#[^ ]+)/g
	var div = document.createElement('div')
	div.className = 'timeline_event'

	var userNameEl = document.createElement('div')
	userNameEl.className = 'timeline_username'
	userNameEl.appendChild(profileLink(msg.UserId, msg.UserName))

	var messageEl = document.createElement('div')
	messageEl.className = 'timeline_msg'

	var textParts = msg.Text.split(hashRegex)

	for (var i = 0; i < textParts.length; i++) {
		var part = textParts[i]
		if (hashRegex.test(part)) {
			var a = document.createElement('a')
			a.href = "#"
			a.onclick = (function(part) {
				return function(e) {
					var hash = part.substr(1)
					changeLocation("Timeline", "/timeline/hash/" + hash)
					e.preventDefault()
				}
			})(part)
			a.appendChild(document.createTextNode(part))
			messageEl.appendChild(a)
		} else {
			messageEl.appendChild(document.createTextNode(part))
		}
	}

	div.appendChild(userNameEl)
	div.appendChild(createTsEl(msg.Ts))
	div.appendChild(messageEl)

	return div
}

function showTimelineResponse(reply, first) {
	var len = reply.Messages.length
	var el = document.getElementById("timeline_texts")

	if (first) {
		el.innerHTML = ''
	}

	var minTs

	for (var i = 0; i < len; i++) {
		var msg = reply.Messages[i]
		el.appendChild(createTimelineEl(msg))
		minTs = msg.Ts

		// do not show 'extra' message because it would otherwise be duplicate
		if (i >= DEFAULT_TIMELINE_LIMIT - 1) break
	}

	loadMoreFunc = null

	if (len > DEFAULT_TIMELINE_LIMIT) {
		loadMoreFunc = function() {
			sendReq(
				"REQUEST_GET_TIMELINE",
				{
					DateEnd: minTs,
					Limit: DEFAULT_TIMELINE_LIMIT + 1
				},
				function (reply) {
					showTimelineResponse(reply, false)
				}
			)
		}
		window.onscroll()
	}
}

function showTimelineHashResponse(hash, reply, first) {
	var len = reply.Messages.length
	var el = document.getElementById("timeline_texts")

	if (first) {
		el.innerHTML = ''
		var header = document.createElement('h3')
		header.appendChild(document.createTextNode('Viewing timeline for #' + hash + ' '))
		var a = document.createElement('a')
		a.appendChild(document.createTextNode('cancel'))
		a.onclick = function(e) {
			changeLocation('Timeline', '/timeline/')
			e.preventDefault()
		}
		a.href = '#'
		header.className = 'hash_header'
		header.appendChild(a)
		el.appendChild(header)
	}

	var minTs

	for (var i = 0; i < len; i++) {
		var msg = reply.Messages[i]
		el.appendChild(createTimelineEl(msg))
		minTs = msg.Ts

		// do not show 'extra' message because it would otherwise be duplicate
		if (i >= DEFAULT_TIMELINE_LIMIT - 1) break
	}

	loadMoreFunc = null

	if (len > DEFAULT_TIMELINE_LIMIT) {
		loadMoreFunc = function() {
			sendReq(
				"REQUEST_GET_TIMELINE_FOR_HASH",
				{
					Hash: hash,
					DateEnd: minTs,
					Limit: DEFAULT_TIMELINE_LIMIT + 1
				},
				function (reply) {
					showTimelineHashResponse(hash, reply, false)
				}
			)
		}
		window.onscroll()
	}
}

function onNewTimelineEvent(msg) {
	var el = document.getElementById("timeline_texts")
	el.insertBefore(createTimelineEl(msg), el.firstChild)
}

function ShowHash(hash) {
	sendReq(
		"REQUEST_GET_TIMELINE_FOR_HASH",
		{Hash: hash, Limit: DEFAULT_TIMELINE_LIMIT + 1},
		function(reply) { showTimelineHashResponse(hash, reply, true) }
	)
}

function ShowTimeline() {
	sendReq(
		"REQUEST_GET_TIMELINE",
		{Limit: DEFAULT_TIMELINE_LIMIT + 1},
		function(reply) { showTimelineResponse(reply, true) }
	)
}
