package tgbot

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/rockneurotiko/gorequest"
)

const (
	baseURL = "https://api.telegram.org/bot%s/%s"
	timeout = 20
)

// NewTgBot creates a new bot <3
func NewTgBot(token string) TgBot {
	url := fmt.Sprintf(baseURL, token, "%s")
	tgbot := TgBot{
		Token:          token,
		BaseRequestURL: url,
		MainListener:   nil,
		CommandFuncs:   map[*regexp.Regexp]func(TgBot, Message, []string, map[string]string) *string{}}
	user, err := tgbot.GetMe()
	if err != nil {
		panic(err)
	} else {
		tgbot.FirstName = user.FirstName
		tgbot.ID = user.ID
		tgbot.Username = *user.Username
	}
	return tgbot
}

// TgBot basic bot struct
type TgBot struct {
	Token              string
	FirstName          string
	ID                 int
	Username           string
	BaseRequestURL     string
	MainListener       chan MessageWithUpdateID
	LastUpdateID       int
	CommandFuncs       map[*regexp.Regexp]func(TgBot, Message, []string, map[string]string) *string
	SimpleCommandFuncs map[*regexp.Regexp]func(TgBot, Message, string) *string
}

// SimpleCommandFuncStruct struct wrapper for simple command funcs
type SimpleCommandFuncStruct struct {
	f func(TgBot, Message, string) *string
}

// CallSimpleCommandFunc wrapper for simple functions
func (scf SimpleCommandFuncStruct) CallSimpleCommandFunc(bot TgBot, msg Message, m []string, km map[string]string) *string {
	res := ""
	if msg.Text != nil {
		res2 := scf.f(bot, msg, *msg.Text)
		if res2 != nil {
			res = *res2
		}
	}
	return &res
}

// AddUsernameExpr ...
func (bot TgBot) AddUsernameExpr(expr string) string {
	strs := strings.Split(expr, " ")
	opts := fmt.Sprintf(`(?:@%s)?`, bot.Username)
	if len(strs) == 1 {
		capt := strs[0]
		lastc := capt[len(capt)-1]
		if lastc == '$' {
			strs[0] = capt[:len(capt)-1] + opts + "$"
		} else {
			strs[0] = strs[0] + opts
		}
	} else {
		strs[0] = strs[0] + opts
	}
	newexpr := strings.Join(strs, " ")
	return newexpr
}

// CommandFn Add a command function callback
func (bot TgBot) CommandFn(path string, f func(TgBot, Message, []string, map[string]string) *string) {
	path = bot.AddUsernameExpr(path)
	r := regexp.MustCompile(path)
	bot.CommandFuncs[r] = f
}

// SimpleCommandFn Add a simple command function callback
func (bot TgBot) SimpleCommandFn(path string, f func(TgBot, Message, string) *string) {
	path = bot.AddUsernameExpr(path)
	r := regexp.MustCompile(path)
	newf := SimpleCommandFuncStruct{f}
	bot.CommandFuncs[r] = newf.CallSimpleCommandFunc
}

// FindStringSubmatchMap ...
func FindStringSubmatchMap(r *regexp.Regexp, s string) map[string]string {
	captures := make(map[string]string)
	match := r.FindStringSubmatch(s)
	if match == nil {
		return captures
	}
	for i, name := range r.SubexpNames() {
		// Ignore the whole regexp match and unnamed groups
		if i == 0 || name == "" {
			continue
		}
		captures[name] = match[i]
	}
	return captures
}

// MessageMatchText ...
func MessageMatchText(r *regexp.Regexp, text string) (result bool, vals []string, kvals map[string]string) {
	result = false
	vals = []string{}
	kvals = map[string]string{}
	if r.MatchString(text) {
		result = true
		vals = r.FindStringSubmatch(text)
		kvals = FindStringSubmatchMap(r, text)
	}
	return
}

// ProcessTextMsg ...
func (bot TgBot) ProcessTextMsg(msg Message, text string) {
	for k, f := range bot.CommandFuncs {
		match, vals, kvals := MessageMatchText(k, text)
		if match {
			// Saving the values in the scope for goroutine
			gorfunc := f
			msg := msg
			bot := bot
			go func() {
				res := gorfunc(bot, msg, vals, kvals)
				if res != nil && *res != "" {
					bot.SimpleSendMessage(msg, *res)
				}
			}()
		}
	}
}

// ProcessAllMsg ...
func (bot TgBot) ProcessAllMsg(msg Message) {
	if msg.Text != nil {
		text := *msg.Text
		fmt.Println(text)
		bot.ProcessTextMsg(msg, text)
	}
}

// MessageHandler ...
func (bot TgBot) MessageHandler(Incoming <-chan MessageWithUpdateID) {
	for {
		input := <-Incoming
		go bot.ProcessAllMsg(input.Msg) // go this or not?
	}
}

// func (tgbot *TgBot) ProcessMessagesFn(messages []MessageWithUpdateID) {
// 	for _, msg := range messages {
// 		if msg.UpdateID > tgbot.LastUpdateID {
// 			tgbot.LastUpdateID = msg.UpdateID
// 		}
// 		tgbot.ProcessAllMsg(msg.Msg)
// 	}
// }

// ProcessMessages ...
func (bot *TgBot) ProcessMessages(messages []MessageWithUpdateID) {
	for _, msg := range messages {
		if msg.UpdateID > bot.LastUpdateID {
			bot.LastUpdateID = msg.UpdateID
		}
		bot.MainListener <- msg
	}
}

// SimpleStart Start with the default listener and callbacks
func (bot TgBot) SimpleStart() {
	ch := make(chan MessageWithUpdateID)
	bot.AddMainListener(ch)
	go bot.MessageHandler(ch)
	bot.Start()
	// old way without channel
	// if bot.ID == 0 {
	// 	fmt.Println("No ID, maybe the token is bad.")
	// 	return
	// }
	// i := 0
	// for {
	// 	i = i + 1
	// 	fmt.Println(i)
	// 	updatesList, err := bot.GetUpdates()
	// 	if err != nil {
	// 		fmt.Println(err)
	// 		continue
	// 	}
	// 	bot.ProcessMessagesFn(updatesList)
	// }
}

// Start ...
func (bot TgBot) Start() {
	if bot.ID == 0 {
		fmt.Println("No ID, maybe the token is bad.")
		return
	}

	if bot.MainListener == nil {
		fmt.Println("No listener!")
		return
	}

	i := 0
	for {
		i = i + 1
		fmt.Println(i)
		updatesList, err := bot.GetUpdates()
		if err != nil {
			fmt.Println(err)
			continue
		}
		bot.ProcessMessages(updatesList)
	}
}

// GetMe Call getMe path
func (bot TgBot) GetMe() (User, error) {
	body, err := getPetition(bot.buildPath("getMe"), nil)

	if err != nil {
		return User{}, err
	}

	var data ResultGetUser
	dec := json.NewDecoder(strings.NewReader(body))
	dec.Decode(&data)

	if !data.Ok {
		errormsg := fmt.Sprintf("Some error happened, maybe your token is bad:\nError code: %d\nDescription: %s\nToken: %s", *data.ErrorCode, *data.Description, bot.Token)
		return User{}, errors.New(errormsg)
	}
	return data.Result, nil
}

// GetUpdates call getUpdates
func (bot TgBot) GetUpdates() ([]MessageWithUpdateID, error) {
	timeoutreq := fmt.Sprintf("timeout=%d", timeout)
	lastid := fmt.Sprintf("offset=%d", bot.LastUpdateID+1)

	body, err := getPetition(bot.buildPath("getUpdates"), []string{timeoutreq, lastid})

	if err != nil {
		return []MessageWithUpdateID{}, err
	}

	var data ResultGetUpdates
	json.Unmarshal([]byte(body), &data)

	if !data.Ok {
		return []MessageWithUpdateID{}, errors.New("Some error happened in your petition, check your token.")
	}
	return data.Result, nil
}

// SimpleSendMessage send a simple text message
func (bot TgBot) SimpleSendMessage(msg Message, text string) (res Message, err error) {
	ressm := bot.SendMessage(msg.Chat.ID, text, nil, nil, nil)

	if ressm.Ok && ressm.Result != nil {
		res = *ressm.Result
		err = nil
	} else {
		res = Message{}
		err = fmt.Errorf("Error in petition.\nError code: %d\nDescription: %s", *ressm.ErrorCode, *ressm.Description)
	}

	return
}

// SendMessageWithKeyboard ...
func (bot TgBot) SendMessageWithKeyboard(cid int, text string, dwp *bool, rtmid *int, rm ReplyKeyboardMarkup) {
	var rkm ReplyMarkupInt = rm
	bot.SendMessage(cid, text, dwp, rtmid, &rkm)
}

// SendMessageWithForceReply ...
func (bot TgBot) SendMessageWithForceReply(cid int, text string, dwp *bool, rtmid *int, rm ForceReply) {
	var rkm ReplyMarkupInt = rm
	bot.SendMessage(cid, text, dwp, rtmid, &rkm)
}

// SendMessageWithKeyboardHide ...
func (bot TgBot) SendMessageWithKeyboardHide(cid int, text string, dwp *bool, rtmid *int, rm ReplyKeyboardHide) {
	var rkm ReplyMarkupInt = rm
	bot.SendMessage(cid, text, dwp, rtmid, &rkm)
}

// SendMessage full function wrapper for sendMessage
func (bot TgBot) SendMessage(cid int, text string, dwp *bool, rtmid *int, rm *ReplyMarkupInt) ResultWithMessage {
	payload := QuerySendMessage{cid, text, dwp, rtmid, rm}
	return bot.SendMessageQuery(payload)
}

// SendMessageQuery full sendMessage call
func (bot TgBot) SendMessageQuery(payload QuerySendMessage) ResultWithMessage {
	url := bot.buildPath("sendMessage")
	return bot.GenericSendPostData(url, payload)
}

// ForwardMessage full function wrapper for forwardMessage
func (bot TgBot) ForwardMessage(cid int, fid int, mid int) ResultWithMessage {
	payload := ForwardMessageQuery{cid, fid, mid}
	return bot.ForwardMessageQuery(payload)
}

// ForwardMessageQuery  full forwardMessage call
func (bot TgBot) ForwardMessageQuery(payload ForwardMessageQuery) ResultWithMessage {
	url := bot.buildPath("forwardMessage")
	return bot.GenericSendPostData(url, payload)
}

// GenericSendPostData ...
func (bot TgBot) GenericSendPostData(url string, payload interface{}) ResultWithMessage {
	body, error := postPetition(url, payload)
	if error != nil {
		errc := 500
		err := "Some error happened while sending the message"
		return ResultWithMessage{ResultBase{false, &errc, &err}, nil}
	}
	var result ResultWithMessage
	json.Unmarshal([]byte(body), &result)
	return result
}

// buildPath build the path
func (bot TgBot) buildPath(action string) string {
	return fmt.Sprintf(bot.BaseRequestURL, action)
}

// AddMainListener ...
func (bot *TgBot) AddMainListener(list chan MessageWithUpdateID) {
	bot.MainListener = list
}

// postPetition ...
func postPetition(url string, payload interface{}) (string, error) {
	request := gorequest.New().Post(url).
		Send(payload)
	request.TargetType = "form"

	_, body, err := request.End()
	if err != nil {
		return "", errors.New("Some error happened")
	}
	return body, nil
}

// getPetition ...
func getPetition(url string, queries []string) (string, error) {
	req := gorequest.New().Get(url)

	for _, q := range queries {
		req.Query(q)
	}
	_, body, errq := req.End()
	if errq != nil {
		return "", errors.New("There were some error trying to do the petition")
	}
	return body, nil
}
