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

function requestAddFriendCallback(ev) {
	var el = ev.target
	var friend_id = el.id.replace('add_friend_link_', '')

	sendReq("REQUEST_ADD_FRIEND", {FriendId: friend_id}, function (reply) {
		el.parentNode.innerHTML = 'request to ' + el.innerHTML + ' has been sent'
	})

	return false
}

function confirmFriendshipCallback(ev) {
	var el = ev.target
	var friend_id = el.id.replace('confirm_friend_link_', '')

	sendReq("REQUEST_CONFIRM_FRIENDSHIP", {FriendId: friend_id}, function (reply) {
		el.parentNode.innerHTML = 'friendship confirmed'
	})

	return false
}

function loadUsersList() {
	sendReq("REQUEST_GET_USERS_LIST", {Limit: 50}, function (reply) {
		var el = document.getElementById('users_list')
		el.innerHTML = '<b>Add to friends:</b><hr/>'

		var users = reply.Users
		for (var i = 0; i < users.length; i++) {
			var r = users[i]
			var ulItem = document.createElement('div')
			var aItem

			ulItem.className = 'users_list_item'

			if (!r.IsFriend) {
				aItem = document.createElement('a')
				aItem.title = 'Add to friends'
				aItem.href = '#add_friend_' + r.Id
				aItem.id = 'add_friend_link_' + r.Id
				aItem.appendChild(document.createTextNode(r.Name))

				addEv(aItem, 'click', requestAddFriendCallback)

				ulItem.appendChild(aItem)
			} else if (r.FriendshipConfirmed) {
				ulItem.appendChild(document.createTextNode(r.Name + ' (friend)'))
			} else {
				ulItem.appendChild(document.createTextNode(r.Name + ' '))

				aItem = document.createElement('a')
				aItem.title = 'Confirm add to friends'
				aItem.href = '#confirm_friend_' + r.Id
				aItem.id = 'confirm_friend_link_' + r.Id
				aItem.appendChild(document.createTextNode('(confirm friendship)'))

				addEv(aItem, 'click', confirmFriendshipCallback)

				ulItem.appendChild(aItem)
			}

			el.appendChild(ulItem)
		}
	})
}