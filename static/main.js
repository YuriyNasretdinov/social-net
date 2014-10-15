var allUsers = {};

function onMessage(evt) {
	console.log(evt.data);
	var parts = evt.data.split(/:/)
	if (parts[0] == 'EVENT_ONLINE_USERS_LIST') {
		if (parts[1].length > 0) {
			var pparts = parts[1].split(/\|/g)
			for (var i = 0; i < pparts.length; i++) {
				allUsers[pparts[i]] = true;
			}
		}
	} else if (parts[0] == 'EVENT_USER_CONNECTED') {
		allUsers[parts[1]] = true;
	} else if (parts[0] == 'EVENT_USER_DISCONNECTED') {
		delete(allUsers[parts[1]]);
	}

	redrawUsers();
}

function redrawUsers() {
	var str = 'online users';
	for (var userName in allUsers) {
		str += '<br/>' + userName;
	}
	document.getElementById("online_users").innerHTML = str;
}

function setWebsocketConnection() {
	var websocket = new WebSocket("ws://" + window.location.host + "/events");
	websocket.onopen = function(evt) { console.log("open"); };
	websocket.onclose = function(evt) { console.log("close"); setTimeout(setWebsocketConnection, 1000); };
	websocket.onmessage = onMessage;
	websocket.onerror = function(evt) { console.log("Error: " + evt) };
}
