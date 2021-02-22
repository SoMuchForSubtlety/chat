package main

import (
	"bufio"
	"fmt"
	"os"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/gempir/go-twitch-irc/v2"
)

type Chat struct {
	screen tcell.Screen
	client *twitch.Client

	scrollOffset int
	messages     []twitch.PrivateMessage

	sync.RWMutex
}

func (c *Chat) drawMessages() {
	c.screen.Clear()
	w, h := c.screen.Size()
	c.RLock()
	defer c.RUnlock()
	y := h
	for i := c.scrollOffset; y >= 0 && i < len(c.messages); i++ {
		msg := c.messages[(len(c.messages) - (1 + i))]
		y = c.drawMessage(msg, y, w)
	}
	c.screen.Show()
}

func (c *Chat) drawMessage(msg twitch.PrivateMessage, yStart, width int) int {
	username := []rune(msg.User.DisplayName)
	message := []rune(msg.Message)
	msgSize := len(username) + len(message) + 2
	lines := msgSize / width
	if msgSize%width > 0 {
		lines++
	}
	x := 0
	y := yStart - (lines - 1)
	usernameStyle := tcell.StyleDefault.Foreground(tcell.GetColor(msg.User.Color))
	x, y = c.drawString(x, y, width, username, usernameStyle)
	x, y = c.drawString(x, y, width, []rune{':', ' '}, tcell.StyleDefault)
	_, _ = c.drawString(x, y, width, message, tcell.StyleDefault)
	return yStart - lines
}

func (c *Chat) drawString(x, y, width int, content []rune, style tcell.Style) (int, int) {
	for i := 0; i < len(content); i++ {
		if x > width {
			x = 0
			y++
		}
		c.screen.SetCell(x, y, style, content[i])
		x++
	}
	return x, y
}

func main() {
	err := run()
	if err != nil && err != twitch.ErrClientDisconnected {
		fmt.Println("error:", err)
	}
}

func run() error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("twich channel: ")
	channel, _ := reader.ReadString('\n')

	tcell.SetEncodingFallback(tcell.EncodingFallbackASCII)
	s, err := tcell.NewScreen()
	if err != nil {
		return err
	}

	s.EnableMouse()
	if err = s.Init(); err != nil {
		return err
	}
	defer s.Fini()

	s.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorWhite))
	s.Clear()

	client := twitch.NewAnonymousClient()
	chat := Chat{
		screen: s,
		client: client,
	}

	go func() {
		for {
			ev := s.PollEvent()
			switch ev := ev.(type) {
			case *tcell.EventKey:
				switch ev.Key() {
				case tcell.KeyEscape, tcell.KeyCtrlC:
					_ = client.Disconnect()
					return
				case tcell.KeyCtrlL:
					s.Sync()
				}
			case *tcell.EventResize:
				chat.drawMessages()
				s.Sync()
			case *tcell.EventMouse:
				switch ev.Buttons() {
				case tcell.WheelUp:
					chat.scrollOffset++
					if chat.scrollOffset > len(chat.messages) {
						chat.scrollOffset = len(chat.messages)
					}
				case tcell.WheelDown:
					chat.scrollOffset--
					if chat.scrollOffset < 0 {
						chat.scrollOffset = 0
					}
				}
				chat.drawMessages()
			}
		}
	}()

	client.Join(channel)
	client.OnPrivateMessage(func(message twitch.PrivateMessage) {
		chat.Lock()
		chat.messages = append(chat.messages, message)

		// remove messages in batches of 50
		if len(chat.messages) >= 5000 && chat.scrollOffset < 4900 {
			chat.messages = chat.messages[50:]
		}
		chat.Unlock()
		if chat.scrollOffset == 0 {
			chat.drawMessages()
		} else {
			chat.scrollOffset++
		}
	})

	err = client.Connect()
	if err != nil {
		return err
	}

	return nil
}
