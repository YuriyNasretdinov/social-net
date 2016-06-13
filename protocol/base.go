package protocol

const (
	REQUEST_GET_MESSAGES = iota
	REQUEST_SEND_MESSAGE
	REQUEST_GET_TIMELINE
	REQUEST_ADD_TO_TIMELINE
	REQUEST_GET_USERS_LIST
	REQUEST_ADD_FRIEND
	REQUEST_CONFIRM_FRIENDSHIP
	REQUEST_GET_MESSAGES_USERS

	REPLY_ERROR = iota
	REPLY_MESSAGES_LIST
	REPLY_GENERIC
	REPLY_GET_TIMELINE
	REPLY_GET_MESSAGES_USERS

	MAX_MESSAGES_LIMIT   = 100
	MAX_TIMELINE_LIMIT   = 100
	MAX_USERS_LIST_LIMIT = 100

	MSG_TYPE_OUT = true
	MSG_TYPE_IN  = false
)

type (
	JSUserInfo struct {
		Name string
		Id   string
	}

	JSUserListInfo struct {
		JSUserInfo
		IsFriend            bool
		FriendshipConfirmed bool
	}

	BaseRequest struct {
		SeqId   int
		Type    string
		ReqData string
	}

	BaseReply struct {
		SeqId int
		Type  string
	}

	Message struct {
		Id       uint64
		UserFrom string
		Ts       string
		IsOut    bool
		Text     string
	}

	TimelineMessage struct {
		Id       uint64
		UserId   string
		UserName string
		Text     string
		Ts       string
	}

	ResponseError struct {
		UserMsg string
		Err     error
	}

	RequestGetMessages struct {
		UserTo  uint64 `json:",string"`
		DateEnd string
		Limit   uint64
	}

	RequestSendMessage struct {
		UserTo uint64 `json:",string"`
		Text   string
	}

	RequestGetTimeline struct {
		DateEnd string
		Limit   uint64
	}

	RequestAddToTimeline struct {
		Text string
	}

	RequestGetUsersList struct {
		Limit uint64
	}

	RequestAddFriend struct {
		FriendId string
	}

	RequestConfirmFriendship struct {
		FriendId string
	}

	RequestGetMessagesUsers struct {
		Limit uint64
	}

	ReplyGetMessages struct {
		BaseReply
		Messages []Message
	}

	ReplyGetUsersList struct {
		BaseReply
		Users []JSUserListInfo
	}

	ReplyGetMessagesUsers struct {
		BaseReply
		Users []JSUserInfo
	}

	ReplyGetTimeline struct {
		BaseReply
		Messages []TimelineMessage
	}

	ReplyGeneric struct {
		BaseReply
		Success bool
	}

	ReplyError struct {
		BaseReply
		Message string
	}
)
