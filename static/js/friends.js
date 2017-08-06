function SetUpFriendsPage() {
	addEv("friends_link", "click", function (ev) {
		changeLocation("Friends", "/friends/")
		return false
	})
	addEv("users_list_link", "click", function (ev) {
		changeLocation("Users", "/users_list/")
		return false
	})
}

loaders['users_list'] = loadUsersList
loaders['friends'] = loadFriends

function requestAddFriendCallback(ev) {
	var el = ev.target
	var friend_id = el.id.replace('add_friend_link_', '')

	sendReq("REQUEST_ADD_FRIEND", {FriendId: friend_id}, function (reply) {
		el.parentNode.innerHTML = 'request to ' + el.dataName + ' has been sent'
	})

	return false
}

function redrawFriendsRequestCount() {
	var el = document.getElementById('friends_badge')
	if (friendsRequestsCount > 0) {
		el.innerHTML = friendsRequestsCount
		el.style.display = ''
	} else {
		el.style.display = 'none'
	}
}

function confirmFriendshipCallback(ev) {
	var el = ev.target
	var friend_id = el.id.replace('confirm_friend_link_', '')

	sendReq("REQUEST_CONFIRM_FRIENDSHIP", {FriendId: friend_id}, function (reply) {
		el.parentNode.innerHTML = 'friendship confirmed'

		friendsRequestsCount--
		redrawFriendsRequestCount()
	})

	return false
}

function profileLink(id, name) {
    var a = document.createElement('a')
    a.href = '/profile/' + id
    addEv(a, 'click', function(ev) { changeLocation('Profile', ev.target.href); return false })
    a.appendChild(document.createTextNode(name))
	return a
}

function loadFriends() {
	sendReq("REQUEST_GET_FRIENDS", {Limit: 50}, function (reply) {
		var el = document.getElementById('friends')
		var friends, i, r, div, a

		el.innerHTML = ''

		friendsRequestsCount = reply.FriendRequests.length
		redrawFriendsRequestCount()

		if (friendsRequestsCount > 0) {
			var b = document.createElement('b')
			b.appendChild(document.createTextNode('Friends requests:'))
			el.appendChild(b)
			el.appendChild(document.createElement('br'))

			friends = reply.FriendRequests
			for (i = 0; i < friends.length; i++) {
				r = friends[i]
				div = document.createElement('div')
				div.appendChild(document.createTextNode(r.Name + ' '))

				a = document.createElement('a')
				a.title = 'Confirm add to friends'
				a.href = '#confirm_friend_' + r.Id
				a.id = 'confirm_friend_link_' + r.Id
				a.appendChild(document.createTextNode('(confirm friendship)'))

				addEv(a, 'click', confirmFriendshipCallback)

				div.appendChild(a)
				el.appendChild(div)
			}

			el.appendChild(document.createElement('br'))

            b = document.createElement('b')
            b.appendChild(document.createTextNode('Friends:'))
            el.appendChild(b)
            el.appendChild(document.createElement('br'))
		}

		friends = reply.Users
		for (i = 0; i < friends.length; i++) {
			r = friends[i]
			div = document.createElement('div')
			div.appendChild(profileLink(r.Id, r.Name))
			el.appendChild(div)
		}
	})
}

function addToFriendsLink(id, name, lnkText) {
	var el = document.createElement('a')
	el.title = 'Add to friends'
	el.className = 'add_friends_link'
	el.href = '#add_friend_' + id
	el.id = 'add_friend_link_' + id
	el.dataName = name
	el.appendChild(document.createTextNode(lnkText))
	addEv(el, 'click', requestAddFriendCallback)
	return el
}

function deleteElements(els) {
	els = Array.prototype.slice.call(els)

	for (var i = 0; i < els.length; i++) {
		var el = els[i];
		el.parentNode.removeChild(el);
	}
}

var curUserSearch = ''

setInterval(function() {
	var el = document.getElementById('search_users')
	if (el.value != curUserSearch) {
		curUserSearch = el.value
		loadUsersList()
	}
}, 100)

function loadUsersList(minId) {
	var limit = 50
	var req = {Limit: limit + 1, MinId: minId || '0', Search: document.getElementById('search_users').value};

	sendReq("REQUEST_GET_USERS_LIST", req, function (reply) {
		var el

		if (!minId) {
			deleteElements(document.getElementsByClassName('users_list_item'));
			el = document.getElementById('users_list_show_more');
			if (el) {
				deleteElements([el]);
			}
		}

		el = document.getElementById('users_list')

		var users = reply.Users
		var lastId

		for (var i = 0; i < users.length; i++) {
			var r = users[i]
			var ulItem = document.createElement('div')

			ulItem.className = 'users_list_item'
			ulItem.appendChild(profileLink(r.Id, r.Name + ' '))

			if (!r.IsFriend) {
				ulItem.appendChild(addToFriendsLink(r.Id, r.Name, '+'))
			} else if (r.FriendshipConfirmed) {
				ulItem.appendChild(document.createTextNode('(friend)'))
			} else {
				ulItem.appendChild(document.createTextNode('(wants to become friend)'))
			}

			el.appendChild(ulItem)

			lastId = r.Id
		}

		loadMoreFunc = null

		if (users.length > limit) {
			loadMoreFunc = function () { loadUsersList(lastId) }
			window.onscroll()
		}
	})
}