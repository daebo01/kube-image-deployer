package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"google.golang.org/api/chat/v1"
	"k8s.io/klog/v2"
)

var _ iLogBackend = (*googleChatBackend)(nil)

type googleChatBackend struct {
	webhookUrl string
	msgPrefix  string
	httpClient *http.Client
	pool       []message
	mutex      sync.RWMutex
}

func NewGoogleChatBackend(stopCh chan struct{}, webhookUrl, msgPrefix string) iLogBackend {
	s := &googleChatBackend{
		webhookUrl: webhookUrl,
		msgPrefix:  msgPrefix,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		pool:       make([]message, 0),
		mutex:      sync.RWMutex{},
	}

	s.start(stopCh)

	return s
}

func (s *googleChatBackend) InfoDepth(depth int, msg string) {
	s.pushMessage(depth+1, "info", msg)

}

func (s *googleChatBackend) WarningDepth(depth int, msg string) {
	s.pushMessage(depth+1, "warn", msg)
}

func (s *googleChatBackend) ErrorDepth(depth int, msg string) {
	s.pushMessage(depth+1, "error", msg)
}

func (s *googleChatBackend) pushMessage(depth int, level, msg string) {
	_, file, line, ok := runtime.Caller(depth)

	if !ok {
		file = "???"
		line = 0
	}

	file = filepath.Base(file)

	s.mutex.Lock()
	s.pool = append(s.pool, message{level: level, msg: msg, time: time.Now(), file: file, line: line})
	s.mutex.Unlock()
}

func (s *googleChatBackend) start(stopCh chan struct{}) {
	toChatColumn := func(columnName string, value string) *chat.GoogleAppsCardV1Column {
		return &chat.GoogleAppsCardV1Column{
			Widgets: []*chat.GoogleAppsCardV1Widgets{
				{
					TextParagraph: &chat.GoogleAppsCardV1TextParagraph{
						Text: columnName,
					},
				},
				{
					TextParagraph: &chat.GoogleAppsCardV1TextParagraph{
						Text: value,
					},
				},
			},
		}
	}

	toChatMsg := func(msg string, columns [][2]string) *chat.Message {
		chatCard := &chat.GoogleAppsCardV1Card{
			Sections: []*chat.GoogleAppsCardV1Section{
				{
					Collapsible: false,
					Widgets: []*chat.GoogleAppsCardV1Widget{
						{
							TextParagraph: &chat.GoogleAppsCardV1TextParagraph{
								Text: msg,
							},
						},
					},
				},
			},
		}

		for _, column := range columns {
			widgets := chatCard.Sections[0].Widgets
			lastWidget := chatCard.Sections[0].Widgets[len(widgets)-1]

			if lastWidget.Columns == nil || len(lastWidget.Columns.ColumnItems) >= 2 {
				chatCard.Sections[0].Widgets = append(chatCard.Sections[0].Widgets, &chat.GoogleAppsCardV1Widget{
					Columns: &chat.GoogleAppsCardV1Columns{
						ColumnItems: []*chat.GoogleAppsCardV1Column{
							toChatColumn(column[0], column[1]),
						},
					},
				})

				continue
			}

			lastWidget.Columns.ColumnItems = append(lastWidget.Columns.ColumnItems, toChatColumn(column[0], column[1]))
		}

		chatMsg := &chat.Message{
			CardsV2: []*chat.CardWithId{
				{
					Card:   chatCard,
					CardId: "card1",
				},
			},
		}

		return chatMsg
	}

	go func() {
		for {
			select {
			case <-stopCh: // exit goroutine
				return
			case <-time.After(time.Second):
				msg := message{}

				s.mutex.RLock()
				if len(s.pool) > 0 {
					msg, s.pool = s.pool[0], s.pool[1:]
				}
				s.mutex.RUnlock()

				if msg.msg == "" {
					break
				}

				<-time.After(time.Second) // wait for a second before sending the next message

				chatMsg := toChatMsg(
					msg.msg,
					[][2]string{
						{"Level", msg.level},
						{"Env", s.msgPrefix},
						{"File", fmt.Sprintf("%s:%d", msg.file, msg.line)},
					},
				)

				if err := s.send(chatMsg); err != nil {
					klog.Errorf("error sending to google chat: %v\n", err)
				}
			}
		}
	}()
}

func (s *googleChatBackend) send(msg *chat.Message) error {

	body, _ := json.Marshal(msg)
	req, err := http.NewRequest(http.MethodPost, s.webhookUrl, bytes.NewBuffer(body))

	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		resBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("non-ok response returned from google chat \n%s", string(resBody))
	}

	resp.Body.Close()
	return nil
}
