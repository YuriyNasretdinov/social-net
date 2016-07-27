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
	if (location_parts[2]) {
		sendReq("REQUEST_GET_PROFILE", {UserId: location_parts[2]}, function(reply) {
			function inp(name, title, value) {
				return "<tr><td><b>" + (title || name) + "</b></td>" +
					"<td>" + htmlspecialchars(value || reply[name]) + "</td></tr>"
			}

			var el = document.getElementById('profile')
			el.innerHTML = '<table>' +
				inp('Name') +
				inp('Birthdate') +
				inp('Sex', 'Sex', SEX_TYPES[reply.Sex]) +
				inp('CityName', 'City') +
				inp('FamilyPosition', 'Position', FAMILY_POSITION_TYPE[reply.FamilyPosition]) +
				'</table>'
		})
	} else {
		sendReq("REQUEST_GET_PROFILE", {UserId: ourUserId}, function(reply) {
			function inp(name, title, value) {
				return "<tr><td><b>" + (title || name) + "</b></td>" +
					"<td><input type='text' name='" + name + "' value='" +
					htmlspecialchars(value || reply[name]) + "' /></td></tr>"
			}

			function sel(name, title, value, variants) {
				var res = "<tr><td><b>"  + (title || name) + "</b></td><td><select name='" + name + "'>"
				for (var k in variants) {
					res += "<option value='" + k + "' " + (value == k ? "selected" : "") + ">" + variants[k] + "</option>"
				}
				res += "</select></td></tr>"
				return res
			}

			var el = document.getElementById('profile')
			el.innerHTML = '<form id="update_profile"><table>' +
				inp('Name') +
				inp('Birthdate') +
				sel('Sex', 'Sex', reply.Sex, SEX_TYPES) +
				inp('CityName', 'City') +
				sel('FamilyPosition', 'Position', reply.FamilyPosition, FAMILY_POSITION_TYPE) +
				'<tr><td colspan="2"><input type="submit" value="Save" onclick="return updateProfile()" /></td></tr>' +
				'</table></form>'
		})
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

	sendReq("REQUEST_UPDATE_PROFILE", req, function(reply) {
		changeLocation("Profile", "/profile/" + ourUserId)
	})

	return false
}
