var msgCurUser = 0
loaders["messages"] = loadMessageUsers
var DEFAULT_MESSAGES_LIMIT = 50

function loadMessageUsers() {
	sendReq(
		"REQUEST_GET_MESSAGES_USERS",
		{Limit: DEFAULT_MESSAGES_LIMIT},
		function (reply) {
			redrawMessageUsers(reply.Users)
			var location_parts = ("" + location.pathname).split(/\//g)
			if (location_parts[2]) {
				msgCurUser = location_parts[2]
				showMessages(msgCurUser)
			}
		}
	)
}

function redrawMessageUsers(users) {
	var msgUsers = [], shownUsers = {}
	var userInfo, i, el
	for (i = 0; i < users.length; i++) {
		userInfo = users[i]
		shownUsers[userInfo.Id] = true
		msgUsers.push('<div class="user" id="messages' + userInfo.Id + '"><span id="messages' + userInfo.Id + '">' + htmlspecialchars(userInfo.Name) + "</span></div>")
	}

	document.getElementById("users").innerHTML = msgUsers.join(" ")

	el = document.getElementById('messages' + msgCurUser)
	if (el) {
		el.className = "user user_current"
	}

	var els = document.getElementsByClassName("user")
	for (i = 0; i < els.length; i++) {
		el = els[i]
		addEv(el.id, 'click', function(ev) {
			showMessages(ev.target.id.replace('messages', ''))
		})
	}
}

function fmtDate(num)
{
	var res = '' + num
	while (res.length < 2) {
		res = '0' + res
	}
	return res
}

/**
 * Return message div that corresponds to msg object
 *
 * @param msg
 * @returns {HTMLElement}
 */
function createMsgEl(msg) {
	var div = document.createElement('div')
	div.className = 'message'
	if (msg.IsOut) {
		div.className += ' message_out'
	} else {
		div.className += ' message_in'
	}
	div.setAttribute('data-ts', msg.Ts)
	div.appendChild(document.createTextNode(msg.Text))
	div.appendChild(createTsEl(msg.Ts))
	return div
}

function showMessagesResponse(id, reply, erase) {
	var len = reply.Messages.length
	var el = document.getElementById("message-content")
	if (erase) el.innerHTML = ''

	var minTs

	if (len > 0) {
		var sdt = new Date(reply.Messages[0].Ts / 1e6)
		var startOfDay = sdt.getDate() + '-' + sdt.getMonth() + '-' + sdt.getYear()
	}

	var drawSep = function() {
		var sep = document.createElement('div')
		sep.className = 'date_separator'
		sep.innerHTML = sdt.getDate() + ' ' + months[sdt.getMonth() + 1] + ' ' + sdt.getFullYear()
		el.insertBefore(sep, el.firstChild)
	}

	for (var i = 0; i < len; i++) {
		var msg = reply.Messages[i]
		var dt = new Date(msg.Ts / 1e6)
		var dayStart = dt.getDate() + '-' + dt.getMonth() + '-' + dt.getYear()

		if (dayStart != startOfDay) {
			drawSep()
			startOfDay = dayStart
			sdt = dt
		}

		var msgEl = createMsgEl(msg);
		el.insertBefore(msgEl, el.firstChild)

		minTs = msg.Ts
		// do not show 'extra' message because it would otherwise be duplicate
		if (i >= DEFAULT_MESSAGES_LIMIT - 1) break
	}

	drawSep()

	if (len > DEFAULT_MESSAGES_LIMIT) {
		var div = document.createElement('div')
		var textEl = document.createTextNode('...')
		div.appendChild(textEl)
		div.id = 'messages_show_more'
		el.insertBefore(div, el.firstChild)
		addEv(div.id, 'click', function() {
			sendReq(
				"REQUEST_GET_MESSAGES",
				{
					UserTo: id,
					DateEnd: minTs,
					Limit: DEFAULT_MESSAGES_LIMIT + 1
				},
				function (reply) {
					div.parentNode.removeChild(div)
					showMessagesResponse(id, reply, false)
				}
			)
		})
	}

	try {
		if (erase || !id) {
			el.lastChild.scrollIntoView()
		}
	} catch (e) {
		console.log('could not scroll into view', e)
	}
}

function onNewMessage(msg) {
	if (msg.UserFrom != msgCurUser) {
        showNotification("Message from " + msg.UserFromName + ":\n" + msg.Text)
		console.log("Received message: ", msg)
		console.log("UserFrom: ", msg.UserFrom, ", msgCurUser: ", msgCurUser)
		return
	}

	var el = document.getElementById("message-content")

	if (el.lastChild) {
		var sdt = new Date(el.lastChild.getAttribute('data-ts') / 1e6)
		var startOfDay = sdt.getDate() + '-' + sdt.getMonth() + '-' + sdt.getYear()
	}

	var dt = new Date(msg.Ts / 1e6)
	var dayStart = dt.getDate() + '-' + dt.getMonth() + '-' + dt.getYear()

	if (dayStart < startOfDay) {
		var sep = document.createElement('div')
		sep.className = 'date_separator'
		sep.innerHTML = sdt.getDate() + ' ' + months[sdt.getMonth() + 1] + ' ' + sdt.getFullYear()
		el.appendChild(sep)
	}

	var msgEl = createMsgEl(msg)
	el.appendChild(msgEl)
	try {
		msgEl.scrollIntoView()
	} catch (e) {
		console.log("Could not scroll into view", e)
	}
}

function showMessages(id) {
	var prev_el = document.getElementById('messages' + msgCurUser)
	if (prev_el) {
		prev_el.className = "user"
	}

	var el = document.getElementById('messages' + id)
	el.className = "user user_current"

	msgCurUser = id

	history.replaceState(null, "Messages", "/messages/" + msgCurUser)

	sendReq(
		"REQUEST_GET_MESSAGES",
		{UserTo: id, Limit: DEFAULT_MESSAGES_LIMIT + 1},
		function (reply) { showMessagesResponse(id, reply, true) }
	)
}

function sendMessage(msgText) {
	sendReq("REQUEST_SEND_MESSAGE", {UserTo: msgCurUser, Text: msgText}, function(reply) {
		console.log("Send message: ", reply)
	})
}

function SetUpMessagesPage() {
	addEv("messages_link", "click", function(ev) {
		changeLocation("Messages", "/messages/")
		return false
	})
	addEv("msg", 'keydown', function(ev) {
		if (ev.keyCode == 13) {
			sendMessage(ev.target.value)
			ev.target.value = ''
			return false
		}
	})
	addEv("send_msg", 'click', function() {
		var msgEl = document.getElementById('msg')
		sendMessage(msgEl.value)
		msgEl.value = ''
	})
}
