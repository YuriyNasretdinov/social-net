var SEX_TYPES = {
	1: 'Male',
	2: 'Female'
}

var FAMILY_POSITION_TYPE = {
	1: 'Single',
	2: 'Married'
}

function SetUpProfilePage() {
	addEv("profile_link", "click", function (ev) {
		changeLocation("Profile", "/profile/")
		return false
	})
}

loaders["profile"] = loadProfile

function loadProfile() {
	var location_parts = ("" + location.pathname).split(/\//g)
	var userId = ourUserId
	if (location_parts[2]) {
		userId = location_parts[2]
	}

	sendReq("REQUEST_GET_PROFILE", {UserId: userId}, function(reply) {
		var el = document.getElementById('profile')
		el.innerHTML = '<table>' +
			'<tr><td><b>Name</b></td><td>' + htmlspecialchars(reply.Name) + '</td></tr>' +
			'<tr><td><b>Birthdate</b></td><td>' + reply.Birthdate + '</td></tr>' +
			'<tr><td><b>Sex</b></td><td>' + SEX_TYPES[reply.Sex] + '</td></tr>' +
			'<tr><td><b>City</b></td><td>' + htmlspecialchars(reply.CityName) + '</td></tr>' +
			'<tr><td><b>Position</b></td><td>' + FAMILY_POSITION_TYPE[reply.FamilyPosition] + '</td></tr>' +
			'</table>'
	})
}
