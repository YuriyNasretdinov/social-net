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
			el.appendChild(document.createElement('hr'))

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
		}

		b = document.createElement('b')
		b.appendChild(document.createTextNode('Friends:'))
		el.appendChild(b)
		el.appendChild(document.createElement('hr'))

		friends = reply.Users
		for (i = 0; i < friends.length; i++) {
			r = friends[i]
			div = document.createElement('div')
			div.appendChild(document.createTextNode(r.Name))
			el.appendChild(div)
		}
	})
}

function loadUsersList() {
	sendReq("REQUEST_GET_USERS_LIST", {Limit: 50}, function (reply) {
		var el = document.getElementById('users_list')
		el.innerHTML = '<b>Users:</b><hr/>'

		var users = reply.Users
		for (var i = 0; i < users.length; i++) {
			var r = users[i]
			var ulItem = document.createElement('div')
			var aItem

			ulItem.className = 'users_list_item'

			aItem = document.createElement('a')
			aItem.href = '/profile/' + r.Id
			addEv(aItem, 'click', function(ev) { changeLocation('Profile', ev.target.href); return false })
			aItem.appendChild(document.createTextNode(r.Name + ' '))
			ulItem.appendChild(aItem)

			if (!r.IsFriend) {
				aItem = document.createElement('a')
				aItem.title = 'Add to friends'
				aItem.className = 'add_friends_link'
				aItem.href = '#add_friend_' + r.Id
				aItem.id = 'add_friend_link_' + r.Id
				aItem.dataName = r.Name
				aItem.appendChild(document.createTextNode('+'))
				addEv(aItem, 'click', requestAddFriendCallback)

				ulItem.appendChild(aItem)
			} else if (r.FriendshipConfirmed) {
				ulItem.appendChild(document.createTextNode('(friend)'))
			} else {
				ulItem.appendChild(document.createTextNode('(wants to become friend)'))
			}

			el.appendChild(ulItem)
		}
	})
}