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

var months = {
	1: 'Jan',
	2: 'Feb',
	3: 'Apr',
	4: 'Mar',
	5: 'May',
	6: 'Jun',
	7: 'Jul',
	8: 'Aug',
	9: 'Sep',
	10: 'Oct',
	11: 'Nov',
	12: 'Dec'
};

function loadProfile() {
    var el = document.getElementById('profile')
	el.innerHTML = ''

	var location_parts = ("" + location.pathname).split(/\//g)
	if (location_parts[2]) {
		var userId = location_parts[2]
		sendReq("REQUEST_GET_PROFILE", {UserId: userId}, function(reply) {
			function inp(name, title, value) {
				return "<tr><td><b>" + (title || name) + "</b></td>" +
					"<td>" + htmlspecialchars(value || reply[name]) + "</td></tr>"
			}

			function fmtDate(value) {
				if (!value) return ''

				var parts = value.split(/\-/g)

				var day = +parts[2]
				var month = +parts[1]
				var year = +parts[0]

				return day + ' ' + months[month] + ' ' + year
			}

			var cont = '<table>' +
				inp('Name') +
				inp('Birthdate', 'Birthdate', fmtDate(reply.Birthdate)) +
				inp('Sex', 'Sex', SEX_TYPES[reply.Sex]) +
				inp('CityName', 'City') +
				inp('FamilyPosition', 'Position', FAMILY_POSITION_TYPE[reply.FamilyPosition]) +
				inp('FriendsCount', 'Friends') +
				'</table>'

			if (ourUserId != userId && reply.IsFriend) {
				if (reply.RequestAccepted) {
					cont += '<div>(you are friends)</div>'
				} else {
					cont += '<div>(friends request already sent)</div>'
				}
			}

			el.innerHTML = cont

			if (ourUserId != userId && !reply.IsFriend) {
				el.appendChild(addToFriendsLink(
					userId,
					reply.Name,
					'add to friends'
				))
			}
		})
	} else {
		sendReq("REQUEST_GET_PROFILE", {UserId: ourUserId}, function(reply) {
			function inp(name, title, value) {
				return "<tr><td><b>" + (title || name) + "</b></td>" +
					"<td><input name='" + name + "' value='" +
					htmlspecialchars(value || reply[name]) + "' /></td></tr>"
			}

			function inpdate(name, title, value) {
				var parts = value.split(/\-/g)

				var day = +parts[2]
				var month = +parts[1]
				var year = (+parts[0]) || 1980

				var days = {};
				for (var i = 1; i <= 31; i++) days[i] = i;

				var years = {};
				for (var i = 1800; i <= (new Date()).getFullYear(); i++) years[i] = i;

				return "<tr><td><b>" + (title || name) + "</b></td><td>" +
					sel('Birthdate.Day', day, days) + " " +
					sel('Birthdate.Month', month, months) + " " +
					sel('Birthdate.Year', year, years) +
					"</td></tr>"
			}

			function seltd(name, title, value, options) {
				return "<tr><td><b>"  + (title || name) + "</b></td><td>" + sel(name, value, options) + "</td></tr>"
			}

			function sel(name, value, options) {
				var res = "<select name='" + name + "'>"
				for (var k in options) {
					res += "<option value='" + k + "' " + (value == k ? "selected" : "") + ">" + options[k] + "</option>"
				}
				res += "</select>"
				return res
			}

			el.innerHTML = '<form id="update_profile"><table>' +
				inp('Name') +
				inpdate('Birthdate', 'Birthdate', reply.Birthdate) +
				seltd('Sex', 'Sex', reply.Sex, SEX_TYPES) +
				inp('CityName', 'City') +
				seltd('FamilyPosition', 'Position', reply.FamilyPosition, FAMILY_POSITION_TYPE) +
				'<tr><td><b>Avatar</b></td><td><input type="file" id="profile_avatar" onchange="handleAvatarUpload(this.files)" /></td></tr>' +
				'<tr><td colspan="2"><input type="submit" value="Save" onclick="return updateProfile()" /></td></tr>' +
				'</table></form>'
		})
	}
}

function handleAvatarUpload(files) {
    for (var i = 0; i < files.length; i++) {
        var file = files[i];

    }
}

function updateProfile() {
	var form = document.getElementById('update_profile')
	var req = {}, err

	function fillReq(arr, cb) {
		for (var i = 0; i < arr.length; i++) {
			var el = arr[i]
			if (!el.name) continue
			el.style.outline = ''

			if (!el.value) {
				el.style.outline = 'red 1px solid'
				err = true
			}
			req[el.name] = cb ? cb(el.value) : el.value
		}
	}

	fillReq(form.getElementsByTagName('input'))
	fillReq(form.getElementsByTagName('select'), parseInt)

	if (err) {
		showError("Mandatory fields must be filled in")
		return false
	}

	function zeropad(m) {
		m = '' + m;
		if (m.length == 0) return '00';
		else if (m.length == 1) return '0' + m;
		return m;
	}

	req['Birthdate'] = req['Birthdate.Year'] + '-' + zeropad(req['Birthdate.Month']) + '-' + zeropad(req['Birthdate.Day'])

	sendReq("REQUEST_UPDATE_PROFILE", req, function(reply) {
		changeLocation("Profile", "/profile/" + ourUserId)
	})

	return false
}
