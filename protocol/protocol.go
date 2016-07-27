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
	REQUEST_GET_FRIENDS
	REQUEST_GET_PROFILE
	REQUEST_UPDATE_PROFILE

	REPLY_ERROR = iota
	REPLY_MESSAGES_LIST
	REPLY_GENERIC
	REPLY_GET_TIMELINE
	REPLY_GET_MESSAGES_USERS
	REPLY_GET_FRIENDS
	REPLY_GET_PROFILE

	MAX_MESSAGES_LIMIT   = 100
	MAX_TIMELINE_LIMIT   = 100
	MAX_USERS_LIST_LIMIT = 100
	MAX_FRIENDS_LIMIT    = 100

	MSG_TYPE_OUT = true
	MSG_TYPE_IN  = false

	SEX_TYPE_MALE   = 1
	SEX_TYPE_FEMALE = 2

	FAMILY_POSITION_SINGLE  = 1
	FAMILY_POSITION_MARRIED = 2
)

// Request types
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

	Reply interface {
		SetSeqId(int)
		SetReplyType(string)
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
		BaseReply
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

	RequestGetFriends struct {
		Limit uint64
	}

	RequestGetProfile struct {
		UserId uint64 `json:",string"`
	}

	RequestUpdateProfile struct {
		Name           string
		Birthdate      string
		Sex            int
		CityName       string
		FamilyPosition int
	}
)

// Reply types
type (
	ReplyMessagesList struct {
		BaseReply
		Messages []Message
	}

	ReplyUsersList struct {
		BaseReply
		Users []JSUserListInfo
	}

	ReplyGetFriends struct {
		BaseReply
		Users          []JSUserInfo
		FriendRequests []JSUserInfo
	}

	ReplyGetMessagesUsers struct {
		BaseReply
		Users []JSUserInfo
	}

	ReplyGetTimeline struct {
		BaseReply
		Messages []TimelineMessage
	}

	ReplyGetProfile struct {
		BaseReply
		Name           string
		Birthdate      string
		Sex            int
		Description    string
		CityId         uint64 `json:",string"`
		CityName       string
		FamilyPosition int
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

func (p *BaseReply) SetSeqId(id int) {
	p.SeqId = id
}

func (p *BaseReply) SetReplyType(t string) {
	p.Type = t
}
