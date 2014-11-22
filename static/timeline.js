var DEFAULT_TIMELINE_LIMIT = 10

function setUpTimelinePage() {
    addEv("timeline_link", "click", function(ev) {
        changeLocation("Timeline", "/timeline/")
        return false
    })
    addEv("timeline_msg", 'keydown', function(ev) {
        if (ev.keyCode == 13) {
            console.log(ev.target.value)
            ev.target.value = ''
        }
    })
}

function showTimeline() {
    sendReq(
        "REQUEST_GET_TIMELINE",
        {Limit: DEFAULT_TIMELINE_LIMIT + 1},
        function (reply) { console.log(reply) }
    )
}
