function setUpFriendsPage() {
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

function loadUsersList() {
    sendReq("REQUEST_GET_USERS_LIST", {Limit: 50}, function (reply) {
        var el = document.getElementById('users_list')
        el.innerHTML = ''

        var users = reply.Users;
        for (var i = 0; i < users.length; i++) {
            var r = users[i]
            var ulItem = document.createElement('div')
            ulItem.className = 'users_list_item'
            var aItem = document.createElement('a')
            aItem.title = 'Add to friends'
            aItem.href = '#add_friend_' + r.Id
            aItem.appendChild(document.createTextNode(r.Name))
            ulItem.appendChild(aItem)
            el.appendChild(ulItem)
        }
    })
}