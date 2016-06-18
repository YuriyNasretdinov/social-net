var allUsers = {}
var seqId = 0
var rcvCallbacks = {}
var websocket

var connected = false
var pendingRequests = []

var DEFAULT_MESSAGES_LIMIT = 5

var tabs = ['messages', 'timeline', 'friends', 'users_list']
var loaders = {}

function updateConnectionStatus() {
	document.getElementById('status').innerHTML = '<span style="color: ' + (connected ? 'green' : 'red') + ';">•</span>';
}

function onUserConnect(userInfo) {
	var userId = userInfo.Id
	if (allUsers[userId]) {
		allUsers[userId].cnt++
	} else {
		allUsers[userId] = userInfo
		allUsers[userId].cnt = 1
	}
}

function onUserDisconnect(userInfo) {
	var userId = userInfo.Id
	if (allUsers[userId]) {
		allUsers[userId].cnt--
		if (allUsers[userId].cnt <= 0) {
			delete(allUsers[userId])
		}
	}
}

function onMessage(evt) {
	var reply = JSON.parse(evt.data)
	if (reply.Type == 'EVENT_ONLINE_USERS_LIST') {
		for (var i = 0; i < reply.Users.length; i++) {
			onUserConnect(reply.Users[i])
		}
	} else if (reply.Type == 'EVENT_USER_CONNECTED') {
		onUserConnect(reply)
	} else if (reply.Type == 'EVENT_USER_DISCONNECTED') {
		onUserDisconnect(reply)
	} else if (reply.Type == 'EVENT_NEW_MESSAGE') {
		onNewMessage(reply)
	} else if (reply.Type == 'EVENT_NEW_TIMELINE_EVENT') {
		onNewTimelineEvent(reply)
	} else {
		if (!rcvCallbacks[reply.SeqId]) {
			console.log("Received response for missing seqid")
			console.log(reply)
		} else {
			if (reply.Type == 'REPLY_ERROR') {
				var el = document.getElementById('error_text')
				el.style.display = ''
				el.innerHTML = ''
				el.appendChild(document.createTextNode(reply.Message))
				setTimeout(function() { el.style.display = 'none' }, 2000)
			} else {
				rcvCallbacks[reply.SeqId](reply)
			}
			
			delete(rcvCallbacks[reply.SeqId])
		}
	}

	redrawUsers()
}

function sendReq(reqType, reqData, onrcv) {
	var cb = function() {
		var msg = reqType + " " + seqId + "\n" + JSON.stringify(reqData)
		websocket.send(msg)
		rcvCallbacks[seqId] = onrcv
		seqId++
	}

	if (connected) {
		cb()
	} else {
		pendingRequests.push(cb)
	}
}

function redrawUsers() {
	var str = '<b>online users</b>'

	for (var userId in allUsers) {
		var userInfo = allUsers[userId]
		if (userId == ourUserId) continue
		str += '<br/>' + userInfo.Name
	}

	document.getElementById("online_users").innerHTML = str
}

function setWebsocketConnection() {
	rcvCallbacks = {}
	websocket = new WebSocket("ws://" + window.location.host + "/events")
	websocket.onopen = function(evt) {
		connected = true
		updateConnectionStatus()

		for (var i = 0; i < pendingRequests.length; i++) {
			pendingRequests[i]()
		}
	}
	websocket.onclose = function(evt) {
		pendingRequests = []
		connected = false
		updateConnectionStatus()

		console.log("close")
		setTimeout(setWebsocketConnection, 1000)
	}
	websocket.onmessage = onMessage
	websocket.onerror = function(evt) { console.log("Error: " + evt) }
}

function addEv(id, evName, func) {
	var el

	if (typeof id == "string") {
		el = document.getElementById(id)
		if (!el) {
			console.log("el not found: ", id)
			return false
		}
	} else {
		el = id
	}

	el.addEventListener(
		evName,
		function (ev) {
			if (func(ev) === false) {
				ev.preventDefault()
				ev.stopPropagation()
			}
		},
		true
	)
	return true
}

function hideAll() {
	for (var i = 0; i < tabs.length; i++) {
		document.getElementById(tabs[i]).style.display = 'none'
	}
}

function ShowCurrent() {
	for (var i = 0; i < tabs.length; i++) {
		var tab = tabs[i]
		if (location.pathname.indexOf('/' + tab + '/') !== -1) {
			document.getElementById(tabs[i]).style.display = ''
			if (loaders[tab]) {
				loaders[tab]()
			}
			break
		}
	}
}

function changeLocation(title, url) {
	history.replaceState(null, title, url)
	hideAll()
	ShowCurrent()
}

function createTsEl(ts) {
	var tsEl = document.createElement('div')
	tsEl.className = 'ts'
	var dt = new Date(ts / 1e6)
	tsEl.appendChild(
		document.createTextNode(
			fmtDate(dt.getHours()) + ':' +
				fmtDate(dt.getMinutes()) + ':' +
				fmtDate(dt.getSeconds())
		)
	)
	return tsEl
}

function setUpPage() {
	hideAll()
	SetUpMessagesPage()
	SetUpTimelinePage()
	SetUpFriendsPage()
	ShowCurrent()

	ShowTimeline()
}
