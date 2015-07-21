var msgCurUser = 0

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
    if (msg.MsgType == 'In') {
        div.className += ' message_in'
    } else {
        div.className += ' message_out'
    }
    div.appendChild(document.createTextNode(msg.Text))
    div.appendChild(createTsEl(msg.Ts))
    return div
}

function showMessagesResponse(id, reply, erase) {
	var len = reply.Messages.length
	var el = document.getElementById("messages_texts")
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
					UserTo: +id,
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
		return;
	}

    document.getElementById("messages_texts").appendChild(createMsgEl(msg))
}

function showMessages(id) {
	id = +id
	msgCurUser = id
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
