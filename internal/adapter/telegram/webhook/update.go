package webhook

type update struct {
	ID      int64    `json:"update_id"`
	Message *message `json:"message"`
}

type message struct {
	ID       int64  `json:"message_id"`
	ThreadID int64  `json:"message_thread_id"`
	Text     string `json:"text"`
	Chat     chat   `json:"chat"`
	From     *user  `json:"from"`
}

type chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type user struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type IncomingMessage struct {
	UpdateID    int64
	MessageID   int64
	ThreadID    int64
	ChatID      int64
	Private     bool
	UserID      int64
	Username    string
	DisplayName string
	Text        string
}
