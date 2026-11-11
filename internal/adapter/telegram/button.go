package telegram

type Button struct {
	Text string `json:"text"`
	Data string `json:"callback_data"`
}
