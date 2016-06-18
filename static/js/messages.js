var msgCurUser = 0
loaders["messages"] = loadMessageUsers;

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
		msgUsers.push('<div class="user" id="messages' + userInfo.Id + '"><img src="https://new.vk.com/images/camera_200.png" id="messages' + userInfo.Id + '"><span id="messages' + userInfo.Id + '">' + userInfo.Name + "</span></div>")
	}

	for (var userId in allUsers) {
		if (shownUsers[userId] || userId == ourUserId) continue
		userInfo = allUsers[userId]
		msgUsers.push('<div class="user" id="messages' + userInfo.Id + '"><img src="https://new.vk.com/images/camera_200.png" id="messages' + userInfo.Id + '">' + userInfo.Name + "</div>")
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
	div.appendChild(document.createTextNode(msg.Text))
	div.appendChild(createTsEl(msg.Ts))
	return div
}

function showMessagesResponse(id, reply, erase) {
	var len = reply.Messages.length
	var el = document.getElementById("message-content")
	if (erase) el.innerHTML = ''

	var minTs

	for (var i = 0; i < len; i++) {
		var msg = reply.Messages[i]
		el.insertBefore(createMsgEl(msg), el.firstChild)

		minTs = msg.Ts
		// do not show 'extra' message because it would otherwise be duplicate
		if (i >= DEFAULT_MESSAGES_LIMIT - 1) break
	}

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
					Limit: DEFAULT_MESSAGES_LIMIT + 1,
				},
				function (reply) {
					div.parentNode.removeChild(div)
					showMessagesResponse(id, reply, false)
				}
			)
		})
	}
}

function onNewMessage(msg) {
	if (msg.UserFrom != msgCurUser) {
		// TODO: show messages from different users
		console.log("Received message: ", msg)
		console.log("UserFrom: ", msg.UserFrom, ", msgCurUser: ", msgCurUser)
		return
	}

	document.getElementById("message-content").appendChild(createMsgEl(msg))
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
		}
	})
}
