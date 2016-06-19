function SetUpProfilePage() {
	addEv("profile_link", "click", function (ev) {
		changeLocation("Profile", "/profile/")
		return false
	})
}

loaders["profile"] = loadProfile;

function loadProfile() {
	
}
