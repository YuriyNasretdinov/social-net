var allUsers = {}
var seqId = 0
var rcvCallbacks = {}
var websocket

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
	} else {
		if (!rcvCallbacks[reply.SeqId]) {
			console.log("Received response for missing seqid")
			console.log(reply)
		} else {
			rcvCallbacks[reply.SeqId](reply)
			delete(rcvCallbacks[reply.SeqId])
		}
	}

	redrawUsers()
}

function sendReq(req, onrcv) {
	var ourSeqId = seqId
	seqId++
	req.SeqId = ourSeqId
	websocket.send(JSON.stringify(req))
	rcvCallbacks[ourSeqId] = onrcv
}

function showMessagesResponse(reply) {
	var messages = []
	var len = reply.Messages.length
	var haveMore = false
	if (len > 5) {
		len = 5
		messages.push("...")
	}
	for (var i = len - 1; i >= 0; i--) {
		var msg = reply.Messages[i]
		messages.push('<div class="message">' + msg.Text + '</div>')
	}

	document.getElementById("messages_texts").innerHTML = messages.join('')
}

function showMessages(id) {
	sendReq(
		{Type: "REQUEST_GET_MESSAGES", ReqData: JSON.stringify({
			UserTo: +id,
			Limit: 6,
		})},
		showMessagesResponse
	)
}

function redrawUsers() {
	var str = '<b>online users</b>'
	var msgUsers = []

	for (var userId in allUsers) {
		var userInfo = allUsers[userId]
		str += '<br/>' + userInfo.Name
		msgUsers.push('<div class="user" id="messages' + userInfo.Id + '">' + userInfo.Name + "</div>")
	}

	document.getElementById("online_users").innerHTML = str
	document.getElementById("users").innerHTML = msgUsers.join(" ")
	var els = document.getElementsByClassName("user")
	for (var i = 0; i < els.length; i++) {
		addEv(els[i].id, 'click', function(ev) {
			showMessages(ev.target.id.replace('messages', ''))
		})
	}
}

function setWebsocketConnection() {
	rcvCallbacks = {}
	websocket = new WebSocket("ws://" + window.location.host + "/events")
	websocket.onopen = function(evt) { console.log("open") }
	websocket.onclose = function(evt) { console.log("close"); setTimeout(setWebsocketConnection, 1000) }
	websocket.onmessage = onMessage
	websocket.onerror = function(evt) { console.log("Error: " + evt) }
}

function addEv(id, evName, func) {
	var el = document.getElementById(id)
	if (!el) {
		console.log("el not found: ", id)
		return false
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

function setUpPage() {
	addEv("messages_link", "click", function(ev) {
		document.getElementsByClassName("messages")[0].style.display = ''
		return false
	})
}
