var allUsers = {}
var seqId = 0
var rcvCallbacks = {}
var websocket

var connected = false
var pendingRequests = []

var tabs = ['messages', 'timeline', 'friends', 'users_list', 'profile']
var loaders = {}

function updateConnectionStatus() {
	document.getElementById('status').innerHTML = '<span style="color: ' + (connected ? 'green' : 'red') + ';">•</span>';
}

function htmlspecialchars (string, quoteStyle, charset, doubleEncode) {
	//       discuss at: http://locutus.io/php/htmlspecialchars/
	//      original by: Mirek Slugen
	//      improved by: Kevin van Zonneveld (http://kvz.io)
	//      bugfixed by: Nathan
	//      bugfixed by: Arno
	//      bugfixed by: Brett Zamir (http://brett-zamir.me)
	//      bugfixed by: Brett Zamir (http://brett-zamir.me)
	//       revised by: Kevin van Zonneveld (http://kvz.io)
	//         input by: Ratheous
	//         input by: Mailfaker (http://www.weedem.fr/)
	//         input by: felix
	// reimplemented by: Brett Zamir (http://brett-zamir.me)
	//           note 1: charset argument not supported
	//        example 1: htmlspecialchars("<a href='test'>Test</a>", 'ENT_QUOTES')
	//        returns 1: '&lt;a href=&#039;test&#039;&gt;Test&lt;/a&gt;'
	//        example 2: htmlspecialchars("ab\"c'd", ['ENT_NOQUOTES', 'ENT_QUOTES'])
	//        returns 2: 'ab"c&#039;d'
	//        example 3: htmlspecialchars('my "&entity;" is still here', null, null, false)
	//        returns 3: 'my &quot;&entity;&quot; is still here'

	var optTemp = 0
	var i = 0
	var noquotes = false
	if (typeof quoteStyle === 'undefined' || quoteStyle === null) {
		quoteStyle = 2
	}
	string = string || ''
	string = string.toString()

	if (doubleEncode !== false) {
		// Put this first to avoid double-encoding
		string = string.replace(/&/g, '&amp;')
	}

	string = string
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')

	var OPTS = {
		'ENT_NOQUOTES': 0,
		'ENT_HTML_QUOTE_SINGLE': 1,
		'ENT_HTML_QUOTE_DOUBLE': 2,
		'ENT_COMPAT': 2,
		'ENT_QUOTES': 3,
		'ENT_IGNORE': 4
	}
	if (quoteStyle === 0) {
		noquotes = true
	}
	if (typeof quoteStyle !== 'number') {
		// Allow for a single string or an array of string flags
		quoteStyle = [].concat(quoteStyle)
		for (i = 0; i < quoteStyle.length; i++) {
			// Resolve string input to bitwise e.g. 'ENT_IGNORE' becomes 4
			if (OPTS[quoteStyle[i]] === 0) {
				noquotes = true
			} else if (OPTS[quoteStyle[i]]) {
				optTemp = optTemp | OPTS[quoteStyle[i]]
			}
		}
		quoteStyle = optTemp
	}
	if (quoteStyle & OPTS.ENT_HTML_QUOTE_SINGLE) {
		string = string.replace(/'/g, '&#039;')
	}
	if (!noquotes) {
		string = string.replace(/"/g, '&quot;')
	}

	return string
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

function showError(msg) {
	var el = document.getElementById('error_text')
	el.style.display = ''
	el.innerHTML = ''
	el.appendChild(document.createTextNode(msg))
	setTimeout(function() { el.style.display = 'none' }, 2000)
}

function showNotification(msg) {
	var el = document.getElementById('notification')
	el.style.display = ''
	el.innerHTML = ''
	el.appendChild(document.createTextNode(msg))
	setTimeout(function() { el.style.display = 'none' }, 5000)
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
	} else if (reply.Type == 'EVENT_FRIEND_REQUEST') {
	    showNotification("User wants to add you to friends")
        friendsRequestsCount++
        redrawFriendsRequestCount()
	} else {
		if (!rcvCallbacks[reply.SeqId]) {
			console.log("Received response for missing seqid")
			console.log(reply)
		} else {
			if (reply.Type == 'REPLY_ERROR') {
				showError(reply.Message)
			} else {
				rcvCallbacks[reply.SeqId](reply)
			}
			
			delete(rcvCallbacks[reply.SeqId])
		}
	}

	redrawUsers()
}

var debugDelay = 0

function sendReq(reqType, reqData, onrcv) {
	setTimeout(function() {
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
    }, debugDelay)
}

function redrawUsers() {
	var el = document.getElementById("online_users")
	el.innerHTML = '<b>online users</b>'

	for (var userId in allUsers) {
		var userInfo = allUsers[userId]
		if (userId == ourUserId) continue
		el.appendChild(document.createElement('br'))
		el.appendChild(profileLink(userId, userInfo.Name))
	}
}

function setWebsocketConnection() {
	rcvCallbacks = {}
	websocket = new WebSocket("ws" + (window.location.protocol.indexOf("https") >= 0 ? "s" : "") + "://" + window.location.host + "/events")
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
		var tab = tabs[i]
		document.getElementById(tab).style.display = 'none'
		document.getElementById(tab + '_link').className = ''
	}
}

function ShowCurrent() {
	for (var i = 0; i < tabs.length; i++) {
		var tab = tabs[i]
		if (location.pathname.indexOf('/' + tab + '/') !== -1) {
			document.getElementById(tab).style.display = ''
			if (loaders[tab]) {
				loaders[tab]()
				document.getElementById(tab + '_link').className = 'current_tab'
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

var loadMoreFunc
var isLoading = false

window.onscroll = function(ev) {
	if ((window.innerHeight + window.scrollY) < document.body.offsetHeight) {
		return;
	}

	if (isLoading) {
		return;
	}

	isLoading = true

	setTimeout(function() {
		if (loadMoreFunc) {
			loadMoreFunc()
		}
		isLoading = false
	}, 0)
}

window.onresize = window.onscroll

function setUpPage() {
	hideAll()
	SetUpMessagesPage()
	SetUpTimelinePage()
	SetUpFriendsPage()
	SetUpProfilePage()

	if (window.location.pathname.length <= 1) {
		window.location.pathname = '/timeline/'
	}

	ShowCurrent()

	redrawFriendsRequestCount()

	document.getElementById('loading').style.display = 'none'
	document.getElementById('loading_overlay').style.display = 'none'
}
